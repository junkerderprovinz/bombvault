package remotes

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func fixture(t *testing.T) map[string]Backend {
	t.Helper()
	raw, err := os.ReadFile("testdata/rclone-1.75.1-providers.json")
	if err != nil {
		t.Fatal(err)
	}
	backends, err := ParseBackends(raw)
	if err != nil {
		t.Fatal(err)
	}
	return backends
}

func TestEveryProviderNamesABackendRcloneHas(t *testing.T) {
	backends := fixture(t)
	for _, p := range providers {
		if p.Route == RouteRest || p.Route == RoutePath {
			if p.Backend != "" {
				t.Errorf("%s: route %s needs no rclone backend, has %q", p.ID, p.Route, p.Backend)
			}
			continue
		}
		if _, ok := backends[p.Backend]; !ok {
			t.Errorf("%s: rclone has no backend %q", p.ID, p.Backend)
		}
	}
}

func TestEveryEssentialSettingExists(t *testing.T) {
	backends := fixture(t)
	for backend, names := range essential {
		b, ok := backends[backend]
		if !ok {
			t.Errorf("essential lists %q, which no provider uses or rclone lacks", backend)
			continue
		}
		for name := range names {
			if !slices.ContainsFunc(b.Options, func(o Option) bool { return o.Name == name }) {
				t.Errorf("%s: rclone has no option %q", backend, name)
			}
		}
	}
}

func TestProviderIDsAreUniqueAndGrouped(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range providers {
		if seen[p.ID] {
			t.Errorf("provider id %q appears twice", p.ID)
		}
		seen[p.ID] = true
		if _, ok := groupRank[p.Group]; !ok {
			t.Errorf("%s: unknown group %q", p.ID, p.Group)
		}
	}
	list := Providers()
	for i := 1; i < len(list); i++ {
		if groupRank[list[i-1].Group] > groupRank[list[i].Group] {
			t.Fatalf("%s comes after %s across groups", list[i].ID, list[i-1].ID)
		}
	}
}

func TestSecretsAreMarkedAndTheirExamplesDropped(t *testing.T) {
	b := fixture(t)["webdav"]
	if !PasswordOption(b, "pass") || !SecretOption(b, "pass") {
		t.Fatal("webdav pass is not marked as a password")
	}
	s3 := fixture(t)["s3"]
	if PasswordOption(s3, "secret_access_key") || !SecretOption(s3, "secret_access_key") {
		t.Fatal("an S3 secret is sensitive but rclone reads it plain")
	}
	for _, o := range s3.Options {
		if o.Secret && len(o.Examples) > 0 {
			t.Errorf("secret option %s carries examples", o.Name)
		}
	}
}

func TestEndpointExamplesKeepTheirProvider(t *testing.T) {
	for _, o := range fixture(t)["s3"].Options {
		if o.Name != "endpoint" {
			continue
		}
		if !slices.ContainsFunc(o.Examples, func(e Example) bool { return e.Provider == "Wasabi" }) {
			t.Fatal("no endpoint example is tied to Wasabi")
		}
		return
	}
	t.Fatal("s3 has no endpoint option")
}

func TestEveryS3ServiceIsAskedForItsEndpoint(t *testing.T) {
	s3 := fixture(t)["s3"]
	i := slices.IndexFunc(s3.Options, func(o Option) bool { return o.Name == "endpoint" })
	if i < 0 {
		t.Fatal("s3 has no endpoint option")
	}
	endpoint := s3.Options[i]
	for _, p := range providers {
		if p.Route != RouteS3 {
			continue
		}
		if len(endpoint.Providers) > 0 && !slices.Contains(endpoint.Providers, p.Preset["provider"]) {
			t.Errorf("%s: the form has no endpoint field, and restic cannot reach the service without one", p.ID)
		}
	}
}

func TestICloudAsksForTheSessionOfATwoFactorSignIn(t *testing.T) {
	p, ok := FindProvider("icloud")
	if !ok {
		t.Fatal("no iCloud provider")
	}
	if p.Auth != AuthTrustToken {
		t.Fatalf("iCloud signs in by %q, want the trust token of a sign-in done in rclone config", p.Auth)
	}
	for _, o := range fixture(t)["iclouddrive"].Options {
		if (o.Name == "trust_token" || o.Name == "cookies") && !o.Essential {
			t.Errorf("%s is not asked for, and rclone does not sign in to iCloud without it", o.Name)
		}
	}
}

