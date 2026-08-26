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
	// settingsMu serialises every write to the single settings row. That row is
	// written as ONE full-row UPDATE (see UpdateSettings), so a read-modify-write
	// pair that is not serialised silently reverts every column another writer
	// changed in between — the whole reason MutateSettings exists. Zero value is
	// ready to use; production has exactly one Repo (cmd/bombvault/main.go), and
	// the transaction inside MutateSettings makes the pairing atomic at the
	// DATABASE level too, so a second Repo over the same DB cannot slip past it
	// either.
	settingsMu sync.Mutex
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
