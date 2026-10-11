package store

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"time"
)

// A settings database holds what its install sealed in two forms: BLOB columns
// whose name ends in _enc, and the settings columns of SealedSettingsText,
// which hold the sealed bytes as base64. The functions here work on a
// database file that is not the live one, such as a staged settings restore,
// and on whatever tables and columns that file has, since it may be older.

// SealedSettingsText are the settings columns that hold a sealed value as
// base64 text. totp_secret is sealed the same way and left out: it belongs to
// the login, which CarryLoginInto replaces.
var SealedSettingsText = []string{"rclone_conf", "notify_conf", "cloud_conf", "registry_auths", "cloud_cred_sets"}

// loginSettings are the settings columns that make up the login. The password
// hash and the recovery codes are peppered with APP_KEY and the second factor
// is sealed with it, so none of them means anything under another key.
var loginSettings = []string{"auth_password_hash", "totp_secret", "totp_enabled", "totp_recovery", "session_epoch"}

const backupKeyTable = `CREATE TABLE IF NOT EXISTS backup_key (
  id         INTEGER PRIMARY KEY CHECK (id = 1),
  key_enc    BLOB    NOT NULL,
  source     TEXT    NOT NULL,
  adopted_at INTEGER NOT NULL
);`

// backupKeyNotes adds what an adoption has to remember: the encryption
// setting it replaced, to put back on an undo, and the sealed values a
// settings restore could not open. The migration takes the second column for
// both, so they are added together.
const backupKeyNotes = `ALTER TABLE backup_key ADD COLUMN encryption_before INTEGER;
ALTER TABLE backup_key ADD COLUMN lost TEXT NOT NULL DEFAULT '';`

// SealedColumn names a column that holds sealed bytes. Text marks the base64
// form.
type SealedColumn struct {
	Table, Column string
	Text          bool
}

type columnQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func tableColumns(q columnQuerier, table string) ([]string, error) {
	//nolint:gosec // G202: table comes from sqlite_master or is a literal of this package
	rows, err := q.Query(`SELECT name FROM pragma_table_info('` + table + `')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// SealedColumns lists the sealed columns db has.
func SealedColumns(db columnQuerier) ([]SealedColumn, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("SealedColumns: %w", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close() //nolint:errcheck,gosec // the scan error is the one to report
			return nil, fmt.Errorf("SealedColumns: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("SealedColumns: %w", err)
	}
	var out []SealedColumn
	for _, table := range tables {
		cols, err := tableColumns(db, table)
		if err != nil {
			return nil, fmt.Errorf("SealedColumns %s: %w", table, err)
		}
		for _, col := range cols {
			switch {
			case strings.HasSuffix(col, "_enc"):
				out = append(out, SealedColumn{Table: table, Column: col})
			case table == "settings" && slices.Contains(SealedSettingsText, col):
				out = append(out, SealedColumn{Table: table, Column: col, Text: true})
			}
		}
	}
	return out, nil
}

// OpenStaged opens a settings database that is not the live one. CloseStaged
// has to close it, so that everything written is in the file itself before
// the file is moved.
func OpenStaged(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("OpenStaged: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// CloseStaged folds the write-ahead log of db into its file and closes it.
func CloseStaged(db *sql.DB) error {
	_, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	if cErr := db.Close(); err == nil {
		err = cErr
	}
	if err != nil {
		return fmt.Errorf("CloseStaged: %w", err)
	}
	return nil
}

// ResealDatabase hands every sealed value of db to reseal, with the column it
// is in, and stores what it returns. A value reseal answers nil for stays as
// it is.
func ResealDatabase(db *sql.DB, reseal func(c SealedColumn, sealed []byte) []byte) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("ResealDatabase: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	cols, err := SealedColumns(tx)
	if err != nil {
		return err
	}
	type cell struct {
		rowid int64
		value []byte
	}
	for _, c := range cols {
		//nolint:gosec // G202: table and column come from the schema of db
		rows, err := tx.Query(`SELECT rowid, ` + c.Column + ` FROM ` + c.Table + ` WHERE length(` + c.Column + `) > 0`)
		if err != nil {
			return fmt.Errorf("ResealDatabase %s.%s: %w", c.Table, c.Column, err)
		}
		var cells []cell
		for rows.Next() {
			var x cell
			if err := rows.Scan(&x.rowid, &x.value); err != nil {
				rows.Close() //nolint:errcheck,gosec // the scan error is the one to report
				return fmt.Errorf("ResealDatabase %s.%s: %w", c.Table, c.Column, err)
			}
			cells = append(cells, x)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("ResealDatabase %s.%s: %w", c.Table, c.Column, err)
		}
		for _, x := range cells {
			sealed := x.value
			if c.Text {
				// Text that is no base64 was never sealed, so it is not shown.
				if sealed, err = base64.StdEncoding.DecodeString(string(x.value)); err != nil {
					continue
				}
			}
			out := reseal(c, sealed)
			if out == nil {
				continue
			}
			var stored any = out
			if c.Text {
				stored = base64.StdEncoding.EncodeToString(out)
			}
			//nolint:gosec // G202: table and column come from the schema of db
			if _, err := tx.Exec(`UPDATE `+c.Table+` SET `+c.Column+` = ? WHERE rowid = ?`, stored, x.rowid); err != nil {
				return fmt.Errorf("ResealDatabase %s.%s: %w", c.Table, c.Column, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ResealDatabase: %w", err)
	}
	return nil
}

// HasAdoptedKey reports whether db, which may be older than the table, holds
// an adopted backup key.
func HasAdoptedKey(db *sql.DB) (bool, error) {
	cols, err := tableColumns(db, "backup_key")
	if err != nil || len(cols) == 0 {
		return false, err
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM backup_key`).Scan(&n); err != nil {
		return false, fmt.Errorf("HasAdoptedKey: %w", err)
	}
	return n > 0, nil
}

