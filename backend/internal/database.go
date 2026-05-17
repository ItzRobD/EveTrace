package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// Init opens the SQLite database at path and configures it for the application.
// Call Close when the process exits.
func Init(path string) error {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	// Serialize all writes through a single connection to prevent "database is
	// locked" errors. SQLite in WAL mode supports concurrent readers, but only
	// one writer at a time; a pool size of 1 makes this explicit.
	db.SetMaxOpenConns(1)

	if _, err = db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	return nil
}

// DB returns the underlying *sql.DB for use with go-jet query builders.
func DB() *sql.DB { return db }

// Close closes the database connection pool.
func Close() {
	if db != nil {
		db.Close()
	}
}