func TestLastErrorKeepsRclonesOwnSentence(t *testing.T) {
	for _, c := range []struct{ stderr, want string }{
		{"2026/10/05 00:10:01 NOTICE: something\n2026/10/05 00:10:02 ERROR : error listing: 401 Unauthorized\n", "error listing: 401 Unauthorized"},
		{"2026/10/05 00:05:34 NOTICE: Failed to lsjson with 2 errors: last error was: error in ListJSON: dial tcp: refused", "error in ListJSON: dial tcp: refused"},
		{"", os.ErrInvalid.Error()},
	} {
		if got := lastError(c.stderr, os.ErrInvalid); got != c.want {
			t.Errorf("lastError(%q) = %q, want %q", c.stderr, got, c.want)
		}
	}
}

func TestAFailureSaysWhatToChangeWithoutTheDraftsName(t *testing.T) {
	for _, c := range []struct{ msg, want string }{
		{
			`Failed to create file system for "BVDRAFT:": NewFs: couldn't connect SSH: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain`,
			"the server did not accept BombVault's SSH key. Add the public key the form shows to the authorized keys of this user on the server, then test again " +
				"(couldn't connect SSH: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain)",
		},
		{
			`Failed to create file system for "BVDRAFT:": the remote url looks incorrect. Note that nextcloud chunked uploads require you to use the /dav/files/USER endpoint instead of /webdav. Please check 'rclone config show remotename' to verify that the url field ends in /dav/files/USER`,
			"this is not the address Nextcloud keeps files under. Use https://cloud.example.com/remote.php/dav/files/USER/ with your Nextcloud user name in place of USER",
		},
		{`error in ListJSON: dial tcp 10.0.0.5:9000: connect: connection refused`, "dial tcp 10.0.0.5:9000: connect: connection refused"},
	} {
		if got := explain(c.msg); got != c.want {
			t.Errorf("explain(%q)\n = %q\nwant %q", c.msg, got, c.want)
		}
	}
	for _, msg := range []string{
		"NewFs: 530 Non-anonymous sessions must use encryption.",
		"error in ListJSON: operation error S3: ListBuckets, https response error StatusCode: 400, api error AuthorizationHeaderMalformed: Authorization header malformed, unexpected scope",
	} {
		if got := explain(msg); !strings.Contains(got, " (") || strings.HasPrefix(got, "NewFs") || strings.HasPrefix(got, "error in") {
			t.Errorf("explain(%q) = %q, want a hint in front of rclone's own words", msg, got)
		}
	}
}

func TestEveryDefaultIsASettingOfTheBackend(t *testing.T) {
	backends := fixture(t)
	for _, p := range providers {
		for name := range p.Defaults {
			if !slices.ContainsFunc(backends[p.Backend].Options, func(o Option) bool { return o.Name == name }) {
				t.Errorf("%s: rclone's %s has no option %q", p.ID, p.Backend, name)
			}
		}
	}
	if garage, _ := FindProvider("garage"); garage.Defaults["region"] != "garage" {
		t.Errorf("Garage starts with the region %q, want the one it signs for", garage.Defaults["region"])
	}
}

func TestJoinNeverDoublesTheSeparator(t *testing.T) {
	for _, c := range []struct{ remote, dir, want string }{
		{"b2", "", "b2:"},
		{"b2", "/bucket/bv/", "b2:bucket/bv"},
		{"BVDRAFT", "bucket", "BVDRAFT:bucket"},
	} {
		if got := Join(c.remote, c.dir); got != c.want {
			t.Errorf("Join(%q, %q) = %q, want %q", c.remote, c.dir, got, c.want)
		}
	}
}

func TestAnOptionKeepsTheKindOfValueRcloneTakes(t *testing.T) {
	raw := []byte(`[{"Name":"ftp","Options":[
		{"Name":"tls","Type":"bool","DefaultStr":"false"},
		{"Name":"host","Type":"string","Required":true}]}]`)
	backends, err := ParseBackends(raw)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, o := range backends["ftp"].Options {
		types[o.Name] = o.Type
	}
	if types["tls"] != "bool" || types["host"] != "string" {
		t.Fatalf("types = %v, want tls bool and host string", types)
	}
}

func TestEveryOptionAProviderLeavesUnaskedExists(t *testing.T) {
	backends := fixture(t)
	for _, p := range providers {
		for _, name := range p.Unasked {
			if !slices.ContainsFunc(backends[p.Backend].Options, func(o Option) bool { return o.Name == name }) {
				t.Errorf("%s: rclone's %s has no option %q", p.ID, p.Backend, name)
			}
		}
	}
}
