package api

import (
	"net/http"
	"strings"
	"testing"
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
