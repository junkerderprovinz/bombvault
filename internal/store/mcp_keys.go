package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// MCPKey is one key an assistant authenticates with. The key itself is shown
// once at creation and never stored: Digest is its peppered HMAC, Hint its last
// four characters and Check a value derived from the id alone, which tells a
// changed APP_KEY from a wrong key.
type MCPKey struct {
	ID              string
	Label           string
	Digest          string
	Hint            string
	Check           string
	CanStartBackups bool
	CreatedAt       int64
	RotatedAt       int64
	LastUsedAt      int64
	LastUsedFrom    string
	RevokedAt       int64
	RevokedReason   string
}

// MCPKeyLimit is how many keys may be active at once. Several named clients are
// the point; a list that grows past this is a sign nobody is retiring them.
const MCPKeyLimit = 10

var (
	ErrMCPKeyNotFound   = errors.New("mcp key not found")
	ErrMCPKeyLimit      = errors.New("mcp key limit reached")
	ErrMCPKeyLabelTaken = errors.New("an active mcp key already has this label")
	ErrMCPKeyInUse      = errors.New("mcp key is referenced by run history")
	ErrMCPKeyActive     = errors.New("mcp key is still active")
)

const mcpKeyCols = `id, label, key_digest, key_hint, key_check, can_start_backups, created_at, rotated_at, last_used_at, last_used_from, revoked_at, revoked_reason` //nolint:gosec // G101: a SQL column list; the digest column holds an HMAC, not a key

func scanMCPKey(s scanner) (MCPKey, error) {
	var k MCPKey
	err := s.Scan(&k.ID, &k.Label, &k.Digest, &k.Hint, &k.Check, &k.CanStartBackups,
		&k.CreatedAt, &k.RotatedAt, &k.LastUsedAt, &k.LastUsedFrom, &k.RevokedAt, &k.RevokedReason)
	if err != nil {
		return MCPKey{}, err
	}
	return k, nil
}

// ActiveMCPKeys returns the keys a request can still authenticate with, oldest
// first.
func (r *Repo) ActiveMCPKeys() ([]MCPKey, error) {
	rows, err := r.db.Query(`SELECT ` + mcpKeyCols + ` FROM mcp_keys
		WHERE revoked_at = 0 AND key_digest != ''
		ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("ActiveMCPKeys: %w", err)
	}
	return collectMCPKeys(rows, "ActiveMCPKeys")
}

// ListMCPKeys returns every key for the settings card: the active ones oldest
// first, then the revoked ones with the most recently revoked at the top.
func (r *Repo) ListMCPKeys() ([]MCPKey, error) {
	rows, err := r.db.Query(`SELECT ` + mcpKeyCols + ` FROM mcp_keys
		ORDER BY revoked_at != 0,
		         CASE WHEN revoked_at = 0 THEN created_at ELSE 0 END,
		         revoked_at DESC,
		         id`)
	if err != nil {
		return nil, fmt.Errorf("ListMCPKeys: %w", err)
	}
	return collectMCPKeys(rows, "ListMCPKeys")
}

func collectMCPKeys(rows *sql.Rows, label string) ([]MCPKey, error) {
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []MCPKey
	for rows.Next() {
		key, err := scanMCPKey(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return out, nil
}

// GetMCPKey returns one key, revoked or not.
func (r *Repo) GetMCPKey(id string) (MCPKey, error) {
	key, err := scanMCPKey(r.db.QueryRow(`SELECT `+mcpKeyCols+` FROM mcp_keys WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MCPKey{}, ErrMCPKeyNotFound
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("GetMCPKey: %w", err)
	}
	return key, nil
}

// CreateMCPKey stores a new key. The id comes from the caller because the check
// value is derived from it before the row exists. Limit and label are settled
// inside the transaction that writes the row, so two creates at once cannot
// both find room.
func (r *Repo) CreateMCPKey(id, label, digest, hint, check string, canStart bool, now int64) (MCPKey, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return MCPKey{}, fmt.Errorf("CreateMCPKey: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	var active int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_keys WHERE revoked_at = 0 AND key_digest != ''`).Scan(&active); err != nil {
		return MCPKey{}, fmt.Errorf("CreateMCPKey count: %w", err)
	}
	if active >= MCPKeyLimit {
		return MCPKey{}, ErrMCPKeyLimit
	}
	taken, err := mcpLabelTaken(tx, label, "")
	if err != nil {
		return MCPKey{}, fmt.Errorf("CreateMCPKey label: %w", err)
	}
	if taken {
		return MCPKey{}, ErrMCPKeyLabelTaken
	}
	_, err = tx.Exec(`INSERT INTO mcp_keys (id, label, key_digest, key_hint, key_check, can_start_backups, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, label, digest, hint, check, canStart, now)
	if err != nil {
		return MCPKey{}, fmt.Errorf("CreateMCPKey: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MCPKey{}, fmt.Errorf("CreateMCPKey commit: %w", err)
	}
	return r.GetMCPKey(id)
}

// mcpLabelTaken reports whether another active key already carries label,
// compared without regard to case as the unique index does.
func mcpLabelTaken(tx *sql.Tx, label, exceptID string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT count(*) FROM mcp_keys
		WHERE revoked_at = 0 AND lower(label) = lower(?) AND id != ?`, label, exceptID).Scan(&n)
	return n > 0, err
}

