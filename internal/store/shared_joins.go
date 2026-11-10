package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SharedRepoServer is the second rest-server a receiving server runs for its
// shared repository. It serves every login the whole data folder, which holds
// that one repository, so it runs without --private-repos and apart from the
// receiver whose logins each see a folder of their own.
type SharedRepoServer struct {
	ContainerName string
	// Folder is the data folder relative to the host data mount, HostPath the
	// same folder as Docker on the host sees it.
	Folder   string
	HostPath string
	Port     int
	// User and PasswordEnc are this server's own login, which its append-only
	// check signs in with.
	User        string
	PasswordEnc []byte
	// Repo is the repository as this server hands it to restic, the location
	// its SharedRepo row carries.
	Repo        string
	CreatedAt   int64
	Check       string
	CheckDetail string
	CheckedAt   int64
}

const sharedRepoServerCols = `container_name, folder, host_path, port, user, password_enc, repo, created_at, check_verdict, check_detail, checked_at`

// GetSharedRepoServer returns the shared repository's server, and false when
// none is set up.
func (r *Repo) GetSharedRepoServer() (SharedRepoServer, bool, error) {
	var s SharedRepoServer
	err := r.db.QueryRow(`SELECT `+sharedRepoServerCols+` FROM shared_repo_server WHERE id = 1`).Scan(
		&s.ContainerName, &s.Folder, &s.HostPath, &s.Port, &s.User, &s.PasswordEnc, &s.Repo,
		&s.CreatedAt, &s.Check, &s.CheckDetail, &s.CheckedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SharedRepoServer{}, false, nil
	}
	if err != nil {
		return SharedRepoServer{}, false, fmt.Errorf("GetSharedRepoServer: %w", err)
	}
	return s, true, nil
}

// SaveSharedRepoServer replaces the shared repository's server with s. A zero
// CreatedAt becomes now.
func (r *Repo) SaveSharedRepoServer(s SharedRepoServer) error {
	if s.CreatedAt == 0 {
		s.CreatedAt = time.Now().Unix()
	}
	if s.PasswordEnc == nil {
		s.PasswordEnc = []byte{}
	}
	_, err := r.db.Exec(`INSERT OR REPLACE INTO shared_repo_server (id, `+sharedRepoServerCols+`)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ContainerName, s.Folder, s.HostPath, s.Port, s.User, s.PasswordEnc, s.Repo,
		s.CreatedAt, s.Check, s.CheckDetail, s.CheckedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveSharedRepoServer: %w", err)
	}
	return nil
}

// RecordSharedRepoServerCheck stores the outcome of an append-only check.
func (r *Repo) RecordSharedRepoServerCheck(verdict, detail string) error {
	_, err := r.db.Exec(`UPDATE shared_repo_server SET check_verdict = ?, check_detail = ?, checked_at = ? WHERE id = 1`,
		verdict, detail, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("RecordSharedRepoServerCheck: %w", err)
	}
	return nil
}

// The directions of a join: one a member asked this server for, and one this
// server asked a member for.
const (
	SharedJoinIn  = "in"
	SharedJoinOut = "out"
)

// The states of a join. A join waits as asked until a person on the owner
// answers. Allowed hands the member a one-time key, and joined says the member
// added its own and the one-time key is gone. Removed is a member that left or
// was taken out.
const (
	SharedJoinAsked   = "asked"
	SharedJoinAllowed = "allowed"
	SharedJoinRefused = "refused"
	SharedJoinJoined  = "joined"
	SharedJoinRemoved = "removed"
)

// SharedJoin is a server's way into a shared repository as one side keeps it.
type SharedJoin struct {
	Direction string
	// MemberID is the other server: the member on the owner, the owner on the
	// member.
	MemberID   string
	MemberName string
	// Host is the restic host of the server that joins.
	Host  string
	State string
	// Repo is the repository as the member hands it to restic, once it knows.
	Repo string
	// User and LoginEnc are the member's login at the shared repository's
	// server, its password sealed with this server's key.
	User     string
	LoginEnc []byte
	// OnceEnc is the sealed password of the one-time key and OnceKey that
	// key's id, both kept only until the member added its own key.
	OnceEnc   []byte
	OnceKey   string
	CreatedAt int64
	DecidedAt int64
}

const sharedJoinCols = `direction, member_id, member_name, host, state, repo, user, login_enc, once_enc, once_key, created_at, decided_at`

// SaveSharedJoin stores j under its direction and server. A zero CreatedAt
// becomes now.
func (r *Repo) SaveSharedJoin(j SharedJoin) error {
	if j.CreatedAt == 0 {
		j.CreatedAt = time.Now().Unix()
	}
	_, err := r.db.Exec(`INSERT OR REPLACE INTO shared_repo_joins (`+sharedJoinCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.Direction, j.MemberID, j.MemberName, j.Host, j.State, j.Repo, j.User,
		nonNil(j.LoginEnc), nonNil(j.OnceEnc), j.OnceKey, j.CreatedAt, j.DecidedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveSharedJoin: %w", err)
	}
	return nil
}

func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

// GetSharedJoin returns the join in direction with the server memberID.
func (r *Repo) GetSharedJoin(direction, memberID string) (SharedJoin, bool, error) {
	all, err := r.sharedJoins(`SELECT `+sharedJoinCols+` FROM shared_repo_joins WHERE direction = ? AND member_id = ?`, direction, memberID)
	if err != nil || len(all) == 0 {
		return SharedJoin{}, false, err
	}
	return all[0], true, nil
}

// ListSharedJoins returns every join in direction, oldest first.
func (r *Repo) ListSharedJoins(direction string) ([]SharedJoin, error) {
	return r.sharedJoins(`SELECT `+sharedJoinCols+` FROM shared_repo_joins WHERE direction = ? ORDER BY created_at, member_id`, direction)
}

// DeleteSharedJoin forgets the join in direction with the server memberID.
func (r *Repo) DeleteSharedJoin(direction, memberID string) error {
	if _, err := r.db.Exec(`DELETE FROM shared_repo_joins WHERE direction = ? AND member_id = ?`, direction, memberID); err != nil {
		return fmt.Errorf("DeleteSharedJoin: %w", err)
	}
	return nil
}

func (r *Repo) sharedJoins(query string, args ...any) ([]SharedJoin, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list shared repository joins: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []SharedJoin
	for rows.Next() {
		var j SharedJoin
		if err := rows.Scan(&j.Direction, &j.MemberID, &j.MemberName, &j.Host, &j.State, &j.Repo, &j.User,
			&j.LoginEnc, &j.OnceEnc, &j.OnceKey, &j.CreatedAt, &j.DecidedAt); err != nil {
			return nil, fmt.Errorf("list shared repository joins: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
