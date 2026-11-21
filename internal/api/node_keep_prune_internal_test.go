package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func pruneAnswer(t *testing.T, body *bytes.Buffer) nodePruneAnswer {
	t.Helper()
	var ans nodePruneAnswer
	if err := json.Unmarshal(body.Bytes(), &ans); err != nil {
		t.Fatalf("answer %s: %v", body, err)
	}
	return ans
}

func TestANodePrunesTheRepositoriesItKeepsOnlyWhileAdopted(t *testing.T) {
	n := newNodeUnderTest(t)
	n.node.svc.engine = &agingRepos{}
	n.node.svc.cfg.HostMountRoot = t.TempDir()
	call := nodePruneCall{Domain: "files", Run: "r1", Keep: restic.RetentionPolicy{KeepLast: 3, Direct: true}}
	if w := n.call(t, keydeposit.ProofNodePrune, nodePrunePath, call); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeNotAdopted)) {
		t.Errorf("a prune before the adoption = %d %s", w.Code, w.Body)
	}
	n.adopt(t)
	if w := n.call(t, keydeposit.ProofNodeRun, nodePrunePath, call); w.Code != http.StatusForbidden {
		t.Errorf("a run passed off as a prune = %d %s", w.Code, w.Body)
	}
	for name, bad := range map[string]nodePruneCall{
		"another section": {Domain: "vms", Run: "r1", Keep: call.Keep},
		"no run":          {Domain: "files", Keep: call.Keep},
		"keep everything": {Domain: "files", Run: "r1"},
	} {
		if w := n.call(t, keydeposit.ProofNodePrune, nodePrunePath, bad); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %s", name, w.Code, w.Body)
		}
	}
	if w := n.call(t, keydeposit.ProofNodePrune, nodePrunePath, call); w.Code != http.StatusOK {
		t.Errorf("a prune once adopted = %d %s", w.Code, w.Body)
	}
	eventually(t, "the prune to end", func() bool {
		return pruneAnswer(t, n.call(t, keydeposit.ProofNodePrune, nodePrunePath, call).Body).State != nodePruneRunning
	})
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatalf("release = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodePrune, nodePrunePath, call); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeReleased)) {
		t.Errorf("a prune after the release = %d %s", w.Code, w.Body)
	}
}

func TestANodeAgesTheRepositoryItKeepsInItsFolderAndSaysWhenItIsDone(t *testing.T) {
	n := newNodeUnderTest(t)
	n.adopt(t)
	repos := &agingRepos{}
	n.node.svc.engine = repos
	n.node.svc.cfg.HostMountRoot = t.TempDir()
	kept := []nodePlacement{{Domain: "containers", Repo: "rest:https://203.0.113.7:8443/" + nodeRestUser + "/containers", Kept: true}}
	if w := n.call(t, keydeposit.ProofNodePlacement, nodePlacementPath, kept); w.Code != http.StatusOK {
		t.Fatalf("placement = %d %s", w.Code, w.Body)
	}
	call := nodePruneCall{Domain: "containers", Run: "r1", Keep: restic.RetentionPolicy{KeepLast: 3, Direct: true}}
	if ans := pruneAnswer(t, n.call(t, keydeposit.ProofNodePrune, nodePrunePath, call).Body); !ans.OK || ans.State != nodePruneRunning {
		t.Fatalf("first answer = %+v, want the prune running", ans)
	}
	eventually(t, "the prune to be done", func() bool {
		return pruneAnswer(t, n.call(t, keydeposit.ProofNodePrune, nodePrunePath, call).Body).State == nodePruneDone
	})
	elsewhere := nodePruneCall{Domain: "files", Run: "r2", Keep: call.Keep}
	eventually(t, "the prune of a section home placed elsewhere to fail", func() bool {
		return pruneAnswer(t, n.call(t, keydeposit.ProofNodePrune, nodePrunePath, elsewhere).Body).State == nodePruneFailed
	})
	repo := path.Join(n.node.svc.cfg.HostMountRoot, nodeRestFolder, nodeRestUser, "containers")
	host := restickey.Host(n.node.svc.cfg.AppKey)
	repos.mu.Lock()
	defer repos.mu.Unlock()
	forgets := slices.Sorted(slices.Values(repos.forgets))
	if want := []string{repo + " " + host + " container:db 3", repo + " " + host + " container:web 3"}; !slices.Equal(forgets, want) {
		t.Errorf("forgets = %q, want %q", forgets, want)
	}
	if !slices.Equal(repos.prunes, []string{repo + " " + host}) {
		t.Errorf("prunes = %q, want the folder once, the same prune asked about again included and the other section left alone", repos.prunes)
	}
}

func TestHomeHasANodeThatKeepsItsBackupsAgeThemItselfAndWaitsForIt(t *testing.T) {
	waits := nodePruneWait
	nodePruneWait = 20 * time.Millisecond
	t.Cleanup(func() { nodePruneWait = waits })
	home := homeWithRelay(t)
	repos := newRepoStore()
	home.svc.engine = repos
	if _, err := home.st.MutateSettings(func(s *store.Settings) error { s.RetentionKeepLast = 4; return nil }); err != nil {
		t.Fatal(err)
	}
	node, id, _ := adoptedNode(t, home, repos)
	node.svc.docker = &receiverDocker{imageID: "sha256:rest"}
	node.svc.cfg.HostMountRoot = t.TempDir()
	node.svc.cfg.HostSourceRoot = "/srv/data"
	node.engine.want = node.svc.backupPassword()
	if code, out := home.do(t, http.MethodPost, "/api/nodes/"+id+"/placement", map[string]any{"kind": "node", "address": "https://203.0.113.7:8443"}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("keep on the node = %d %v", code, out)
	}
	aging := &agingRepos{}
	node.svc.engine = aging
	n, err := home.st.GetNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := home.svc.pruneNodeSection(context.Background(), n, "files"); err != nil {
		t.Fatalf("prune on the node: %v", err)
	}
	aging.mu.Lock()
	defer aging.mu.Unlock()
	if len(aging.prunes) != 1 || !strings.HasPrefix(aging.prunes[0], path.Join(node.svc.cfg.HostMountRoot, nodeRestFolder, nodeRestUser, "files")+" ") {
		t.Errorf("the node pruned %q, want its own files folder once", aging.prunes)
	}
	if len(aging.forgets) == 0 || !strings.HasSuffix(aging.forgets[0], " 4") {
		t.Errorf("forgets = %q, want home's keep rule of four", aging.forgets)
	}
}
