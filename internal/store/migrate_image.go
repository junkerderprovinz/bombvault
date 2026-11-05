package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// reuseMigrations and migratedImage serve a test binary with thousands of stores
// of its own, where the migrations of each would take most of its run time.
var (
	reuseMigrations atomic.Bool
	migratedImage   = sync.OnceValues(buildMigratedImage)
)

// ReuseMigrations makes Migrate start every empty in-memory database from a copy
// of one it migrated once. Only a test binary calls it. A database that holds
// anything, or one in a file, still runs the migrations themselves.
func ReuseMigrations() { reuseMigrations.Store(true) }

// buildMigratedImage runs the migrations once and returns the database they
// leave.
func buildMigratedImage() ([]byte, error) {
	db, err := Open(":memory:")
	if err != nil {
		return nil, err
	}
	defer db.Close() //nolint:errcheck // an in-memory database has nothing to lose
	if err := migrateSteps(db); err != nil {
		return nil, err
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Close() //nolint:errcheck // returns the connection to a pool about to close
	var image []byte
	err = conn.Raw(func(driverConn any) error {
		image, err = driverConn.(interface{ Serialize() ([]byte, error) }).Serialize()
		return err
	})
	return image, err
}

// isEmptyInMemory reports whether db is an in-memory database with nothing in
// it.
func isEmptyInMemory(db *sql.DB) (bool, error) {
	var file string
	if err := db.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&file); err != nil {
		return false, fmt.Errorf("migrate: read the database list: %w", err)
	}
	if file != "" {
		return false, nil
	}
	var objects int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema`).Scan(&objects); err != nil {
		return false, fmt.Errorf("migrate: read the schema: %w", err)
	}
	return objects == 0, nil
}

// startFromImage turns an empty in-memory database into a copy of the migrated
// one, and reports whether it did. What a migration makes up for each database,
// the dates it was applied and the instance ID, is made up again, so no two
// tests share it.
func startFromImage(db *sql.DB) (bool, error) {
	if empty, err := isEmptyInMemory(db); err != nil || !empty {
		return false, err
	}
	image, err := migratedImage()
	if err != nil {
		return false, err
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return false, err
	}
	err = conn.Raw(func(driverConn any) error {
		return driverConn.(interface{ Deserialize([]byte) error }).Deserialize(image)
	})
	if cerr := conn.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return false, fmt.Errorf("migrate: start from the image: %w", err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET applied_at = ?`, time.Now().Unix()); err != nil {
		return false, fmt.Errorf("migrate: date the image: %w", err)
	}
	if _, err := db.Exec(`UPDATE group_state SET instance_id = lower(hex(randomblob(16)))`); err != nil {
		return false, fmt.Errorf("migrate: name the image: %w", err)
	}
	return true, nil
}
