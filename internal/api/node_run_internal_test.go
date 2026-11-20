package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestANodeStartsASectionsBackupsOnlyWhileAdopted(t *testing.T) {
	n := newNodeUnderTest(t)
	run := nodeRunCall{Domain: "files"}
	if w := n.call(t, keydeposit.ProofNodeRun, nodeRunPath, run); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeNotAdopted)) {
		t.Errorf("a run before the adoption = %d %s", w.Code, w.Body)
	}
	n.adopt(t)
	stranger := newInstance(t, "stranger", strings.Repeat("c3", 32))
	if code := post(n.routes, nodeRunPath, signedBodyForNode(t, stranger, keydeposit.ProofNodeRun, n.id, "stranger", run)); code != http.StatusForbidden {
		t.Errorf("a run signed by another instance = %d", code)
	}
	if w := n.call(t, keydeposit.ProofNodeAPI, nodeRunPath, run); w.Code != http.StatusForbidden {
		t.Errorf("a request passed off as a run = %d %s", w.Code, w.Body)
	}
	w := n.call(t, keydeposit.ProofNodeRun, nodeRunPath, run)
	var ans nodeRunAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || w.Code != http.StatusOK || !ans.OK || ans.Started != 0 {
		t.Errorf("a run of a section with nothing in it = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRun, nodeRunPath, nodeRunCall{Domain: "vms"}); w.Code != http.StatusBadRequest {
		t.Errorf("a run of a section a node does not back up = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatalf("release = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRun, nodeRunPath, run); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeReleased)) {
		t.Errorf("a run after the release = %d %s", w.Code, w.Body)
	}
}

func TestANodeRunTakesTheEntriesAScheduledRoundWould(t *testing.T) {
	n := newNodeUnderTest(t)
	st := n.node.st
	for _, tg := range []store.Target{
		{ContainerName: "web", IncludeInSchedule: true},
		{ContainerName: "scratch", IncludeInSchedule: false},
	} {
		if _, err := st.UpsertTarget(tg); err != nil {
			t.Fatal(err)
		}
	}
	for _, fs := range []store.FileSet{
		{Name: "photos", Enabled: true, Path: "photos"},
		{Name: "old", Enabled: false, Path: "old"},
	} {
		if _, err := st.CreateFileSet(fs); err != nil {
			t.Fatal(err)
		}
	}
	names, err := n.node.svc.roundEntries("containers")
	if err != nil || !slices.Equal(names, []string{"web"}) {
		t.Errorf("containers = %v %v, want the one in the schedule", names, err)
	}
	ids, err := n.node.svc.roundEntries("files")
	if err != nil || len(ids) != 1 {
		t.Fatalf("folder sets = %v %v, want the one switched on", ids, err)
	}
	if set, err := st.GetFileSet(ids[0]); err != nil || set.Name != "photos" {
		t.Errorf("the set = %+v %v, want photos", set, err)
	}
}

func TestHomeStartsASectionOnItsNodeOverTheLink(t *testing.T) {
	home := homeWithRelay(t)
	_, id, _ := adoptedNode(t, home, newRepoStore())
	var ans nodeRunAnswer
	err := home.svc.callNodeByID(context.Background(), id, keydeposit.ProofNodeRun, nodeRunPath, nodeRunCall{Domain: "containers"}, &ans)
	if err != nil || !ans.OK || ans.Started != 0 {
		t.Errorf("a run of a node with nothing to back up = %+v %v", ans, err)
	}
	if err := home.svc.callNodeByID(context.Background(), "elsewhere", keydeposit.ProofNodeRun, nodeRunPath, nodeRunCall{Domain: "files"}, &ans); !errors.Is(err, errNodeAway) {
		t.Errorf("a run of a node home holds no link to = %v", err)
	}
}
