package api_test

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabe-santos/yogurt/internal/apitest"
	"github.com/gabe-santos/yogurt/internal/app"
	"github.com/gabe-santos/yogurt/internal/config"
	"github.com/gabe-santos/yogurt/internal/store"
	"github.com/gabe-santos/yogurt/internal/version"
)

func TestAnEmptyDataDirectoryIsBootstrappedOnFirstRun(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "not-created-yet")

	h := apitest.NewInDir(t, dataDir)
	h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)

	if _, err := os.Stat(filepath.Join(dataDir, store.FileName)); err != nil {
		t.Fatalf("database file after first run: %v", err)
	}
}

func TestRestartingOverAnExistingDatabaseKeepsWorking(t *testing.T) {
	dataDir := t.TempDir()

	first := apitest.NewInDir(t, dataDir)
	token := first.Login(apitest.Password).ExpectStatus(http.StatusNoContent).Cookie("yogurt_session").Value
	first.Stop()

	// Same data directory, new process-equivalent: migrations must be a no-op and
	// the session issued before the restart must still be good.
	second := apitest.NewInDir(t, dataDir)
	second.UseSessionToken(token)
	second.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusOK)
}

func TestADatabaseMigratedByANewerYogurtIsRefused(t *testing.T) {
	dataDir := t.TempDir()
	apitest.NewInDir(t, dataDir).Stop()

	// A newer Yogurt leaves behind a migration this one does not carry, as
	// when a Reader copies a database into an older Instance.
	db, err := sql.Open("sqlite", filepath.Join(dataDir, store.FileName))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (name, applied_at, applied_by)
		VALUES ('9999_from_a_newer_yogurt.sql', unixepoch(), 'v99.0.0')`); err != nil {
		t.Fatalf("record a newer migration: %v", err)
	}
	db.Close()

	application, err := app.New(config.Config{DataDir: dataDir, Password: apitest.Password}, app.Deps{})
	if err == nil {
		application.Close()
		t.Fatal("booted over a database a newer Yogurt migrated")
	}
	for _, want := range []string{"v99.0.0", version.Version} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name version %q", err, want)
		}
	}
}

func TestADatabaseFromBeforeVersioningStillMigratesForward(t *testing.T) {
	dataDir := t.TempDir()

	first := apitest.NewInDir(t, dataDir)
	token := first.Login(apitest.Password).ExpectStatus(http.StatusNoContent).Cookie("yogurt_session").Value
	first.Stop()

	// Every Yogurt before versioning recorded its migrations without saying
	// which Yogurt applied them.
	db, err := sql.Open("sqlite", filepath.Join(dataDir, store.FileName))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if _, err := db.Exec(`ALTER TABLE schema_migrations DROP COLUMN applied_by`); err != nil {
		t.Fatalf("restore the unversioned schema_migrations: %v", err)
	}
	db.Close()

	second := apitest.NewInDir(t, dataDir)
	second.UseSessionToken(token)
	second.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusOK)
}
