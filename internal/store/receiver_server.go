package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ReceiverServer is the append-only rest-server BombVault set up on this host
// for group members to copy their off-site backups to. There is at most one.
type ReceiverServer struct {
	ContainerName string
	// Folder is the data folder relative to the host data mount, as it was
	// picked; HostPath is the same folder as Docker on the host sees it.
	Folder   string
	HostPath string
	Port     int
	User     string
	// PasswordEnc is the server's login password sealed with this instance's
	// key (internal/secret). It is never logged or returned in the clear
	// outside the group.
	PasswordEnc []byte
	// Host is the address members reach the server at, empty when it is the
	// address they reach this instance at.
	Host      string
	CreatedAt int64
	// Check is the last append-only check: "protected", "unprotected",
	// "inconclusive", or empty before the first one.
	Check       string
	CheckDetail string
	CheckedAt   int64
}

const receiverServerCols = `container_name, folder, host_path, port, rest_user, password_enc, host, created_at, check_verdict, check_detail, checked_at`

// GetReceiverServer returns the receiving server, and false when none is set
// up.
func (r *Repo) GetReceiverServer() (ReceiverServer, bool, error) {
	var s ReceiverServer
	err := r.db.QueryRow(`SELECT `+receiverServerCols+` FROM receiver_server WHERE id = 1`).Scan(
		&s.ContainerName, &s.Folder, &s.HostPath, &s.Port, &s.User, &s.PasswordEnc, &s.Host,
		&s.CreatedAt, &s.Check, &s.CheckDetail, &s.CheckedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceiverServer{}, false, nil
	}
	if err != nil {
		return ReceiverServer{}, false, fmt.Errorf("GetReceiverServer: %w", err)
	}
	return s, true, nil
}

// SaveReceiverServer replaces the receiving server with s. A zero CreatedAt
// becomes now.
func (r *Repo) SaveReceiverServer(s ReceiverServer) error {
	if s.CreatedAt == 0 {
		s.CreatedAt = time.Now().Unix()
	}
	if s.PasswordEnc == nil {
		s.PasswordEnc = []byte{}
	}
	_, err := r.db.Exec(`INSERT OR REPLACE INTO receiver_server (id, `+receiverServerCols+`)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ContainerName, s.Folder, s.HostPath, s.Port, s.User, s.PasswordEnc, s.Host,
		s.CreatedAt, s.Check, s.CheckDetail, s.CheckedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveReceiverServer: %w", err)
	}
	return nil
}

// RecordReceiverServerCheck stores the outcome of an append-only check.
func (r *Repo) RecordReceiverServerCheck(verdict, detail string) error {
	_, err := r.db.Exec(`UPDATE receiver_server SET check_verdict = ?, check_detail = ?, checked_at = ? WHERE id = 1`,
		verdict, detail, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("RecordReceiverServerCheck: %w", err)
	}
	return nil
}

// DeleteReceiverServer forgets the receiving server and the logins it handed
// out. Its container and data are left alone.
func (r *Repo) DeleteReceiverServer() error {
	if _, err := r.db.Exec(`DELETE FROM receiver_server WHERE id = 1; DELETE FROM receiver_logins;`); err != nil {
		return fmt.Errorf("DeleteReceiverServer: %w", err)
	}
	return nil
}

// ReceiverLogin is the login a group member got on the receiving server.
type ReceiverLogin struct {
	MemberID   string
	MemberName string
	User       string
	// PasswordEnc is sealed with this instance's key, like the server's own.
	PasswordEnc []byte
	CreatedAt   int64
}

const receiverLoginCols = `member_id, member_name, rest_user, password_enc, created_at`

func scanReceiverLogin(row interface{ Scan(...any) error }) (ReceiverLogin, error) {
	var l ReceiverLogin
	err := row.Scan(&l.MemberID, &l.MemberName, &l.User, &l.PasswordEnc, &l.CreatedAt)
	return l, err
}

// ListReceiverLogins returns every member's login, oldest first.
func (r *Repo) ListReceiverLogins() ([]ReceiverLogin, error) {
	rows, err := r.db.Query(`SELECT ` + receiverLoginCols + ` FROM receiver_logins ORDER BY created_at, member_id`)
	if err != nil {
		return nil, fmt.Errorf("ListReceiverLogins: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only rows
	var out []ReceiverLogin
	for rows.Next() {
		l, err := scanReceiverLogin(rows)
		if err != nil {
			return nil, fmt.Errorf("ListReceiverLogins: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetReceiverLogin returns the login of memberID, and false when it has none.
func (r *Repo) GetReceiverLogin(memberID string) (ReceiverLogin, bool, error) {
	l, err := scanReceiverLogin(r.db.QueryRow(`SELECT `+receiverLoginCols+` FROM receiver_logins WHERE member_id = ?`, memberID))
	if errors.Is(err, sql.ErrNoRows) {
		return ReceiverLogin{}, false, nil
	}
	if err != nil {
		return ReceiverLogin{}, false, fmt.Errorf("GetReceiverLogin: %w", err)
	}
	return l, true, nil
}

// CreateReceiverLogin stores a new login. A zero CreatedAt becomes now.
func (r *Repo) CreateReceiverLogin(l ReceiverLogin) error {
	if l.CreatedAt == 0 {
		l.CreatedAt = time.Now().Unix()
	}
	if l.PasswordEnc == nil {
		l.PasswordEnc = []byte{}
	}
	_, err := r.db.Exec(`INSERT INTO receiver_logins (`+receiverLoginCols+`) VALUES (?, ?, ?, ?, ?)`,
		l.MemberID, l.MemberName, l.User, l.PasswordEnc, l.CreatedAt)
	if err != nil {
		return fmt.Errorf("CreateReceiverLogin: %w", err)
	}
	return nil
}

// RenameReceiverLogin records a new name for the login of memberID.
func (r *Repo) RenameReceiverLogin(memberID, name string) error {
	if _, err := r.db.Exec(`UPDATE receiver_logins SET member_name = ? WHERE member_id = ?`, name, memberID); err != nil {
		return fmt.Errorf("RenameReceiverLogin: %w", err)
	}
	return nil
}

// DeleteReceiverLogin forgets the login of memberID.
func (r *Repo) DeleteReceiverLogin(memberID string) error {
	if _, err := r.db.Exec(`DELETE FROM receiver_logins WHERE member_id = ?`, memberID); err != nil {
		return fmt.Errorf("DeleteReceiverLogin: %w", err)
	}
	return nil
}
