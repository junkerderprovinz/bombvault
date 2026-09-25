package api

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// envEngine records the environment each location was last probed or listed
// with, and answers the rest as the fixture's engine does.
type envEngine struct {
	*placementEngine
	envs map[string][]string // by slash-spelled location
}

// newEnvEngine puts an envEngine in front of the fixture's engine.
func newEnvEngine(f *placementFixture) *envEngine {
	e := &envEngine{placementEngine: f.eng, envs: map[string][]string{}}
	f.svc.engine = e
	return e
}

func (e *envEngine) note(repo string, m restic.Mode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.envs[filepath.ToSlash(repo)] = slices.Clone(m.Env)
}

func (e *envEngine) env(repo string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.envs[repo]
}

func (e *envEngine) RepoOpensErr(ctx context.Context, repo string, m restic.Mode) error {
	e.note(repo, m)
	return e.placementEngine.RepoOpensErr(ctx, repo, m)
}

func (e *envEngine) Snapshots(ctx context.Context, repo string, m restic.Mode) ([]restic.Snapshot, error) {
	e.note(repo, m)
	return e.placementEngine.Snapshots(ctx, repo, m)
}

// credsScene is a containers domain whose remote path and first target sit on
// the credential set b2, while the shared credentials hold another key.
func credsScene(t *testing.T) (*placementFixture, *envEngine, string) {
	t.Helper()
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "B2-KEY", S3Secret: "b"}}}); err != nil {
		t.Fatal(err)
	}
	path := "s3:https://s3.us-west-004.backblazeb2.com/bv/containers"
	settings := settingsOf(t, f.svc)
	settings.ContainersPath = path
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: path, CredsRef: "b2", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	target := f.target("containers", "B2 copies", "s3:https://s3.us-west-004.backblazeb2.com/bv/copies")
	target.CredsRef = "b2"
	if _, err := f.st.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	return f, newEnvEngine(f), path
}

func TestTheOffsiteConnectionTestUsesTheTargetsCredentials(t *testing.T) {
	f, eng, _ := credsScene(t)
	if _, _, err := f.svc.TestOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if env := eng.env("s3:https://s3.us-west-004.backblazeb2.com/bv/copies"); !slices.Contains(env, "AWS_ACCESS_KEY_ID=B2-KEY") {
		t.Fatalf("the test ran with %v, want the target's set", env)
	}
}

func TestEncryptionDetectionOpensARemoteDomainPathWithItsCredentials(t *testing.T) {
	f, _, path := credsScene(t)
	for _, target := range f.svc.encryptionProbeTargets(settingsOf(t, f.svc)) {
		if target.domain != "containers" || target.source != "local" {
			continue
		}
		if target.repo != path || !slices.Contains(target.mode.Env, "AWS_ACCESS_KEY_ID=B2-KEY") {
			t.Fatalf("the containers path is probed as %q with %v", target.repo, target.mode.Env)
		}
		return
	}
	t.Fatal("the containers path is not probed at all")
}

func TestExclusionSuggestionsReadARemoteDomainPathWithItsCredentials(t *testing.T) {
	f, _, path := credsScene(t)
	f.hold(path, snap("s1", 100, "container:web"))
	_, repo, mode, ok := f.svc.newestSnapshotFor(context.Background(), "web")
	if !ok || repo != path || !slices.Contains(mode.Env, "AWS_ACCESS_KEY_ID=B2-KEY") {
		t.Fatalf("newestSnapshotFor = %q, %v, %v", repo, mode.Env, ok)
	}
}
