package backend

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure Go SQLite driver, registers as "sqlite"
)

const (
	hashCacheDir  = "PhotoBridge"
	hashCacheFile = "hash-cache.db"
)

// HashCache remembers SHA-256 sums so unchanged files are never hashed twice.
// An entry is only valid for the exact (key, size, modTime) it was stored
// with, so a file that changed is simply re-hashed. The cache is best effort:
// a nil *HashCache is valid and always misses, and write failures are logged
// and ignored.
type HashCache struct {
	db *sql.DB
}

// DefaultHashCachePath returns %APPDATA%\PhotoBridge\hash-cache.db (the
// per-user config directory on other platforms).
func DefaultHashCachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find the user config directory: %w", err)
	}
	return filepath.Join(dir, hashCacheDir, hashCacheFile), nil
}

// OpenHashCache opens (and creates when missing) the cache database at path.
func OpenHashCache(path string) (*HashCache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create cache folder: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open hash cache: %w", err)
	}
	// One connection serialises access, which is plenty for a cache.
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS hashes (
			key      TEXT    NOT NULL,
			size     INTEGER NOT NULL,
			mod_time INTEGER NOT NULL,
			sha256   TEXT    NOT NULL,
			PRIMARY KEY (key, size, mod_time)
		) WITHOUT ROWID`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare hash cache: %w", err)
	}
	return &HashCache{db: db}, nil
}

// Get returns the cached sum for exactly this key, size and modTime.
func (c *HashCache) Get(key string, size, modTime int64) (string, bool) {
	if c == nil {
		return "", false
	}
	var sum string
	err := c.db.QueryRow(
		`SELECT sha256 FROM hashes WHERE key = ? AND size = ? AND mod_time = ?`,
		key, size, modTime,
	).Scan(&sum)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.Debug("hash cache read failed", "key", key, "error", err)
		}
		return "", false
	}
	return sum, true
}

// Put stores a sum and drops older entries of the same key, which describe
// versions of the file that no longer exist.
func (c *HashCache) Put(key string, size, modTime int64, sum string) {
	if c == nil {
		return
	}
	tx, err := c.db.Begin()
	if err != nil {
		slog.Debug("hash cache write failed", "key", key, "error", err)
		return
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	if _, err := tx.Exec(`DELETE FROM hashes WHERE key = ?`, key); err != nil {
		slog.Debug("hash cache write failed", "key", key, "error", err)
		return
	}
	if _, err := tx.Exec(
		`INSERT INTO hashes (key, size, mod_time, sha256) VALUES (?, ?, ?, ?)`,
		key, size, modTime, sum,
	); err != nil {
		slog.Debug("hash cache write failed", "key", key, "error", err)
		return
	}
	if err := tx.Commit(); err != nil {
		slog.Debug("hash cache write failed", "key", key, "error", err)
	}
}

// Close closes the database. It is safe to call on a nil cache.
func (c *HashCache) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}
