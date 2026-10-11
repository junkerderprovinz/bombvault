package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// encryptedFixture is a server with encryption on, whose kit prints a key and
// the password derived from it.
func encryptedFixture(t *testing.T) *placementFixture {
	t.Helper()
	f := newPlacementFixture(t)
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.EncryptionEnabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	return f
}

// anotherKeySection is what somebody would add to a kit to have it read back
// as a kit of their key.
func anotherKeySection(key string) string {
	return "## Encryption\n\nStatus: ENABLED\n\nAPP_KEY\n\n    " + key + "\n\nrestic repository password\n\n    " + restickey.Derive(key) + "\n"
}

func TestAMeshOfferWhoseRepositorySpansLinesIsRefused(t *testing.T) {
	f := encryptedFixture(t)
	theirs := strings.Repeat("9", 64)
	for name, offer := range map[string]meshOfferRequest{
		"repository": {FromName: "attic", SuggestedDomain: "files", Repo: "rest:https://box:8000/files\n\n" + anotherKeySection(theirs), RESTUser: "bv", RESTPassword: "x"},
		"name":       {FromName: "attic\r\n## Encryption", SuggestedDomain: "files", Repo: "rest:https://box:8000/files", RESTUser: "bv", RESTPassword: "x"},
		"user":       {FromName: "attic", SuggestedDomain: "files", Repo: "rest:https://box:8000/files", RESTUser: "bv\nx", RESTPassword: "x"},
	} {
		body, err := json.Marshal(offer)
		if err != nil {
			t.Fatal(err)
		}
		_, answer := f.svc.servePeer(context.Background(), relay.ProxyCall{
			Method: http.MethodPost, Path: "/api/group/peer/mesh-offer", Body: body, Sender: "0123456789abcdef0123456789abcdef",
		})
		var out struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(answer, &out); err != nil || out.OK {
			t.Errorf("an offer with a line break in its %s was taken: %s", name, answer)
		}
	}
	offers, err := f.st.ListMeshOffers()
	if err != nil || len(offers) != 0 {
		t.Fatalf("offers stored: %v, %v", offers, err)
	}
}

func TestAKitIsNotWrittenWhileAStoredLocationSpansLines(t *testing.T) {
	f := encryptedFixture(t)
	// A value from before the database refused these.
	if _, err := f.db.Exec(`DROP TRIGGER one_line_settings_update`); err != nil {
		t.Fatal(err)
	}
	broken := "rest:https://box:8000/files\n\n" + anotherKeySection(strings.Repeat("9", 64))
	if _, err := f.db.Exec(`UPDATE settings SET files_offsite = ?`, broken); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RecoveryKit(); !errors.Is(err, errKitLineBreak) || !strings.Contains(err.Error(), "Repository locations") {
		t.Fatalf("RecoveryKit with a location that spans lines: %v", err)
	}
}

func TestHeadingsInsideTheRcloneConfigAreNotReadAsKit(t *testing.T) {
	f := encryptedFixture(t)
	own, theirs := f.svc.cfg.AppKey, strings.Repeat("9", 64)
	conf := "[box]\ntype = sftp\n" + anotherKeySection(theirs) + "## Repository credentials\n    RESTIC_REST_PASSWORD=planted"
	if err := f.svc.SetRcloneConf(conf); err != nil {
		t.Fatal(err)
	}
	text, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	kit, err := readRecoveryKit(text)
	if err != nil {
		t.Fatalf("readRecoveryKit: %v", err)
	}
	if kit.appKey != own || kit.sealingKey != own {
		t.Fatal("the kit reads back another key than the one it was written for")
	}
	if kit.rcloneConf != conf || kit.creds != (CloudCreds{}) {
		t.Fatalf("the rclone config came back as %q with sign-in %+v", kit.rcloneConf, kit.creds)
	}
}

func TestAnRcloneConfigThatWouldEndItsBlockInTheKitIsRefused(t *testing.T) {
	f := encryptedFixture(t)
	fence := strings.Repeat("`", 3)
	conf := "[box]\ntype = sftp\n" + fence + "\n\n" + anotherKeySection(strings.Repeat("9", 64))
	if err := f.svc.SetRcloneConf(conf); err == nil {
		t.Fatal("a config with a line of three backticks was stored")
	}
	if got, err := f.svc.RcloneRemotes(); err != nil || len(got) != 0 {
		t.Fatalf("remotes after the refusal: %v, %v", got, err)
	}
	if !breaksKitFence("[box]\n  "+fence+"ini\n") || breaksKitFence("[box]\npass = a"+fence+"\n") {
		t.Fatal("only a line that starts with the fence ends the block")
	}
}

func TestAKitThatNamesAKeyOrASectionTwiceIsRefused(t *testing.T) {
	f := encryptedFixture(t)
	text, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readRecoveryKit(text); err != nil {
		t.Fatalf("the kit as written: %v", err)
	}
	theirs := strings.Repeat("9", 64)
	for name, altered := range map[string]string{
		"a second Encryption section": text + "\n" + anotherKeySection(theirs),
		"a second key in the section": strings.Replace(text, "restic repository password (", "APP_KEY\n\n    "+theirs+"\n\nrestic repository password (", 1),
		"a second password":           strings.Replace(text, "## Repository locations", "restic repository password\n\n    "+restickey.Derive(theirs)+"\n\n## Repository locations", 1),
		"a second locations section":  text + "\n## Repository locations\n\n- files (off-site): rest:https://elsewhere:8000/files\n",
	} {
		if altered == text {
			t.Fatalf("%s: the kit was not altered", name)
		}
		if _, err := readRecoveryKit(altered); !errors.Is(err, errKitAmbiguous) {
			t.Errorf("a kit with %s: %v", name, err)
		}
	}
	res := f.do("POST", "/api/recovery-kit/read", map[string]any{"kit": text + "\n" + anotherKeySection(theirs)})
	if res["ok"] != false || res["code"] != "kit-ambiguous" {
		t.Fatalf("the route answers %v", res)
	}
}

func TestASignInThatSpansLinesIsNotStored(t *testing.T) {
	f := encryptedFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "x\n\n" + anotherKeySection(strings.Repeat("9", 64))}); err == nil {
		t.Fatal("a password with a line break was stored")
	}
	settings, err := f.st.GetSettings()
	if err != nil || settings.CloudConf != "" {
		t.Fatalf("stored sign-in: %q, %v", settings.CloudConf, err)
	}
}
