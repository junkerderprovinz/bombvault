package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// MCPOAuthSettings is whether the MCP endpoint offers OAuth sign-in and the
// public address it does so under, which is the issuer of every token.
type MCPOAuthSettings struct {
	Enabled   bool
	Issuer    string
	UpdatedAt int64
}

// OAuthClient is a client that registered itself through dynamic client
// registration. SecretDigest is the HMAC of its secret, empty for a public
// client. Known is the id of a client the card has a mark for, decided from
// the redirect URIs and never from the name the client sent.
type OAuthClient struct {
	ID           string
	Name         string
	RedirectURIs []string
	AuthMethod   string
	SecretDigest string
	Known        string
	CreatedAt    int64
	CreatedFrom  string
}

// OAuthGrant is what a new grant row is made of. The token digests, when set,
// are stored in the same transaction, so no prune can find the grant without
// a refresh token.
type OAuthGrant struct {
	ID              string
	Label           string
	Client          string
	OAuthClient     string
	Resource        string
	Check           string
	CanStartBackups bool

	AccessDigest   string
	RefreshDigest  string
	AccessExpires  int64
	RefreshExpires int64
}

// Limits of the authorization server. Registration needs no credentials, so the
// clients nobody signed in with are capped and expire; a client with a grant
// stays as long as the grant's row does, because a connector keeps using the
// client id it registered once.
const (
	OAuthUnusedClientLimit = 100
	OAuthUnusedClientTTL   = 24 * 60 * 60
	MCPGrantLimit          = 10

	// OAuthPendingClientGrace keeps a new client from being evicted at the
	// limit for long enough to finish the sign-in it registered for. Past
	// OAuthUnusedClientCeiling unused clients even those go, so a flood still
	// costs a fixed number of rows.
	OAuthPendingClientGrace  = 10 * 60
	OAuthUnusedClientCeiling = 1000

	// OAuthSpentRefreshKept is how many rotated-out refresh tokens of a grant
	// are remembered to recognise one presented again.
	OAuthSpentRefreshKept = 20

	// OAuthRefreshRetryWindow is how many seconds the refresh token spent last
	// may come back as a retry, from a client whose answer got lost on the way.
	OAuthRefreshRetryWindow = 30
)

var (
	ErrOAuthClientNotFound = errors.New("oauth client not found")
	ErrMCPGrantLimit       = errors.New("oauth grant limit reached")
	ErrOAuthTokenNotFound  = errors.New("oauth token not found")
	ErrOAuthRefreshReused  = errors.New("a spent refresh token was presented again")
)

// MCPOAuthSettings returns the stored settings, all zero while none were saved.
func (r *Repo) MCPOAuthSettings() (MCPOAuthSettings, error) {
	var s MCPOAuthSettings
	err := r.db.QueryRow(`SELECT enabled, issuer, updated_at FROM mcp_oauth_settings WHERE id = 1`).
		Scan(&s.Enabled, &s.Issuer, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MCPOAuthSettings{}, nil
	}
	if err != nil {
		return MCPOAuthSettings{}, fmt.Errorf("MCPOAuthSettings: %w", err)
	}
	return s, nil
}

// SetMCPOAuthSettings stores the switch and the issuer.
func (r *Repo) SetMCPOAuthSettings(s MCPOAuthSettings, now int64) error {
	_, err := r.db.Exec(`INSERT INTO mcp_oauth_settings (id, enabled, issuer, updated_at) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled, issuer = excluded.issuer, updated_at = excluded.updated_at`,
		s.Enabled, s.Issuer, now)
	if err != nil {
		return fmt.Errorf("SetMCPOAuthSettings: %w", err)
	}
	return nil
}

// unusedClient is the condition for a client no grant row names.
const unusedClient = `NOT EXISTS (SELECT 1 FROM mcp_keys k WHERE k.oauth_client = mcp_oauth_clients.id)`

