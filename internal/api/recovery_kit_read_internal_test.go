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
	if err != nil || kit.Encrypted || kit.appKey != "" || len(kit.Locations) == 0 {
		t.Fatalf("a kit without encryption reads as %+v, %v; want its locations and no key", kit, err)
	}
	if _, err := foreignKeyOf("", plain); !errors.Is(err, errKitHasNoKey) {
		t.Fatalf("opening with a kit that names no key: %v, want %v", err, errKitHasNoKey)
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