// SetAdoptedKeyIn stores an adopted backup key in a database that is not the
// live one, with the list of sealed values the restore of that database could
// not open. The table and its later columns are created where the file is
// older than them, in the order the migrations look for them.
func SetAdoptedKeyIn(db *sql.DB, enc []byte, source, lost string) error {
	if _, err := db.Exec(backupKeyTable); err != nil {
		return fmt.Errorf("SetAdoptedKeyIn: %w", err)
	}
	cols, err := tableColumns(db, "backup_key")
	if err != nil {
		return fmt.Errorf("SetAdoptedKeyIn: %w", err)
	}
	if !slices.Contains(cols, "lost") {
		if _, err := db.Exec(backupKeyNotes); err != nil {
			return fmt.Errorf("SetAdoptedKeyIn: %w", err)
		}
	}
	_, err = db.Exec(`
		INSERT INTO backup_key (id, key_enc, source, adopted_at, lost) VALUES (1, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET key_enc = excluded.key_enc, source = excluded.source,
			adopted_at = excluded.adopted_at, encryption_before = NULL, lost = excluded.lost`,
		enc, source, time.Now().Unix(), lost)
	if err != nil {
		return fmt.Errorf("SetAdoptedKeyIn: %w", err)
	}
	return nil
}

// CarryLoginInto puts this install's login into dst in place of the one dst
// has: the password, the second factor, the session epoch and the passkeys.
func (r *Repo) CarryLoginInto(dst *sql.DB) error {
	there, err := tableColumns(dst, "settings")
	if err != nil {
		return fmt.Errorf("CarryLoginInto: %w", err)
	}
	for _, col := range loginSettings {
		if !slices.Contains(there, col) {
			continue
		}
		var value any
		//nolint:gosec // G202: col is one of the literals in loginSettings
		if err := r.db.QueryRow(`SELECT ` + col + ` FROM settings WHERE id = 1`).Scan(&value); err != nil {
			return fmt.Errorf("CarryLoginInto %s: %w", col, err)
		}
		//nolint:gosec // G202: col is one of the literals in loginSettings
		if _, err := dst.Exec(`UPDATE settings SET `+col+` = ? WHERE id = 1`, value); err != nil {
			return fmt.Errorf("CarryLoginInto %s: %w", col, err)
		}
	}
	return r.carryPasskeysInto(dst)
}

func (r *Repo) carryPasskeysInto(dst *sql.DB) error {
	there, err := tableColumns(dst, "passkeys")
	if err != nil {
		return fmt.Errorf("CarryLoginInto passkeys: %w", err)
	}
	if len(there) == 0 {
		return nil
	}
	here, err := tableColumns(r.db, "passkeys")
	if err != nil {
		return fmt.Errorf("CarryLoginInto passkeys: %w", err)
	}
	shared := slices.DeleteFunc(here, func(col string) bool { return !slices.Contains(there, col) })
	if _, err := dst.Exec(`DELETE FROM passkeys`); err != nil {
		return fmt.Errorf("CarryLoginInto passkeys: %w", err)
	}
	// The driver writes an empty slice as NULL, which the columns refuse. So
	// each value comes with its type, and an empty BLOB goes into the statement
	// as a literal.
	types := make([]string, len(shared))
	for i, col := range shared {
		types[i] = "typeof(" + col + ")"
	}
	//nolint:gosec // G202: the column names come from the schema of both databases
	rows, err := r.db.Query(`SELECT ` + strings.Join(shared, ", ") + `, ` + strings.Join(types, ", ") + ` FROM passkeys`)
	if err != nil {
		return fmt.Errorf("CarryLoginInto passkeys: %w", err)
	}
	type passkey struct {
		marks  []string
		values []any
	}
	var keys []passkey
	for rows.Next() {
		row := make([]any, len(shared))
		kinds := make([]string, len(shared))
		ptrs := make([]any, 0, 2*len(shared))
		for i := range row {
			ptrs = append(ptrs, &row[i])
		}
		for i := range kinds {
			ptrs = append(ptrs, &kinds[i])
		}
		if err := rows.Scan(ptrs...); err != nil {
			rows.Close() //nolint:errcheck,gosec // the scan error is the one to report
			return fmt.Errorf("CarryLoginInto passkeys: %w", err)
		}
		var key passkey
		for i, kind := range kinds {
			if b, _ := row[i].([]byte); kind == "blob" && len(b) == 0 {
				key.marks = append(key.marks, "x''")
				continue
			}
			key.marks = append(key.marks, "?")
			key.values = append(key.values, row[i])
		}
		keys = append(keys, key)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("CarryLoginInto passkeys: %w", err)
	}
	for _, key := range keys {
		//nolint:gosec // G202: the column names come from the schema of both databases
		if _, err := dst.Exec(`INSERT INTO passkeys (`+strings.Join(shared, ", ")+`) VALUES (`+strings.Join(key.marks, ", ")+`)`, key.values...); err != nil {
			return fmt.Errorf("CarryLoginInto passkeys: %w", err)
		}
	}
	return nil
}