// CreateOAuthClient stores a registration. Unused clients past their day go
// first, and at the limit the oldest unused ones past their grace make room.
// A client somebody signed in with is never pushed out.
func (r *Repo) CreateOAuthClient(c OAuthClient, now int64) error {
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return fmt.Errorf("CreateOAuthClient: %w", err)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("CreateOAuthClient: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	if _, err := tx.Exec(`DELETE FROM mcp_oauth_clients WHERE created_at <= ? AND `+unusedClient,
		now-OAuthUnusedClientTTL); err != nil {
		return fmt.Errorf("CreateOAuthClient prune: %w", err)
	}
	if err := evictUnusedClients(tx, OAuthUnusedClientLimit, now-OAuthPendingClientGrace); err != nil {
		return fmt.Errorf("CreateOAuthClient evict: %w", err)
	}
	if err := evictUnusedClients(tx, OAuthUnusedClientCeiling, now); err != nil {
		return fmt.Errorf("CreateOAuthClient evict: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO mcp_oauth_clients
		(id, name, redirect_uris, auth_method, secret_digest, known, created_at, created_from)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, string(uris), c.AuthMethod, c.SecretDigest, c.Known, now, c.CreatedFrom); err != nil {
		return fmt.Errorf("CreateOAuthClient: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("CreateOAuthClient commit: %w", err)
	}
	return nil
}

// evictUnusedClients deletes the oldest unused clients registered at or before
// createdBy until one more fits under limit, or none of them are left.
func evictUnusedClients(tx *sql.Tx, limit int, createdBy int64) error {
	var unused int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_oauth_clients WHERE ` + unusedClient).Scan(&unused); err != nil {
		return err
	}
	over := unused - limit + 1
	if over <= 0 {
		return nil
	}
	_, err := tx.Exec(`DELETE FROM mcp_oauth_clients WHERE id IN (
		SELECT id FROM mcp_oauth_clients WHERE created_at <= ? AND `+unusedClient+` ORDER BY created_at, id LIMIT ?)`,
		createdBy, over)
	return err
}

// GetOAuthClient returns one registered client.
func (r *Repo) GetOAuthClient(id string) (OAuthClient, error) {
	var c OAuthClient
	var uris string
	err := r.db.QueryRow(`SELECT id, name, redirect_uris, auth_method, secret_digest, known, created_at, created_from
		FROM mcp_oauth_clients WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &uris, &c.AuthMethod, &c.SecretDigest, &c.Known, &c.CreatedAt, &c.CreatedFrom)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthClient{}, ErrOAuthClientNotFound
	}
	if err != nil {
		return OAuthClient{}, fmt.Errorf("GetOAuthClient: %w", err)
	}
	if err := json.Unmarshal([]byte(uris), &c.RedirectURIs); err != nil {
		return OAuthClient{}, fmt.Errorf("GetOAuthClient %s: redirect uris: %w", id, err)
	}
	return c, nil
}

