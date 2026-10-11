package store_test

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A sealed BLOB column is found by its name and rewritten by rowid. One on a
// table without a rowid, or one that is no BLOB, would be skipped or break a
// settings restore of another install.
func TestEverySealedColumnCanBeSealedAgain(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	columns, err := store.SealedColumns(db)
	if err != nil {
		t.Fatal(err)
	}
	var text []string
	for _, c := range columns {
		//nolint:gosec // G202: table and column come from the schema under test
		if _, err := db.Exec(`SELECT rowid, ` + c.Column + ` FROM ` + c.Table + ` LIMIT 0`); err != nil {
			t.Errorf("%s.%s cannot be read by rowid: %v", c.Table, c.Column, err)
		}
		if c.Text {
			text = append(text, c.Column)
			continue
		}
		var typ string
		if err := db.QueryRow(`SELECT type FROM pragma_table_info(?) WHERE name = ?`, c.Table, c.Column).Scan(&typ); err != nil || typ != "BLOB" {
			t.Errorf("%s.%s is declared %q, %v; a sealed column named _enc has to be a BLOB", c.Table, c.Column, typ, err)
		}
	}
	slices.Sort(text)
	want := slices.Sorted(slices.Values(store.SealedSettingsText))
	if !slices.Equal(text, want) {
		t.Fatalf("the settings table has the sealed text columns %v, the list says %v", text, want)
	}
	if !slices.ContainsFunc(columns, func(c store.SealedColumn) bool { return c.Table == "backup_key" && c.Column == "key_enc" }) {
		t.Fatal("the adopted backup key is not among the sealed columns")
	}
}

// stagedCopy migrates a database file and returns its path, closed.
func stagedCopy(t *testing.T, fill func(*sql.DB)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bombvault.sqlite")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	fill(db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResealDatabaseRewritesBothFormsAndLeavesNoLogBehind(t *testing.T) {
	path := stagedCopy(t, func(db *sql.DB) {
		for _, q := range []string{
			`UPDATE settings SET cloud_conf = '` + base64.StdEncoding.EncodeToString([]byte("old:cloud")) + `', rclone_conf = '', notify_conf = 'not base64 !'`,
			`INSERT INTO group_state (id, secret_enc) VALUES (1, x'6f6c643a67726f7570') ON CONFLICT (id) DO UPDATE SET secret_enc = excluded.secret_enc`,
		} {
			if _, err := db.Exec(q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
	})

	db, err := store.OpenStaged(path)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	err = store.ResealDatabase(db, func(c store.SealedColumn, sealed []byte) []byte {
		seen = append(seen, c.Table+"."+c.Column+"="+string(sealed))
		rest, ok := bytes.CutPrefix(sealed, []byte("old:"))
		if !ok {
			return nil
		}
		return append([]byte("new:"), rest...)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CloseStaged(db); err != nil {
		t.Fatal(err)
	}
	slices.Sort(seen)
	if !slices.Equal(seen, []string{"group_state.secret_enc=old:group", "settings.cloud_conf=old:cloud"}) {
		t.Fatalf("the sealed values shown were %q, want the cloud sign-in and the group secret alone", seen)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if info, err := os.Stat(path + suffix); err == nil && info.Size() > 0 {
			t.Fatalf("%s holds %d bytes after the close, which a move of the file would lose", suffix, info.Size())
		}
	}

	again, err := store.OpenStaged(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close() //nolint:errcheck // test database
	var cloud, notify string
	var group []byte
	if err := again.QueryRow(`SELECT cloud_conf, notify_conf FROM settings`).Scan(&cloud, &notify); err != nil {
		t.Fatal(err)
	}
	if err := again.QueryRow(`SELECT secret_enc FROM group_state`).Scan(&group); err != nil {
		t.Fatal(err)
	}
	if plain, _ := base64.StdEncoding.DecodeString(cloud); string(plain) != "new:cloud" || string(group) != "new:group" || notify != "not base64 !" {
		t.Fatalf("after the reseal: cloud %q, group %q, notify %q", plain, group, notify)
	}
}

func TestAnAdoptedKeyAndTheLoginGoIntoAnOlderDatabase(t *testing.T) {
	path := stagedCopy(t, func(db *sql.DB) {
		for _, q := range []string{
			`DROP TABLE backup_key`,
			`UPDATE settings SET auth_password_hash = 'old-hash', session_epoch = 'old-epoch', totp_enabled = 1, totp_secret = 'old-totp'`,
			`INSERT INTO passkeys (id, credential_id, public_key, name, created_at) VALUES ('old', x'01', x'02', 'old key', 1)`,
		} {
			if _, err := db.Exec(q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
	})
	live := store.OpenMem(t)
	if err := store.Migrate(live); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE settings SET auth_password_hash = 'live-hash', session_epoch = 'live-epoch'`,
		`INSERT INTO passkeys (id, credential_id, public_key, name, created_at) VALUES ('live', x'03', x'04', 'live key', 2)`,
	} {
		if _, err := live.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	db, err := store.OpenStaged(path)
	if err != nil {
		t.Fatal(err)
	}
	if has, err := store.HasAdoptedKey(db); err != nil || has {
		t.Fatalf("a database without the table has an adopted key: %v, %v", has, err)
	}
	if err := store.SetAdoptedKeyIn(db, []byte("sealed"), store.KeyFromKit, `{"count":2}`); err != nil {
		t.Fatal(err)
	}
	if has, err := store.HasAdoptedKey(db); err != nil || !has {
		t.Fatalf("after SetAdoptedKeyIn: %v, %v", has, err)
	}
	if err := store.New(live).CarryLoginInto(db); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseStaged(db); err != nil {
		t.Fatal(err)
	}

	// The boot migrates the file it swaps in, and has to take the table as it is.
	reopened, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close() //nolint:errcheck // test database
	if err := store.Migrate(reopened); err != nil {
		t.Fatalf("migrating the database the key went into: %v", err)
	}
	r := store.New(reopened)
	key, found, err := r.AdoptedKey()
	if err != nil || !found || string(key.KeyEnc) != "sealed" || key.Source != store.KeyFromKit || key.Lost != `{"count":2}` || key.EncryptionBefore != nil {
		t.Fatalf("the adopted key reads back as %+v, %v, %v", key, found, err)
	}
	settings, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AuthPasswordHash != "live-hash" || settings.SessionEpoch != "live-epoch" || settings.TOTPEnabled || settings.TOTPSecret != "" {
		t.Fatalf("the login reads back as hash %q, epoch %q, second factor %v", settings.AuthPasswordHash, settings.SessionEpoch, settings.TOTPEnabled)
	}
	var names []string
	rows, err := reopened.Query(`SELECT name FROM passkeys`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test query
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"live key"}) {
		t.Fatalf("the passkeys are %v, want the live one alone", names)
	}
}
