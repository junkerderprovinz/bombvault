package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/ageseal"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// oldKey is the key of the install whose backups an emergency brings back.
var oldKey = strings.Repeat("d", 64)

// kitOfTheOldInstall is the recovery kit of an install under oldKey, with
// encryption on and a shared rest-server sign-in.
func kitOfTheOldInstall(t *testing.T) string {
	t.Helper()
	old := newPlacementFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	if err := old.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "rest-secret"}); err != nil {
		t.Fatal(err)
	}
	return writtenKit(t, old)
}

func (f *placementFixture) emergency(method, path string, body any) (map[string]any, emergencyView) {
	f.t.Helper()
	res := f.do(method, path, body)
	var view emergencyView
	if raw, ok := res["emergency"]; ok {
		b, err := json.Marshal(raw)
		if err != nil {
			f.t.Fatal(err)
		}
		if err := json.Unmarshal(b, &view); err != nil {
			f.t.Fatal(err)
		}
	}
	return res, view
}

func (f *placementFixture) adoptedRow() (store.AdoptedKey, bool) {
	f.t.Helper()
	row, found, err := f.st.AdoptedKey()
	if err != nil {
		f.t.Fatal(err)
	}
	return row, found
}

func placeStates(view emergencyView) map[string]string {
	out := map[string]string{}
	for _, p := range view.Session.Places {
		out[p.Kind+"/"+p.Domain+p.Name] = p.State
	}
	return out
}

// keyRuns returns kind and status of the runs the key events wrote, oldest
// first, and fails when one of them carries a key.
func (f *placementFixture) keyRuns(secrets ...string) []string {
	f.t.Helper()
	rows, err := f.db.Query(`SELECT kind, status, coalesce(error, '') FROM runs WHERE target_id = ? ORDER BY rowid`, store.KeyTargetID)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test query
	var out []string
	for rows.Next() {
		var kind, status, why string
		if err := rows.Scan(&kind, &status, &why); err != nil {
			f.t.Fatal(err)
		}
		for _, secret := range secrets {
			if strings.Contains(why, secret) {
				f.t.Fatalf("the activity log carries a secret in %q", why)
			}
		}
		out = append(out, kind+" "+status)
	}
	return out
}

func TestAnEmergencyByKitTakesTheKeyOverOnlyWhenAskedTo(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	before, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	res, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res["ok"] != true || view.Session == nil {
		t.Fatalf("read: %v", res)
	}
	if b, _ := json.Marshal(res); strings.Contains(string(b), oldKey) || strings.Contains(string(b), restickey.Derive(oldKey)) || strings.Contains(string(b), "rest-secret") {
		t.Fatal("the answer to reading a kit carries one of its secrets")
	}
	states := placeStates(view)
	for place, want := range map[string]string{
		"local/containers": placeReadable, "local/vms": placeReadable, "local/files": placeReadable,
		"named/Cold": placeMissing, "destination/filesVault": placeNeedsSignIn,
	} {
		if states[place] != want {
			t.Errorf("%s is %q, want %q (all: %v)", place, states[place], want, states)
		}
	}
	if view.BackupKey != backupKeyOwn || view.Session.Source != store.KeyFromKit || !view.Session.Encrypted || view.Session.Refusal != "" || !view.Session.CloudSignIn {
		t.Fatalf("after the read: %+v with %+v", view, view.Session)
	}
	if _, found := f.adoptedRow(); found {
		t.Fatal("reading a kit stored its key")
	}
	if after, _ := f.st.GetSettings(); after.EncryptionEnabled != before.EncryptionEnabled || after.CloudConf != before.CloudConf {
		t.Fatal("reading a kit changed the settings")
	}

	res, view = f.emergency("POST", "/api/emergency/adopt", nil)
	if res["ok"] != true || view.BackupKey != backupKeyAdopted || view.Source != store.KeyFromKit || !view.Undoable {
		t.Fatalf("adopt: %v, %+v", res, view)
	}
	row, found := f.adoptedRow()
	if !found || bytes.Contains(row.KeyEnc, []byte(oldKey)) {
		t.Fatalf("the adopted key is stored: %v, in the clear: %v", found, bytes.Contains(row.KeyEnc, []byte(oldKey)))
	}
	settings, _ := f.st.GetSettings()
	if !settings.EncryptionEnabled || f.svc.ModeFor(settings).Password != restickey.Derive(oldKey) {
		t.Fatal("after the adoption the repositories are not opened with the password of the old key")
	}

	if res, _ := f.emergency("POST", "/api/emergency/sign-ins", nil); res["ok"] != true {
		t.Fatalf("sign-ins: %v", res)
	}
	settings, _ = f.st.GetSettings()
	if creds, err := f.svc.decodeCloud(settings); err != nil || creds.RESTUser != "bv" || creds.RESTPassword != "rest-secret" {
		t.Fatalf("the sign-in of the kit is stored as %+v, %v", creds, err)
	}

	res, view = f.emergency("DELETE", "/api/emergency", nil)
	if res["ok"] != true || view.BackupKey != backupKeyOwn || view.Session != nil {
		t.Fatalf("undo: %v, %+v", res, view)
	}
	if _, found := f.adoptedRow(); found {
		t.Fatal("the undone adoption left its key")
	}
	want := []string{"keyread success", "keyadopt success", "keysignins success", "keyundo success"}
	if got := f.keyRuns(oldKey, "rest-secret"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the activity log has %v, want %v", got, want)
	}
}

