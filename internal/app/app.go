// Package app assembles the application: configuration and dependencies in,
// one HTTP handler and one running server out. Both the binary and the API
// tests boot through here, so tests exercise the real wiring.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gabe-santos/rss-reader/internal/api"
	"github.com/gabe-santos/rss-reader/internal/auth"
	"github.com/gabe-santos/rss-reader/internal/clock"
	"github.com/gabe-santos/rss-reader/internal/config"
	"github.com/gabe-santos/rss-reader/internal/fetch"
	"github.com/gabe-santos/rss-reader/internal/pull"
	"github.com/gabe-santos/rss-reader/internal/store"
	"github.com/gabe-santos/rss-reader/internal/webui"
)

// Deps are the collaborators a caller may substitute. Zero values mean "the
// real thing".
type Deps struct {
	Clock  clock.Clock
	Logger *slog.Logger
}

// App is one assembled application over one database.
type App struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *store.Store
	handler http.Handler
	pull    *pull.Service
	cancel  context.CancelFunc
}

// New opens the database, applies migrations, and wires the HTTP surface. It
// also starts the background Feed schedule, which runs until Close.
func New(cfg config.Config, deps Deps) (*App, error) {
	if deps.Clock == nil {
		deps.Clock = clock.System{}
	}
	if deps.Logger == nil {
		deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = config.Defaults().SessionTTL
	}

	password, err := auth.NewPassword(cfg.Password)
	if err != nil {
		return nil, err
	}

	db, err := store.Open(context.Background(), cfg.DataDir)
	if err != nil {
		return nil, err
	}

	spa, built := webui.FS()
	if !built {
		deps.Logger.Warn("no frontend build embedded; only the API will respond")
		spa = nil
	}

	pullService := pull.New(db, fetch.New(fetch.Options{
		AllowPrivate: cfg.AllowPrivateFetch,
	}), deps.Clock, deps.Logger, cfg.PollInterval)

	handler := api.New(api.Deps{
		Password: password,
		Sessions: auth.NewSessions(db, deps.Clock, cfg.SessionTTL),
		Limiter:  auth.NewLimiter(deps.Clock),
		Store:    db,
		Pull:     pullService,
		Clock:    deps.Clock,
		Logger:   deps.Logger,
		SPA:      spa,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go pullService.Run(ctx, cfg.PollTick)

	return &App{
		cfg: cfg, logger: deps.Logger, store: db, handler: handler,
		pull: pullService, cancel: cancel,
	}, nil
}

// Handler is the application's HTTP surface.
func (a *App) Handler() http.Handler { return a.handler }

// Close stops the background Feed schedule and releases the database.
func (a *App) Close() error {
	a.cancel()
	return a.store.Close()
}

// Serve runs the HTTP server until the context is cancelled, then shuts down
// gracefully.
func (a *App) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", a.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", a.cfg.Addr, err)
	}

	server := &http.Server{
		Handler:           a.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		a.logger.Info("listening", "addr", listener.Addr().String())
		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		a.logger.Info("shutting down")
		return server.Shutdown(shutdownCtx)
	}
}
