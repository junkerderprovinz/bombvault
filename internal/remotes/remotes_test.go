package remotes

import (
	"os"
	"slices"
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
