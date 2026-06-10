package api

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/belak/btta/internal/storage"
)

// testDB opens a migrated SQLite database in a temp dir.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	assert.NoError(t, err)
	t.Cleanup(func() { database.Close() })
	return database
}

// decodeJSON unmarshals an httptest recorder body into v.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), v))
}