// RotateMCPKey gives an active key new secret material and keeps its id, label,
// permission and creation time, so everything that references it stays valid.
func (r *Repo) RotateMCPKey(id, digest, hint, check string, now int64) (MCPKey, error) {
	res, err := r.db.Exec(`UPDATE mcp_keys
		SET key_digest = ?, key_hint = ?, key_check = ?, rotated_at = ?
		WHERE id = ? AND revoked_at = 0`, digest, hint, check, now, id)
	if err != nil {
		return MCPKey{}, fmt.Errorf("RotateMCPKey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return MCPKey{}, ErrMCPKeyNotFound
	}
	return r.GetMCPKey(id)
}

// UpdateMCPKey changes an active key's label, its permission to start backups,
// or both. A nil argument leaves that column alone.
func (r *Repo) UpdateMCPKey(id string, label *string, canStart *bool) (MCPKey, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return MCPKey{}, fmt.Errorf("UpdateMCPKey: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	var active int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_keys WHERE id = ? AND revoked_at = 0`, id).Scan(&active); err != nil {
		return MCPKey{}, fmt.Errorf("UpdateMCPKey: %w", err)
	}
	if active == 0 {
		return MCPKey{}, ErrMCPKeyNotFound
	}
	if label != nil {
		taken, err := mcpLabelTaken(tx, *label, id)
		if err != nil {
			return MCPKey{}, fmt.Errorf("UpdateMCPKey label: %w", err)
		}
		if taken {
			return MCPKey{}, ErrMCPKeyLabelTaken
		}
		if _, err := tx.Exec(`UPDATE mcp_keys SET label = ? WHERE id = ?`, *label, id); err != nil {
			return MCPKey{}, fmt.Errorf("UpdateMCPKey label: %w", err)
		}
	}
	if canStart != nil {
		if _, err := tx.Exec(`UPDATE mcp_keys SET can_start_backups = ? WHERE id = ?`, *canStart, id); err != nil {
			return MCPKey{}, fmt.Errorf("UpdateMCPKey permission: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return MCPKey{}, fmt.Errorf("UpdateMCPKey commit: %w", err)
	}
	return r.GetMCPKey(id)
}

// RevokeMCPKey empties a key's digest and keeps the row, so the Activity log can
// still name the client behind an old run. A revoked key never comes back.
func (r *Repo) RevokeMCPKey(id, reason string, now int64) error {
	res, err := r.db.Exec(`UPDATE mcp_keys
		SET key_digest = '', revoked_at = ?, revoked_reason = ?
		WHERE id = ? AND revoked_at = 0`, now, reason, id)
	if err != nil {
		return fmt.Errorf("RevokeMCPKey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrMCPKeyNotFound
	}
	return nil
}

// RevokeAllMCPKeys revokes every active key and returns how many there were.
func (r *Repo) RevokeAllMCPKeys(reason string, now int64) (int64, error) {
	res, err := r.db.Exec(`UPDATE mcp_keys
		SET key_digest = '', revoked_at = ?, revoked_reason = ?
		WHERE revoked_at = 0`, now, reason)
	if err != nil {
		return 0, fmt.Errorf("RevokeAllMCPKeys: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PurgeMCPKey removes a revoked key for good. It refuses while the key is
// active or while a run still names it, because the Activity log reads the
// label from this row.
func (r *Repo) PurgeMCPKey(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("PurgeMCPKey: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	var revokedAt int64
	err = tx.QueryRow(`SELECT revoked_at FROM mcp_keys WHERE id = ?`, id).Scan(&revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMCPKeyNotFound
	}
	if err != nil {
		return fmt.Errorf("PurgeMCPKey: %w", err)
	}
	if revokedAt == 0 {
		return ErrMCPKeyActive
	}
	var used bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM runs WHERE started_via_key = ?)`, id).Scan(&used); err != nil {
		return fmt.Errorf("PurgeMCPKey references: %w", err)
	}
	if used {
		return ErrMCPKeyInUse
	}
	if _, err := tx.Exec(`DELETE FROM mcp_keys WHERE id = ?`, id); err != nil {
		return fmt.Errorf("PurgeMCPKey: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PurgeMCPKey commit: %w", err)
	}
	return nil
}

// TouchMCPKey records when and from where a key was last used. A key revoked in
// the meantime simply records nothing: this bookkeeping must not fail a request.
func (r *Repo) TouchMCPKey(id string, at int64, from string) error {
	_, err := r.db.Exec(`UPDATE mcp_keys SET last_used_at = ?, last_used_from = ?
		WHERE id = ? AND revoked_at = 0`, at, from, id)
	if err != nil {
		return fmt.Errorf("TouchMCPKey: %w", err)
	}
	return nil
}
