package api

import (
	"encoding/json"
	"testing"
)

// fileWith returns a valid settings file with mutate applied to it.
func fileWith(t *testing.T, mutate func(exp *settingsExport)) []byte {
	t.Helper()
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, exp := doExport(t, src, "")
	if err := json.Unmarshal(body, &exp); err != nil {
		t.Fatal(err)
	}
	mutate(&exp)
	out, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEveryImportProblemCarriesTheCodeThePageWords(t *testing.T) {
	dst, _ := newPortableHandler(t, appKeyB)
	for name, c := range map[string]struct {
		mutate func(exp *settingsExport)
		want   map[string]any
	}{
		"another format": {
			func(exp *settingsExport) { exp.SchemaVersion = 99 },
			map[string]any{"code": "import-version", "row": 99.0},
		},
		"repository without a name": {
			func(exp *settingsExport) { exp.NamedRepos = []offsiteTargetView{{Repo: "b2:bucket:a"}} },
			map[string]any{"code": "import-row-name", "part": "repository", "row": 1.0},
		},
		"repository at a full host path": {
			func(exp *settingsExport) {
				exp.NamedRepos = []offsiteTargetView{{Name: "Cold", Repo: "b2:bucket:a"}, {Name: "Warm", Repo: "/mnt/user/warm"}}
			},
			map[string]any{"code": "import-row-location", "part": "repository", "row": 2.0, "cause": "path-absolute"},
		},
		"storage location without a place": {
			func(exp *settingsExport) { exp.Destinations = []offsiteTargetView{{Name: "Box"}} },
			map[string]any{"code": "import-row-location", "part": "destination", "row": 1.0, "cause": "location-needed"},
		},
		"off-site storage location with an unknown compression": {
			func(exp *settingsExport) {
				exp.OffsiteTargets = []offsiteTargetView{{Domain: "containers", Repo: "b2:bucket:c", Compression: "huge"}}
			},
			map[string]any{"code": "import-row-compression", "part": "target", "row": 1.0},
		},
		"unknown compression": {
			func(exp *settingsExport) { exp.Settings.Compression = map[string]string{"containers": "huge"} },
			map[string]any{"code": "import-setting", "name": "compression"},
		},
		"repository on a section's folder": {
			func(exp *settingsExport) {
				exp.NamedRepos = []offsiteTargetView{{Name: "Here", Repo: exp.Settings.ContainersPath}}
			},
			map[string]any{"code": "import-place-taken", "part": "repository", "row": 1.0, "place": "path:containers"},
		},
	} {
		env := doImport(t, dst, fileWith(t, c.mutate), "")
		if env["ok"] != false {
			t.Errorf("%s: answer = %v, want a refusal", name, env)
			continue
		}
		for key, want := range c.want {
			if env[key] != want {
				t.Errorf("%s: %s = %v, want %v (answer %v)", name, key, env[key], want, env)
			}
		}
	}
	if env := doImport(t, dst, []byte("not a settings file"), ""); env["code"] != "import-unreadable" {
		t.Errorf("a file that is not one = %v, want import-unreadable", env)
	}
}
