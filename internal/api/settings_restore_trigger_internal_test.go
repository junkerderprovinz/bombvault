package api

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func execIn(t *testing.T, path, query string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func triggersIn(t *testing.T, path string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test file
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'trigger' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test file
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	return out
}

func TestATriggerInAStagedDatabaseCannotUndoTheCarriedLogin(t *testing.T) {
	f := newPlacementFixture(t)
	own := f.svc.cfg.AppKey
	f.withPassword("this server's password")
	path := bareSettings(t, oldKey, "the old password", false)
	execIn(t, path, `CREATE TRIGGER wipe AFTER UPDATE OF session_epoch ON settings
		BEGIN UPDATE settings SET auth_password_hash = '' WHERE id = 1; END;
		CREATE VIEW leak AS SELECT auth_password_hash FROM settings;`)
	staged := f.stage(path)
	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("takeOverStagedSettings: %v", err)
	}
	if left := triggersIn(t, staged); len(left) > 0 {
		t.Fatalf("triggers left in the staged database: %v", left)
	}
	if hash := restoredLogin(t, staged, own); !secret.VerifyPassword(own, "this server's password", hash) {
		t.Fatalf("the restored login is %q, not this server's", hash)
	}
	// The migration that made BombVault's own triggers runs again at the boot.
	triggers := triggersIn(t, staged)
	if len(triggers) == 0 || !strings.HasPrefix(triggers[0], "one_line_") || strings.Contains(strings.Join(triggers, " "), "wipe") {
		t.Fatalf("triggers after the boot: %v", triggers)
	}
}

func TestACopiedCheckValueDoesNotKeepTheFilesLogin(t *testing.T) {
	f := newPlacementFixture(t)
	own := f.svc.cfg.AppKey
	f.withPassword("this server's password")
	path := bareSettings(t, oldKey, "the old password", false)
	_, st, closeDB := fileInstall(t, path, oldKey)
	if err := st.SetKeyCheck(secret.KeyCheck(own)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	closeDB()
	staged := f.stage(path)
	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("takeOverStagedSettings: %v", err)
	}
	if hash := restoredLogin(t, staged, own); !secret.VerifyPassword(own, "this server's password", hash) {
		t.Fatalf("the restored login is %q, not this server's", hash)
	}
}
