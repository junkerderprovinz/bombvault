package api

import (
	"net/url"
	"testing"
)

func TestEveryLocationRefusalCarriesTheCodeTheInterfaceTranslates(t *testing.T) {
	f := newPlacementFixture(t)
	f.namedRepo("Cold", "cold/vault")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	save := func(change func(v *settingsView)) map[string]any {
		v := toView(settings)
		change(&v)
		return f.do("PUT", "/api/settings", v)
	}
	preview := func(repo, as string) map[string]any {
		res := f.do("GET", "/api/placement/new-target-preview?domain=files&repo="+url.QueryEscape(repo)+"&as="+as, nil)
		if res["saveRefused"] != true {
			t.Errorf("preview of %s as %s = %v, want the save refused before the question", repo, as, res)
		}
		return res
	}

	for name, c := range map[string]struct {
		res  map[string]any
		code string
	}{
		"own folder in a repository":   {save(func(v *settingsView) { v.FilesPath = "cold/vault/files" }), "named-repo-overlap"},
		"unprefixed remote":            {save(func(v *settingsView) { v.ContainersOffsite = "BackBlaze:bucket" }), "unprefixed-remote"},
		"absolute path":                {save(func(v *settingsView) { v.ContainersPath = "/mnt/user/backups" }), "path-absolute"},
		"path out of the mount":        {save(func(v *settingsView) { v.ContainersPath = "../backups" }), "path-outside"},
		"remote restore folder":        {save(func(v *settingsView) { v.RestoreFolder = "s3:restore" }), "restore-folder-remote"},
		"target in a repository":       {f.do("POST", "/api/offsite/targets", map[string]any{"domain": "files", "name": "In", "repo": "cold/vault/in", "enabled": true}), "named-repo-overlap"},
		"repository without a place":   {f.do("POST", "/api/repos", map[string]any{"name": "Empty", "repo": ""}), "location-needed"},
		"question on a repository":     {preview("cold/vault/in", "target"), "named-repo-overlap"},
		"question on an unprefixed one": {preview("BackBlaze:bucket", "field"), "unprefixed-remote"},
	} {
		if c.res["ok"] != false || c.res["code"] != c.code {
			t.Errorf("%s: answer = %v, want code %s", name, c.res, c.code)
		}
	}
}
