package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
)

// Repo provides typed access to the bombvault SQLite database.
type Repo struct {
	db *sql.DB
	// settingsMu serialises writes to the settings row. UpdateSettings rewrites
	// every column, so an unserialised read-modify-write reverts whatever another
	// writer changed in between. MutateSettings also holds a transaction, which
	// covers a second Repo on the same database.
	settingsMu sync.Mutex
	// runFinishedMu guards the run-finished hook. It is installed once during
	// startup, but tests install it after New while other goroutines finish runs.
	runFinishedMu sync.RWMutex
	runFinished   func(RunFinished)
}

// New wraps db in a Repo. Migrate must have been called before using the Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// newID returns a 16-byte (32 hex char) cryptographically random ID.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("newID: %v", err))
	}
	return hex.EncodeToString(b)
}
