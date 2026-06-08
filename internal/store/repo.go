package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// Repo provides typed access to the bombvault SQLite database.
type Repo struct {
	db *sql.DB
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