// OAuthClientCount is how many clients are registered.
func (r *Repo) OAuthClientCount() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT count(*) FROM mcp_oauth_clients`).Scan(&n); err != nil {
		return 0, fmt.Errorf("OAuthClientCount: %w", err)
	}
	return n, nil
}

// CreateOAuthGrant stores the grant a client earned by signing in. An earlier
// active grant of the same client is revoked as replaced, because the client
// holds only the newest tokens anyway. The label is made unique among the
// active rows by a number behind it.
func (r *Repo) CreateOAuthGrant(g OAuthGrant, now int64) (MCPKey, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return MCPKey{}, fmt.Errorf("CreateOAuthGrant: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	earlier, err := grantIDs(tx, `SELECT id FROM mcp_keys WHERE kind = 'oauth' AND revoked_at = 0 AND oauth_client = ?`, g.OAuthClient)
	if err != nil {
		return MCPKey{}, fmt.Errorf("CreateOAuthGrant: %w", err)
	}
	for _, id := range earlier {
		if _, err := revokeMCPKeyTx(tx, id, "replaced", now); err != nil {
			return MCPKey{}, fmt.Errorf("CreateOAuthGrant replace: %w", err)
		}
	}
	var active int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_keys WHERE kind = 'oauth' AND revoked_at = 0`).Scan(&active); err != nil {
		return MCPKey{}, fmt.Errorf("CreateOAuthGrant count: %w", err)
	}
	if active >= MCPGrantLimit {
		return MCPKey{}, ErrMCPGrantLimit
	}
	label, err := freeLabel(tx, g.Label)
	if err != nil {
		return MCPKey{}, fmt.Errorf("CreateOAuthGrant label: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO mcp_keys
		(id, kind, oauth_client, resource, label, client, key_digest, key_hint, key_check, can_start_backups, created_at)
		VALUES (?, 'oauth', ?, ?, ?, ?, '', '', ?, ?, ?)`,
		g.ID, g.OAuthClient, g.Resource, label, g.Client, g.Check, g.CanStartBackups, now); err != nil {
		return MCPKey{}, fmt.Errorf("CreateOAuthGrant: %w", err)
	}
	if g.AccessDigest != "" {
		if err := insertTokens(tx, g.ID, g.AccessDigest, g.RefreshDigest, g.AccessExpires, g.RefreshExpires, now); err != nil {
			return MCPKey{}, fmt.Errorf("CreateOAuthGrant tokens: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return MCPKey{}, fmt.Errorf("CreateOAuthGrant commit: %w", err)
	}
	return r.GetMCPKey(g.ID)
}

// RevokeOAuthGrants revokes every active grant, keys untouched, and returns
// the rows it revoked.
func (r *Repo) RevokeOAuthGrants(reason string, now int64) ([]MCPKey, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("RevokeOAuthGrants: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	ids, err := grantIDs(tx, `SELECT id FROM mcp_keys WHERE kind = 'oauth' AND revoked_at = 0`)
	if err != nil {
		return nil, fmt.Errorf("RevokeOAuthGrants: %w", err)
	}
	for _, id := range ids {
		if _, err := revokeMCPKeyTx(tx, id, reason, now); err != nil {
			return nil, fmt.Errorf("RevokeOAuthGrants: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("RevokeOAuthGrants commit: %w", err)
	}
	rows := make([]MCPKey, 0, len(ids))
	for _, id := range ids {
		k, err := r.GetMCPKey(id)
		if err != nil {
			return nil, fmt.Errorf("RevokeOAuthGrants: %w", err)
		}
		rows = append(rows, k)
	}
	return rows, nil
}

func grantIDs(tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// freeLabel returns base, or base followed by the first number that no active
// row carries yet, cut so the result stays within 64 characters.
func freeLabel(tx *sql.Tx, base string) (string, error) {
	for n := 1; n < 1000; n++ {
		label := base
		if n > 1 {
			suffix := " " + strconv.Itoa(n)
			label = base
			if keep := 64 - utf8.RuneCountInString(suffix); utf8.RuneCountInString(base) > keep {
				label = string([]rune(base)[:keep])
			}
			label += suffix
		}
		taken, err := mcpLabelTaken(tx, label, "")
		if err != nil {
			return "", err
		}
		if !taken {
			return label, nil
		}
	}
	return "", ErrMCPKeyLabelTaken
}

func insertTokens(tx *sql.Tx, grantID, access, refresh string, accessExp, refreshExp, now int64) error {
	_, err := tx.Exec(`INSERT INTO mcp_oauth_tokens (digest, grant_id, kind, created_at, expires_at)
		VALUES (?, ?, 'access', ?, ?), (?, ?, 'refresh', ?, ?)`,
		access, grantID, now, accessExp, refresh, grantID, now, refreshExp)
	return err
}

// OAuthAccessGrant returns the active grant an unexpired access token belongs
// to.
func (r *Repo) OAuthAccessGrant(digest string, now int64) (MCPKey, error) {
	k, err := scanMCPKey(r.db.QueryRow(`SELECT `+mcpKeyCols+` FROM mcp_keys
		WHERE kind = 'oauth' AND revoked_at = 0 AND id = (
			SELECT grant_id FROM mcp_oauth_tokens WHERE digest = ? AND kind = 'access' AND expires_at > ?)`,
		digest, now))
	if errors.Is(err, sql.ErrNoRows) {
		return MCPKey{}, ErrOAuthTokenNotFound
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("OAuthAccessGrant: %w", err)
	}
	return k, nil
}

// RotateOAuthRefresh trades a live refresh token of clientID for a new access
// and refresh token, as long as its grant was issued for resource. A spent one
// presented again revokes the grant and answers ErrOAuthRefreshReused with it,
// because somebody may hold a copy, unless it is the one spent last and back
// within OAuthRefreshRetryWindow: that is a client whose answer got lost. The
// newest access token stays beside the new one, so a request already on its
// way still gets through.
func (r *Repo) RotateOAuthRefresh(oldDigest, clientID, resource, access, refresh string, accessExp, refreshExp, now int64) (MCPKey, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return MCPKey{}, fmt.Errorf("RotateOAuthRefresh: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	var grantID string
	var expires, spent int64
	err = tx.QueryRow(`SELECT grant_id, expires_at, spent_at FROM mcp_oauth_tokens WHERE digest = ? AND kind = 'refresh'`, oldDigest).
		Scan(&grantID, &expires, &spent)
	if errors.Is(err, sql.ErrNoRows) {
		return MCPKey{}, ErrOAuthTokenNotFound
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("RotateOAuthRefresh: %w", err)
	}
	grant, err := scanMCPKey(tx.QueryRow(`SELECT `+mcpKeyCols+` FROM mcp_keys WHERE id = ? AND kind = 'oauth' AND revoked_at = 0`, grantID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (grant.OAuthClient != clientID || grant.Resource != resource)) {
		return MCPKey{}, ErrOAuthTokenNotFound
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("RotateOAuthRefresh grant: %w", err)
	}
	if spent != 0 {
		var last string
		err := tx.QueryRow(`SELECT digest FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'refresh' AND spent_at != 0
			ORDER BY spent_at DESC, rowid DESC LIMIT 1`, grantID).Scan(&last)
		if err != nil {
			return MCPKey{}, fmt.Errorf("RotateOAuthRefresh: %w", err)
		}
		if last != oldDigest || now-spent > OAuthRefreshRetryWindow {
			if _, err := revokeMCPKeyTx(tx, grantID, "refresh-reuse", now); err != nil {
				return MCPKey{}, fmt.Errorf("RotateOAuthRefresh revoke: %w", err)
			}
			if err := tx.Commit(); err != nil {
				return MCPKey{}, fmt.Errorf("RotateOAuthRefresh commit: %w", err)
			}
			return grant, ErrOAuthRefreshReused
		}
	}
	if expires <= now {
		return MCPKey{}, ErrOAuthTokenNotFound
	}
	steps := []struct {
		q    string
		args []any
	}{
		// A grant has one live refresh token: the one presented, or on a retry
		// the one the lost answer carried.
		{`UPDATE mcp_oauth_tokens SET spent_at = ? WHERE grant_id = ? AND kind = 'refresh' AND spent_at = 0`, []any{now, grantID}},
		{`DELETE FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'access' AND digest NOT IN (
			SELECT digest FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'access'
			ORDER BY created_at DESC, rowid DESC LIMIT 1)`, []any{grantID, grantID}},
		{`DELETE FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'refresh' AND spent_at != 0 AND digest NOT IN (
			SELECT digest FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'refresh' AND spent_at != 0
			ORDER BY spent_at DESC, rowid DESC LIMIT ?)`, []any{grantID, grantID, OAuthSpentRefreshKept}},
	}
	for _, s := range steps {
		if _, err := tx.Exec(s.q, s.args...); err != nil {
			return MCPKey{}, fmt.Errorf("RotateOAuthRefresh: %w", err)
		}
	}
	if err := insertTokens(tx, grantID, access, refresh, accessExp, refreshExp, now); err != nil {
		return MCPKey{}, fmt.Errorf("RotateOAuthRefresh insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MCPKey{}, fmt.Errorf("RotateOAuthRefresh commit: %w", err)
	}
	return grant, nil
}

// RevokeOAuthToken is token revocation by the client itself. An access token
// is dropped on its own; a refresh token ends the grant, which it returns. A
// token that is unknown or belongs to another client changes nothing and is no
// error, as RFC 7009 wants.
func (r *Repo) RevokeOAuthToken(digest, clientID string, now int64) (MCPKey, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return MCPKey{}, fmt.Errorf("RevokeOAuthToken: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	var grantID, kind string
	err = tx.QueryRow(`SELECT grant_id, kind FROM mcp_oauth_tokens WHERE digest = ?`, digest).Scan(&grantID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return MCPKey{}, nil
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("RevokeOAuthToken: %w", err)
	}
	grant, err := scanMCPKey(tx.QueryRow(`SELECT `+mcpKeyCols+` FROM mcp_keys WHERE id = ? AND kind = 'oauth'`, grantID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && grant.OAuthClient != clientID) {
		return MCPKey{}, nil
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("RevokeOAuthToken grant: %w", err)
	}
	if kind == "access" {
		grant = MCPKey{}
		_, err = tx.Exec(`DELETE FROM mcp_oauth_tokens WHERE digest = ?`, digest)
	} else {
		var found bool
		found, err = revokeMCPKeyTx(tx, grantID, "client", now)
		if !found {
			grant = MCPKey{}
		}
	}
	if err != nil {
		return MCPKey{}, fmt.Errorf("RevokeOAuthToken: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MCPKey{}, fmt.Errorf("RevokeOAuthToken commit: %w", err)
	}
	return grant, nil
}

// PruneOAuth drops expired tokens, revokes the grants that no longer hold a
// live refresh token, and removes unused clients past their day.
func (r *Repo) PruneOAuth(now int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("PruneOAuth: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	if _, err := tx.Exec(`DELETE FROM mcp_oauth_tokens WHERE expires_at <= ?`, now); err != nil {
		return fmt.Errorf("PruneOAuth tokens: %w", err)
	}
	idle, err := grantIDs(tx, `SELECT id FROM mcp_keys k WHERE k.kind = 'oauth' AND k.revoked_at = 0
		AND NOT EXISTS (SELECT 1 FROM mcp_oauth_tokens t
			WHERE t.grant_id = k.id AND t.kind = 'refresh' AND t.spent_at = 0 AND t.expires_at > ?)`, now)
	if err != nil {
		return fmt.Errorf("PruneOAuth grants: %w", err)
	}
	for _, id := range idle {
		if _, err := revokeMCPKeyTx(tx, id, "expired", now); err != nil {
			return fmt.Errorf("PruneOAuth revoke: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM mcp_oauth_clients WHERE created_at <= ? AND `+unusedClient,
		now-OAuthUnusedClientTTL); err != nil {
		return fmt.Errorf("PruneOAuth clients: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PruneOAuth commit: %w", err)
	}
	return nil
}

// OAuthTokenCount is how many token rows a grant has, spent ones included.
func (r *Repo) OAuthTokenCount(grantID string) (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT count(*) FROM mcp_oauth_tokens WHERE grant_id = ?`, grantID).Scan(&n); err != nil {
		return 0, fmt.Errorf("OAuthTokenCount: %w", err)
	}
	return n, nil
}
