package api

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// envOpensEngine opens a location only when the mode carries the variable it
// holds for that location.
type envOpensEngine struct {
	*placementEngine
	want map[string]string
}

func (e *envOpensEngine) RepoOpens(_ context.Context, repo string, mode restic.Mode) bool {
	return slices.Contains(mode.Env, e.want[repo])
}

func TestAWebDAVDirectRepositoryKeepsThePasswordItOpenedWith(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{davSet()}); err != nil {
		t.Fatal(err)
	}
	target := f.target("containers", "Nextcloud", "rclone:"+places.RemoteName(davPlace)+":bombvault/containers")
	target.CredsRef = "dav"
	target, err := f.st.UpsertOffsiteTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	d := f.direct(target)
	f.container("web", d.ID)
	// The last variable davEnv renders is the old password in rclone's form.
	oldPass := davEnv(davPlace)[4]
	f.svc.engine = &envOpensEngine{placementEngine: f.eng, want: map[string]string{d.Repo: oldPass}}

	drafts := f.credSetDrafts()
	drafts[0]["webdavPass"] = "new-pass"
	res := f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": drafts})
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
	kept := f.keptFor(d.ID)
	if len(kept) != 1 || kept[0].Kind != "webdav" || kept[0].WebDAVPass != "app-pass" || kept[0].WebDAVURL != davSet().WebDAVURL {
		t.Fatalf("kept sets = %+v, want the whole WebDAV set with the old password", kept)
	}
	if env := f.runsWith(d); !slices.Contains(env, oldPass) {
		t.Fatalf("a backup into the direct repository runs with %v, want the old password", env)
	}
}
