package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabe-santos/rss-reader/internal/apitest"
	"github.com/gabe-santos/rss-reader/internal/store"
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
	token := first.Login(apitest.Password).ExpectStatus(http.StatusNoContent).Cookie("reader_session").Value
	first.Stop()

	// Same data directory, new process-equivalent: migrations must be a no-op and
	// the session issued before the restart must still be good.
	second := apitest.NewInDir(t, dataDir)
	second.UseSessionToken(token)
	second.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusOK)
}