func TestAKeyThatOpensNothingIsNotTakenOver(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	for _, place := range append([]string{"rest:https://box:8000/vms"}, f.domainPath("containers"), f.domainPath("vms"), f.domainPath("files")) {
		f.eng.opens[place] = false
		f.eng.openErr[place] = errors.New("Fatal: wrong password or no key found")
	}

	res, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res["ok"] != true || placeStates(view)["local/vms"] != placeWrongKey || view.Session.Refusal != "adopt-nothing-readable" {
		t.Fatalf("read: %v, %+v", res, view.Session)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != false || res["code"] != "adopt-nothing-readable" {
		t.Fatalf("adopt: %v", res)
	}

	mistyped := strings.Replace(kit, oldKey, strings.Repeat("b", 64), 1)
	if res, _ := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": mistyped}); res["code"] != "kit-key-broken" {
		t.Fatalf("a mistyped kit: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/read", map[string]any{"key": "not a key"}); res["code"] != "emergency-key" {
		t.Fatalf("a typed key of the wrong shape: %v", res)
	}
	if _, found := f.adoptedRow(); found {
		t.Fatal("a refused emergency stored a key")
	}
	if got := f.keyRuns(oldKey); len(got) != 4 || got[1] != "keyadopt failed" || got[2] != "keyread failed" {
		t.Fatalf("the activity log has %v", got)
	}
}

func TestAServerWithBackupsOfItsOwnKeepsItsKey(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	f.backupRun(f.container("nginx", "").ID, time.Now().Unix()-3600)

	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if view.Session == nil || view.Session.Refusal != "adopt-own-backups" {
		t.Fatalf("read on a server with backups: %+v", view.Session)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "adopt-own-backups" {
		t.Fatalf("adopt: %v", res)
	}
	if _, found := f.adoptedRow(); found {
		t.Fatal("the refused adoption stored a key")
	}

	own := f.svc.cfg.AppKey
	if res, _ := f.emergency("POST", "/api/emergency/read", map[string]any{"key": own, "places": []string{"backups/vms"}}); res["ok"] != true {
		t.Fatalf("read with the server's own key: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "adopt-same-key" {
		t.Fatalf("adopting the key the server already has: %v", res)
	}
}

func TestAnAdoptedKeyIsReplacedOnlyWhileNothingWasWrittenUnderIt(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	other := strings.Repeat("e", 64)

	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("first adoption: %v", res)
	}
	f.emergency("POST", "/api/emergency/read", map[string]any{"key": other, "places": []string{"backups/vms"}})
	res, view := f.emergency("POST", "/api/emergency/adopt", nil)
	if res["ok"] != true || view.Source != store.KeyFromTyped {
		t.Fatalf("a second adoption before any backup: %v, %+v", res, view)
	}
	if got := f.svc.ModeFor(store.Settings{EncryptionEnabled: true}).Password; got != restickey.Derive(other) {
		t.Fatal("the second key did not replace the first")
	}

	f.backupRun(f.container("nginx", "").ID, time.Now().Unix()+5)
	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "adopt-own-backups" {
		t.Fatalf("an adoption over a key with backups: %v", res)
	}
	res, view = f.emergency("DELETE", "/api/emergency", nil)
	if res["code"] != "adopt-in-use" {
		t.Fatalf("undoing a key with backups: %v", res)
	}
	if _, view = f.emergency("GET", "/api/emergency", nil); view.BackupKey != backupKeyAdopted || view.Undoable {
		t.Fatalf("state after the refused undo: %+v", view)
	}
}

func TestALockedKeyIsOpenedAgainByTheKit(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	f.adopt(oldKey, strings.Repeat("e", 64))
	f.backupRun(f.container("nginx", "").ID, time.Now().Unix()+5)

	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if view.BackupKey != backupKeyLocked || view.Session.Refusal != "" {
		t.Fatalf("read under a locked key: %+v with %+v", view, view.Session)
	}
	res, view := f.emergency("POST", "/api/emergency/adopt", nil)
	if res["ok"] != true || view.BackupKey != backupKeyAdopted {
		t.Fatalf("adopting again: %v, %+v", res, view)
	}
	if key, err := f.svc.backupKey(); err != nil || key != oldKey {
		t.Fatalf("the backup key after adopting again: %v", err)
	}
}

func TestAnEmergencyEndsWithItsTimeAndOnFinish(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)

	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	f.svc.emergencyMu.Lock()
	f.svc.emergency.expires = time.Now().Add(-time.Second)
	f.svc.emergencyMu.Unlock()
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "emergency-ended" {
		t.Fatalf("adopting after the time is up: %v", res)
	}

	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	f.emergency("POST", "/api/emergency/adopt", nil)
	_, view := f.emergency("POST", "/api/emergency/finish", nil)
	if view.Session != nil || view.BackupKey != backupKeyAdopted {
		t.Fatalf("after finish: %+v", view)
	}
	f.svc.emergencyMu.Lock()
	held := f.svc.emergency
	f.svc.emergencyMu.Unlock()
	if held != nil {
		t.Fatal("the keys of a finished emergency are still in memory")
	}
}

func TestASealedKitOpensWithItsIdentityWhichReachesNoLog(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := ageseal.SealArmored([]byte(kit), []age.Recipient{identity.Recipient()})
	if err != nil {
		t.Fatal(err)
	}
	f := newPlacementFixture(t)

	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	var answers strings.Builder
	read := func(identity string) map[string]any {
		t.Helper()
		res, _ := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": string(sealed), "identity": identity})
		b, _ := json.Marshal(res)
		answers.Write(b)
		return res
	}

	if res := read(""); res["code"] != "kit-sealed" {
		t.Fatalf("a sealed kit without an identity: %v", res)
	}
	if res := read(stranger.String()); res["code"] != "kit-not-for-identity" {
		t.Fatalf("a sealed kit with another identity: %v", res)
	}
	broken := identity.String()[:40]
	if res := read(broken); res["code"] != "kit-identity" {
		t.Fatalf("a sealed kit with a cut identity: %v", res)
	}
	if _, view := f.emergency("GET", "/api/emergency", nil); view.Session != nil {
		t.Fatal("a refused read started an emergency")
	}
	res := read(identity.String())
	if res["ok"] != true {
		t.Fatalf("a sealed kit with its identity: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}

	for _, secret := range []string{identity.String(), stranger.String(), broken, oldKey} {
		if strings.Contains(logged.String(), secret) {
			t.Fatal("a secret reached the log")
		}
		if strings.Contains(answers.String(), secret) {
			t.Fatal("a secret came back in an answer")
		}
	}
	f.keyRuns(identity.String(), stranger.String(), broken, oldKey)
}

func TestUndoingAnAdoptionPutsTheEncryptionSettingBack(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.EncryptionEnabled = false; return nil }); err != nil {
		t.Fatal(err)
	}
	encryption := func() bool {
		t.Helper()
		settings, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		return settings.EncryptionEnabled
	}

	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	f.emergency("POST", "/api/emergency/adopt", nil)
	if !encryption() {
		t.Fatal("adopting from a kit with encryption on left the setting off")
	}
	// A second key over the first must not take the first one's setting for
	// the one to put back.
	f.emergency("POST", "/api/emergency/read", map[string]any{"key": strings.Repeat("e", 64), "places": []string{"backups/vms"}})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("second adoption: %v", res)
	}

	if res, _ := f.emergency("DELETE", "/api/emergency", nil); res["ok"] != true {
		t.Fatalf("undo: %v", res)
	}
	if encryption() {
		t.Fatal("the undo left encryption on, which the adoption had switched on")
	}
}
