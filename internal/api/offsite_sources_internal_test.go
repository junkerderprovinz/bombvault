package api

import (
	"context"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// logBuffer collects what the package logs during one test.
type logBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func captureLog(t *testing.T) *logBuffer {
	t.Helper()
	l := &logBuffer{}
	prev := log.Writer()
	log.SetOutput(l)
	t.Cleanup(func() { log.SetOutput(prev) })
	return l
}

func TestADomainWhoseItemsAllLiveRemotelyIdles(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.container("nginx", box.ID)
	f.replicated("containers")
	if err := os.RemoveAll(f.domainPath("containers")); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite = %v, want an idle pass", err)
	}
	if len(f.eng.copies) != 0 || len(offsiteRuns(t, f, "containers")) != 0 {
		t.Fatalf("an idle pass copied %+v and wrote %v", f.eng.copies, offsiteRuns(t, f, "containers"))
	}
}

func TestASourceNothingIsCopiedFromIsNotReadWhenNoTargetHoldsItsItems(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("files", "B2", "b2:bucket:files")
	nas := f.namedRepo("NAS", "nas")
	f.fileSet("Photos", nas.ID)
	f.fileSet("Docs", "")
	f.rule("files", "fileset:Photos", store.SkipAll)
	f.replicated("files")
	f.listing("files", b2.ID, 1000)
	f.hold(f.domainPath("files"), snap("d1", 100, "fileset:Docs"))
	f.hold(f.root+"/nas", snap("p1", 100, "fileset:Photos"))

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if n := f.eng.lists[f.root+"/nas"]; n != 0 {
		t.Fatalf("the NAS was listed %d times; nothing of it is copied and B2 holds none of it", n)
	}
	if len(f.eng.copies) != 1 || !slices.Equal(f.eng.copies[0].IDs, []string{"d1"}) {
		t.Fatalf("copies = %+v, want d1 from the domain path", f.eng.copies)
	}
}

func TestAnUnreachableSourceNothingIsCopiedFromHoldsTheKeepPolicyQuietly(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := keepLast(t, f, f.target("files", "B2", "b2:bucket:files"), 3)
	nas := f.namedRepo("NAS", "nas")
	f.fileSet("Photos", nas.ID)
	f.fileSet("Docs", "")
	f.rule("files", "fileset:Photos", store.SkipAll)
	f.replicated("files")
	f.listing("files", b2.ID, 1000, copiesRow("fileset:Photos", 20, 900))
	if err := f.st.MarkRepoEstablished(f.root + "/nas"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.root + "/nas"); err != nil {
		t.Fatal(err)
	}
	f.hold(f.domainPath("files"), snap("d1", 100, "fileset:Docs"))
	logs := captureLog(t)

	f.svc.replicateOffsite(context.Background(), "files", settingsOf(t, f.svc), f.domainPath("files"), "fileset:Docs")
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatalf("ReplicateOffsite = %v; an unreachable repository nothing is copied from is no failure", err)
	}

	if len(f.eng.copies) == 0 {
		t.Fatal("Docs was not copied")
	}
	if len(f.eng.forgets)+len(f.eng.prunes) != 0 {
		t.Fatalf("B2 was aged (forgot %+v, pruned %v) while the NAS, whose items it holds, was unreachable", f.eng.forgets, f.eng.prunes)
	}
	if !strings.Contains(logs.String(), "NAS") {
		t.Fatalf("the log does not name the NAS:\n%s", logs.String())
	}
	for _, run := range offsiteRuns(t, f, "files") {
		if !strings.Contains(run, " ok=1 ") {
			t.Fatalf("runs = %v, want every run green", offsiteRuns(t, f, "files"))
		}
	}
}

func TestADomainPathHoldingOnlyProjectFoldersIsACountedSource(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 1)
	f.setDefault("containers", "", b2.ID)
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("s1", 100, "stack:immich"), snap("s2", 200, "stack:immich"))
	f.hold("b2:bucket:containers", copied("c1", "s1", 100, "stack:immich"), copied("c2", "s2", 200, "stack:immich"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v although the default leaves B2 out", f.eng.copies)
	}
	held, _ := f.eng.Snapshots(context.Background(), "b2:bucket:containers", restic.Mode{})
	if len(held) != 1 || restic.Identity(held[0]) != "s2" {
		t.Fatalf("B2 holds %+v, want the newest project folder only: the domain path answered for it", held)
	}
}

func TestASingleRepositoryDomainKeepsItsNeverCreatedSourceForRestic(t *testing.T) {
	f := newPlacementFixture(t)
	if err := os.RemoveAll(f.domainPath("files")); err != nil {
		t.Fatal(err)
	}

	sources, skipped := f.svc.offsiteReplicationSources(settingsOf(t, f.svc), "files")

	if len(sources) != 1 || !sources[0].Own {
		t.Fatalf("sources = %+v, want the domain's own repository kept so the caller still opens it and hears restic's own error", sources)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped = %+v, want none: a single missing repository is reported by the copy attempt itself, not noted here", skipped)
	}
}

func TestADomainWithAMissingNamedRepositoryIdlesQuietly(t *testing.T) {
	f := newPlacementFixture(t)
	cold := f.namedRepo("Cold", "cold")
	f.container("nginx", cold.ID)
	if err := os.RemoveAll(f.root + "/cold"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.domainPath("containers")); err != nil {
		t.Fatal(err)
	}

	sources, skipped := f.svc.offsiteReplicationSources(settingsOf(t, f.svc), "containers")

	if len(sources) != 0 {
		t.Fatalf("sources = %+v, want none: nothing this pass could reach exists yet", sources)
	}
	if i := slices.IndexFunc(skipped, func(s repoSkip) bool { return s.Note && s.Name == "containers" }); i < 0 {
		t.Fatalf("skipped = %+v, want a quiet note that nothing in containers is copied off site", skipped)
	}
}

func TestTheHookCopiesFromARepositoryItDoesNotRecognise(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("files", "B2", "b2:bucket:files")
	f.listing("files", b2.ID, 1000)
	f.rule("files", "fileset:Ghost", store.SkipAll)
	orphan := f.root + "/orphan"
	f.hold(orphan, snap("x1", 100, "fileset:Docs"))

	f.svc.replicateOffsite(context.Background(), "files", settingsOf(t, f.svc), orphan, "")

	if len(f.eng.copies) != 1 || f.eng.copies[0].Src != orphan {
		t.Fatalf("copies = %+v, want the hook's own source copied even though it names no domain path and no named repository", f.eng.copies)
	}
}
