package api

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/ageseal"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
)

// writtenKit is the recovery kit of a fixture with encryption on, a named
// repository and a destination with one section.
func writtenKit(t *testing.T, f *placementFixture) string {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.EncryptionEnabled = true
	settings.VMsOffsite = "rest:https://box:8000/vms"
	settings.SetCompression("vms", "max")
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.namedRepo("Cold", "cold/store")
	f.tickDestination(f.destination("Vault", vaultRepo), "files")
	kit, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	return kit
}

func TestAWrittenRecoveryKitReadsBackItsKeyAndItsRepositories(t *testing.T) {
	f := newPlacementFixture(t)
	text := writtenKit(t, f)

	kit, err := readRecoveryKit(text)
	if err != nil {
		t.Fatalf("readRecoveryKit: %v", err)
	}
	if !kit.Encrypted || kit.appKey != f.svc.cfg.AppKey {
		t.Fatalf("read encrypted = %v and a key that is the instance's own: %v", kit.Encrypted, kit.appKey == f.svc.cfg.AppKey)
	}
	want := []kitLocation{
		{Domain: "containers", Kind: kitLocal, Folder: "backups/containers"},
		{Domain: "vms", Kind: kitLocal, Folder: "backups/vms"},
		{Domain: "vms", Kind: kitOffsite, Location: "rest:https://box:8000/vms"},
		{Domain: "files", Kind: kitLocal, Folder: "backups/files"},
		{Kind: kitNamed, Name: "Cold", Folder: "cold/store"},
		{Domain: "files", Kind: kitDestination, Name: "Vault", Location: vaultRepo + "/files"},
	}
	for _, w := range want {
		found := slices.ContainsFunc(kit.Locations, func(got kitLocation) bool {
			return got.Domain == w.Domain && got.Kind == w.Kind && got.Name == w.Name && got.Folder == w.Folder &&
				(w.Location == "" || got.Location == w.Location)
		})
		if !found {
			t.Errorf("the kit read back lacks %+v among %+v", w, kit.Locations)
		}
	}
	for _, loc := range kit.Locations {
		if strings.Contains(loc.Location, "compression") {
			t.Errorf("a location kept its compression note: %+v", loc)
		}
	}

	// Carried over a Windows clipboard, the kit reads the same.
	if again, err := readRecoveryKit(strings.ReplaceAll(text, "\n", "\r\n")); err != nil || again.appKey != kit.appKey || len(again.Locations) != len(kit.Locations) {
		t.Fatalf("with CRLF line ends: %+v, %v", again, err)
	}
}

