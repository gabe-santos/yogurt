// Package config reads the application's settings from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/rss-reader/internal/pullpolicy"
)

// Prefix is applied to every environment variable this application reads.
const Prefix = "READER_"

// Config is the whole of the application's configuration. Every field has a
// working default except the password, which must be supplied.
type Config struct {
	// Addr is the listening address of the HTTP server.
	Addr string
	// DataDir holds the SQLite database file.
	DataDir string
	// Password is the reader's login password.
	Password string
	// SessionTTL is how long a session stays valid.
	SessionTTL time.Duration
	// AllowPrivateFetch lets Feed fetches reach private, loopback and
	// link-local addresses. Off by default, so that a malicious Feed URL
	// cannot make this app probe the network it runs on.
	AllowPrivateFetch bool
	// LogLevel is the minimum level of emitted logs.
	LogLevel slog.Level
	// PollInterval is how often a Feed is checked when nothing else — a
	// publisher's hint, or a run of failures — says otherwise.
	PollInterval time.Duration
	// PollTick is how often the background schedule wakes to look for a due
	// Feed. It only needs to be finer than PollInterval.
	PollTick time.Duration
}

// Defaults are the settings used when the environment says nothing.
func Defaults() Config {
	return Config{
		Addr:         ":8080",
		DataDir:      "./data",
		SessionTTL:   30 * 24 * time.Hour,
		LogLevel:     slog.LevelInfo,
		PollInterval: pullpolicy.DefaultInterval,
		PollTick:     time.Minute,
	}
}

// ErrNoPassword reports a missing password setting, the one thing a self-hoster
// must supply.
var ErrNoPassword = errors.New(Prefix + "PASSWORD is required")

// Load reads the configuration from the environment, applying defaults.
func Load() (Config, error) {
	cfg := Defaults()

	if v, ok := lookup("ADDR"); ok {
		cfg.Addr = v
	}
	if v, ok := lookup("DATA_DIR"); ok {
		cfg.DataDir = v
	}
	if v, ok := lookup("PASSWORD"); ok {
		cfg.Password = v
	}
	if v, ok := lookup("SESSION_TTL"); ok {
		ttl, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("%sSESSION_TTL: %w", Prefix, err)
		}
		if ttl <= 0 {
			return Config{}, fmt.Errorf("%sSESSION_TTL must be positive, got %q", Prefix, v)
		}
		cfg.SessionTTL = ttl
	}
	if v, ok := lookup("LOG_LEVEL"); ok {
		var level slog.Level
		if err := level.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("%sLOG_LEVEL: %w", Prefix, err)
		}
		cfg.LogLevel = level
	}
	if v, ok := lookup("ALLOW_PRIVATE_FETCH"); ok {
		allow, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("%sALLOW_PRIVATE_FETCH: %w", Prefix, err)
		}
		cfg.AllowPrivateFetch = allow
	}
	if v, ok := lookup("POLL_INTERVAL"); ok {
		interval, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("%sPOLL_INTERVAL: %w", Prefix, err)
		}
		if interval <= 0 {
			return Config{}, fmt.Errorf("%sPOLL_INTERVAL must be positive, got %q", Prefix, v)
		}
		cfg.PollInterval = interval
	}
	if v, ok := lookup("POLL_TICK"); ok {
		tick, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("%sPOLL_TICK: %w", Prefix, err)
		}
		if tick <= 0 {
			return Config{}, fmt.Errorf("%sPOLL_TICK must be positive, got %q", Prefix, v)
		}
		cfg.PollTick = tick
	}

	if cfg.Password == "" {
		return Config{}, ErrNoPassword
	}
	return cfg, nil
}

func lookup(name string) (string, bool) {
	v, ok := os.LookupEnv(Prefix + name)
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	return v, true
}
