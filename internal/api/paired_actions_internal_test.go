package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pairedInstance is an instance whose requests go out with the session of a
// paired phone.
func pairedInstance(t *testing.T) *instance {
	t.Helper()
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	hash, epoch, _ := in.h.phoneSigning()
	in.session = listedPhoneSession(t, in.st, in.appKey, hash, epoch)
	return in
}

func TestAPairedPhoneReadsThatTheKeyIsLockedAndNothingElseOfTheEmergency(t *testing.T) {
	in := pairedInstance(t)
	if err := in.st.SetAdoptedKey([]byte("sealed under another APP_KEY"), "kit", true); err != nil {
		t.Fatal(err)
	}
	in.svc.forgetBackupKey()

	code, out := in.do(t, http.MethodGet, "/api/status", nil)
	if code != http.StatusOK || out["backupKey"] != backupKeyLocked {
		t.Fatalf("the overview with a paired session: %d, backupKey %v, want %q", code, out["backupKey"], backupKeyLocked)
	}
	if code, out := in.do(t, http.MethodGet, "/api/emergency", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("the emergency with a paired session: %d %v", code, out)
	}
}

func TestAPairedPhoneStartsARestoreProbeAndAStartTest(t *testing.T) {
	in := pairedInstance(t)
	for _, c := range []struct{ path, answer string }{
		{"/api/checks/probe/" + store.FlashTargetID, "no backup with data to check"},
		{"/api/checks/starttest/gone", "no container with id"},
	} {
		code, out := in.do(t, http.MethodPost, c.path, nil)
		if code != http.StatusOK || !strings.Contains(fmt.Sprint(out["error"]), c.answer) {
			t.Errorf("%s with a paired session: %d %v, want the check's own answer", c.path, code, out)
		}
	}
	// The probe of a ZFS item's snapshot access reads the host, which the
	// Verify tab does not.
	if code, out := in.do(t, http.MethodPost, "/api/zfs/datasets/abc/probe", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("the ZFS access probe with a paired session: %d %v", code, out)
	}
}

func TestAPairedPhoneChecksForAnUpdateAndReadsItsNotes(t *testing.T) {
	fakeGitHub(t, http.StatusOK)
	in := pairedInstance(t)
	code, out := in.do(t, http.MethodPost, "/api/update-check", nil)
	update, _ := out["update"].(map[string]any)
	if code != http.StatusOK || update["updateAvailable"] != true || update["latest"] != "v99.0.0" {
		t.Fatalf("the update check with a paired session: %d %v", code, out)
	}
	if code, out := in.do(t, http.MethodGet, "/api/release-notes?version=v99.0.0", nil); code != http.StatusOK || out["body"] != "## Added\n\n- v99.0.0" {
		t.Errorf("the notes of the newer release with a paired session: %d %v", code, out)
	}
}