func TestAKitThatCannotBeUsedSaysWhy(t *testing.T) {
	f := newPlacementFixture(t)
	text := writtenKit(t, f)
	key := f.svc.cfg.AppKey

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := ageseal.SealArmored([]byte(text), []age.Recipient{identity.Recipient()})
	if err != nil {
		t.Fatal(err)
	}
	mistyped := strings.Replace(text, key, strings.Repeat("b", 64), 1)

	cases := []struct {
		name, text string
		want       error
	}{
		{"sealed with age", string(sealed), errKitSealed},
		{"another file", "# Shopping list\n\n- milk\n", errNotAKit},
		{"a key that does not give the password", mistyped, errKitKeyBroken},
	}
	for _, c := range cases {
		if _, err := readRecoveryKit(c.text); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.EncryptionEnabled = false
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	plain, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	kit, err := readRecoveryKit(plain)
	if err != nil || kit.Encrypted || !kit.HasKey || kit.appKey != key || len(kit.Locations) == 0 {
		t.Fatalf("a kit without encryption reads as %+v, %v; want its locations and its key", kit, err)
	}

	// The kit an install without encryption wrote before kits named the key.
	older := strings.Replace(plain, "APP_KEY (the key the definitions are sealed with):\n\n    "+key+"\n\n", "", 1)
	kit, err = readRecoveryKit(older)
	if err != nil || kit.HasKey || len(kit.Locations) == 0 {
		t.Fatalf("an older kit without encryption reads as %+v, %v; want its locations and no key", kit, err)
	}
	if _, err := foreignKeyOf("", older); !errors.Is(err, errKitHasNoKey) {
		t.Fatalf("opening with a kit that names no key: %v, want %v", err, errKitHasNoKey)
	}
}

// The docs describe the kit, so what an install that adopted nothing prints
// is pinned here line by line.
func TestTheKitOfAnInstallThatAdoptedNothingKeepsItsWording(t *testing.T) {
	f := newPlacementFixture(t)
	key := f.svc.cfg.AppKey
	f.encrypted()
	on, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	dash := string(rune(0x2014))
	wantOn := "## Encryption\n\n" +
		"Status: ENABLED\n\n" +
		"APP_KEY (the master key " + dash + " recreate the BombVault container with this exact value):\n\n" +
		"    " + key + "\n\n" +
		"restic repository password (derived from APP_KEY; use this with plain restic):\n\n" +
		"    " + restickey.Derive(key) + "\n\n" +
		"## Repository locations\n"
	if !strings.Contains(on, wantOn) {
		t.Fatalf("the Encryption section with encryption on changed:\n%s", on)
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.EncryptionEnabled = false
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	off, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	wantOff := "## Encryption\n\n" +
		"Status: DISABLED\n\n" +
		"The repositories are created WITHOUT a password (restic --insecure-no-password).\n" +
		"There is no key to lose; the value of this kit is the repository locations and\n" +
		"the restore instructions below.\n\n" +
		"BombVault still seals the definitions it recreates containers and VMs from.\n" +
		"Restoring those on another server needs this key:\n\n" +
		"APP_KEY (the key the definitions are sealed with):\n\n" +
		"    " + key + "\n\n" +
		"## Repository locations\n"
	if !strings.Contains(off, wantOff) {
		t.Fatalf("the Encryption section with encryption off is not the old one plus the key:\n%s", off)
	}
	for _, kit := range []string{on, off} {
		if strings.Contains(kit, "Backup key") || strings.Contains(kit, "APP_KEY of this server") {
			t.Fatal("a kit of an install that adopted nothing speaks of a backup key")
		}
	}
}

func TestTheKitOfAnInstallWithAnAdoptedKeyNamesBothKeys(t *testing.T) {
	f := newPlacementFixture(t)
	own, old := f.svc.cfg.AppKey, strings.Repeat("d", 64)
	settings := f.encrypted()
	f.adopt(old, own)

	for _, encrypted := range []bool{true, false} {
		settings.EncryptionEnabled = encrypted
		if err := f.st.UpdateSettings(settings); err != nil {
			t.Fatal(err)
		}
		text, err := f.svc.RecoveryKit()
		if err != nil {
			t.Fatal(err)
		}
		kit, err := readRecoveryKit(text)
		if err != nil {
			t.Fatalf("encryption %v: %v", encrypted, err)
		}
		if kit.appKey != old || kit.sealingKey != own || kit.Encrypted != encrypted {
			t.Fatalf("encryption %v: the kit reads back the backup key %v and the sealing key %v", encrypted, kit.appKey == old, kit.sealingKey == own)
		}
		if encrypted && !strings.Contains(text, "export RESTIC_PASSWORD='"+restickey.Derive(old)+"'") {
			t.Fatal("the restore steps do not carry the password of the adopted key")
		}
	}

	f.adopt(old, strings.Repeat("e", 64))
	if _, err := f.svc.RecoveryKit(); !errors.Is(err, errBackupKeyLocked) {
		t.Fatalf("a kit under a locked backup key: %v, want %v", err, errBackupKeyLocked)
	}
}

func TestAForeignRepositoryOpensWithAnUploadedKit(t *testing.T) {
	old := newPlacementFixture(t)
	text := writtenKit(t, old)

	// A fresh instance under a key of its own, with the old share mounted.
	fresh := newPlacementFixture(t)
	fresh.svc.cfg.AppKey = strings.Repeat("c", 64)
	fresh.hold(fresh.domainPath("vms"), snap("a1", 100, "vm:win"))
	before, err := fresh.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	read := fresh.do("POST", "/api/recovery-kit/read", map[string]any{"kit": text})
	if read["ok"] != true {
		t.Fatalf("read: %v", read)
	}
	answer, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(answer), old.svc.cfg.AppKey) {
		t.Fatal("the answer to reading a kit carries its key")
	}

	res := fresh.do("POST", "/api/foreign/open", map[string]any{"location": "backups/vms", "kit": text})
	if res["ok"] != true {
		t.Fatalf("open with the kit: %v", res)
	}
	id, _ := res["session"].(string)
	fresh.svc.foreignMu.Lock()
	sess, open := fresh.svc.foreignSessions[id]
	fresh.svc.foreignMu.Unlock()
	if !open || sess.key != old.svc.cfg.AppKey {
		t.Fatalf("the session holds the key of the kit: %v (open %v)", sess.key == old.svc.cfg.AppKey, open)
	}
	if after, err := fresh.st.GetSettings(); err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("opening with a kit changed the settings: %v", err)
	}

	bad := fresh.do("POST", "/api/foreign/open", map[string]any{"location": "backups/vms", "kit": "not a kit"})
	if bad["ok"] != false || bad["code"] != "not-a-kit" {
		t.Fatalf("open with another file: %v", bad)
	}
}
