package remotes

import (
	"encoding/base64"
	"testing"
)

func TestAConfigBlobFromRcloneAuthorizeFillsTheFormItCameFrom(t *testing.T) {
	blob := base64.RawStdEncoding.EncodeToString([]byte(`{"token":"{\"access_token\":\"a\"}","region":"eu","endpoint":"https://acme.sf-api.com"}`))
	settings := map[string]string{"token": blob, "region": "com"}

	UnpackAuthorize(settings)

	want := map[string]string{"token": `{"access_token":"a"}`, "region": "com", "endpoint": "https://acme.sf-api.com"}
	for k, v := range want {
		if settings[k] != v {
			t.Errorf("%s = %q, want %q", k, settings[k], v)
		}
	}
}

func TestATokenThatIsNoConfigBlobStaysAsTyped(t *testing.T) {
	// A Jottacloud login token is base64 JSON too, without a token in it.
	login := base64.StdEncoding.EncodeToString([]byte(`{"username":"me","auth_token":"x"}`))
	for _, token := range []string{`{"access_token":"a"}`, login, "plain api token"} {
		settings := map[string]string{"token": token}
		UnpackAuthorize(settings)
		if settings["token"] != token || len(settings) != 1 {
			t.Errorf("token %q became %v", token, settings)
		}
	}
}

func TestTheFormOffersASharedDriveAndTheShareFileAddress(t *testing.T) {
	backends := fixture(t)
	for backend, name := range map[string]string{"drive": "team_drive", "sharefile": "endpoint"} {
		for _, o := range backends[backend].Options {
			if o.Name == name && o.Advanced {
				t.Errorf("%s %s is left out of the form", backend, name)
			}
		}
	}
}

func TestOnlyTheBackendsWhoseSetupAFormCannotSkipWaitForIt(t *testing.T) {
	cases := []struct {
		draft Draft
		due   bool
	}{
		{Draft{Backend: "jottacloud", Settings: map[string]string{"token": "LOGIN"}}, true},
		{Draft{Backend: "jottacloud", Settings: map[string]string{"token": `{"access_token":"a"}`}}, false},
		{Draft{Backend: "onedrive", Settings: map[string]string{"token": `{}`}}, true},
		{Draft{Backend: "onedrive", Settings: map[string]string{"token": `{}`, "drive_id": "d", "drive_type": "personal"}}, false},
		{Draft{Backend: "zoho", Settings: map[string]string{"token": `{}`, "root_folder_id": "w"}}, false},
		{Draft{Backend: "box", Settings: map[string]string{"token": `{}`}}, false},
	}
	for _, c := range cases {
		if got := SetupDue(c.draft); got != c.due {
			t.Errorf("%s %v: due = %v, want %v", c.draft.Backend, c.draft.Settings, got, c.due)
		}
	}
}
