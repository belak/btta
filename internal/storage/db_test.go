package storage

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestOpenRunsMigrations(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	assert.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	for _, table := range []string{"users", "scores", "images", "sessions", "schema_migrations"} {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table,
		).Scan(&name)
		assert.NoError(t, err, "table %q should exist", table)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db1, err := Open(path)
	assert.NoError(t, err)
	db1.Close()

	// Opening again re-runs migrate, which must be a no-op.
	db2, err := Open(path)
	assert.NoError(t, err)
	t.Cleanup(func() { db2.Close() })

	var applied int
	assert.NoError(t, db2.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied))
	assert.True(t, applied >= 2, "expected at least the two bundled migrations")
}

func TestOpenSetsPragmas(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	assert.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	var busyTimeout int
	assert.NoError(t, db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout))
	assert.Equal(t, 5000, busyTimeout)

	var foreignKeys int
	assert.NoError(t, db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys))
	assert.Equal(t, 1, foreignKeys)
}
