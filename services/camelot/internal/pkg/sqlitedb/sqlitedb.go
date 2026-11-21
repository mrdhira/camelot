// Package sqlitedb opens Camelot's SQLite database on the pure-Go modernc
// driver: a file in WAL mode with foreign keys on, immediate write
// transactions and times in UTC, so the CLI, the MCP server and the web
// server can share one file.
package sqlitedb

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Wigata-Intech/w-tools/cli/migrationx"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// dsnParams: WAL lets readers run beside one writer; busy_timeout waits
// for another process's write; _txlock=immediate makes every transaction
// take the write lock at BEGIN, so two read-then-write transactions
// serialize instead of failing.
const dsnParams = "_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)" +
	"&_time_format=sqlite&_timezone=UTC&_txlock=immediate"

// Open opens (creating it and its directory when missing) the database
// file at path. ":memory:" opens a private in-memory database on one
// connection, for tests.
func Open(path string) (*sql.DB, error) {
	if path == ":memory:" {
		db, err := sql.Open("sqlite", "file::memory:?"+dsnParams)
		if err != nil {
			return nil, fmt.Errorf("sqlitedb: open memory: %w", err)
		}
		db.SetMaxOpenConns(1)
		return db, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("sqlitedb: create directory: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+(&url.URL{Path: path}).EscapedPath()+"?"+dsnParams)
	if err != nil {
		return nil, fmt.Errorf("sqlitedb: open %s: %w", path, err)
	}
	db.SetConnMaxIdleTime(time.Minute)
	return db, nil
}

// WithTx runs fn in a transaction: committed when fn returns nil, rolled
// back when it returns an error (returned unchanged) or panics.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlitedb: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlitedb: commit: %w", err)
	}
	return nil
}

// Backup writes a consistent copy of db to path with VACUUM INTO, safe
// while other connections write. path must not exist.
func Backup(ctx context.Context, db *sql.DB, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("sqlitedb: create backup directory: %w", err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return fmt.Errorf("sqlitedb: backup to %s: %w", path, err)
	}
	return nil
}

// MigrationSet is one repository's migrations in the w-tools migrationx
// file format, recorded in its own history table <Name>_migration_histories.
type MigrationSet struct {
	Name string
	FS   fs.FS
	Dir  string
}

// Migrate applies every pending migration of set on db.
func Migrate(ctx context.Context, logger *slog.Logger, db *sql.DB, set MigrationSet) error {
	src, err := fs.Sub(set.FS, set.Dir)
	if err != nil {
		return fmt.Errorf("sqlitedb: migrations %s: %w", set.Name, err)
	}
	m, err := migrationx.New(db, src, migrationx.Config{
		Dialect: migrationx.DialectSQLite,
		Table:   set.Name + "_" + migrationx.DefaultTable,
		Log:     logger,
	})
	if err != nil {
		return fmt.Errorf("sqlitedb: migrations %s: %w", set.Name, err)
	}
	if err := m.Up(ctx); err != nil {
		return fmt.Errorf("sqlitedb: migrate %s: %w", set.Name, err)
	}
	return nil
}
