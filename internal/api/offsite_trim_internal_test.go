package api

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// copiedIDs lists the ids each copy call to dest carried, "whole" for a copy
// without ids.
func copiedIDs(f *placementFixture, dest string) [][]string {
	var out [][]string
	for _, c := range f.eng.copies {
		if c.Dest != dest {
			continue
		}
		if c.IDs == nil {
			out = append(out, []string{"whole"})
			continue
		}
		out = append(out, c.IDs)
	}
	return out
}

// trimScene is the files domain of the live test: A on the domain path, C on
// the NAS, B left out, and S3 keeping the last two of each. Both sources hold
// three backups.
func trimScene(t *testing.T) (*placementFixture, store.OffsiteTarget, string, store.FileSet) {
	t.Helper()
	f := newPlacementFixture(t)
	s3 := keepLast(t, f, f.target("files", "S3", "s3:minio/bucket/files"), 2)
	nas := f.namedRepo("NAS", "nas")
	a := f.fileSet("A", "")
	f.fileSet("B", "")
	f.fileSet("C", nas.ID)
	f.rule("files", "fileset:B", store.SkipAll)
	f.replicated("files")
	f.hold(f.domainPath("files"),
		snap("a1", 100, "fileset:A"), snap("a2", 200, "fileset:A"), snap("a3", 300, "fileset:A"),
		snap("b1", 100, "fileset:B"))
	f.hold(f.root+"/nas", snap("c1", 100, "fileset:C"), snap("c2", 200, "fileset:C"), snap("c3", 300, "fileset:C"))
	return f, s3, "s3:minio/bucket/files", a
}

// holdSeries opens a critical finding on the file set that pauses its aging.
func holdSeries(t *testing.T, f *placementFixture, fileSet store.FileSet) {
	t.Helper()
	f.settings(func(s *store.Settings) { s.AnomalyEnabled, s.AnomalyRetentionHold = true, true })
	row := openCritical("rewrite", metricNewDataRewrite, fileSet.ID, time.Now().Unix())
	row.TargetID, row.Domain = fileSet.ID, "files"
	if _, err := f.st.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{row}, Now: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	if held, err := f.svc.anomalies.HeldIdentityTags(); err != nil || !held.holds("fileset:"+fileSet.Name) {
		t.Fatalf("held = %v, %v, want the file set", held, err)
	}
}

func replicate(t *testing.T, f *placementFixture, domain string) {
	t.Helper()
	f.eng.copies, f.eng.forgets = nil, nil
	if err := f.svc.ReplicateOffsite(context.Background(), domain); err != nil {
		t.Fatalf("ReplicateOffsite = %v", err)
	}
}

func TestASnapshotTheTargetWouldForgetIsNotCopied(t *testing.T) {
	f, _, s3, _ := trimScene(t)

	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a2", "a3"}, {"c2", "c3"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("first pass copied %v, want %v", got, want)
	}
	replicate(t, f, "files")
	if got := copiedIDs(f, s3); len(got) != 0 {
		t.Fatalf("second pass copied %v again, which S3's keep-last 2 forgets right after", got)
	}
	if got, want := forgetLines(f, s3), []string{"fileset:A keep-last 2 prune=false", "fileset:C keep-last 2 prune=false"}; !slices.Equal(got, want) {
		t.Fatalf("forgets = %v, want the keep-policy still applied", got)
	}
	if n := len(heldAt(t, f, s3)); n != 4 {
		t.Fatalf("S3 holds %d, want the newest two of A and of C", n)
	}
}

func TestANewBackupIsCopiedAndTheOldestCopyAged(t *testing.T) {
	f, _, s3, _ := trimScene(t)
	replicate(t, f, "files")

	f.hold(f.domainPath("files"),
		snap("a1", 100, "fileset:A"), snap("a2", 200, "fileset:A"), snap("a3", 300, "fileset:A"), snap("a4", 400, "fileset:A"),
		snap("b1", 100, "fileset:B"))
	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a4"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("copied %v, want only the new backup", got)
	}
}

func TestARaisedRetentionAtTheTargetFetchesWhatItNowKeeps(t *testing.T) {
	f, target, s3, _ := trimScene(t)
	replicate(t, f, "files")
	replicate(t, f, "files")

	keepLast(t, f, target, 3)
	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a1"}, {"c1"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("after keep-last 3 copied %v, want %v", got, want)
	}
}

func TestAWholeCopyNarrowsToWhatTheTargetKeeps(t *testing.T) {
	f := newPlacementFixture(t)
	keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 2)
	f.replicated("containers")
	f.hold(f.domainPath("containers"),
		snap("n1", 100, "container:nginx"), snap("n2", 200, "container:nginx"), snap("n3", 300, "container:nginx"),
		snap("p1", 100, "container:plex"))

	replicate(t, f, "containers")
	if got, want := copiedIDs(f, "b2:bucket:containers"), [][]string{{"n2", "n3", "p1"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("copied %v, want %v", got, want)
	}
	replicate(t, f, "containers")
	if got := copiedIDs(f, "b2:bucket:containers"); len(got) != 0 {
		t.Fatalf("second pass copied %v", got)
	}
}

func TestAWholeCopyTheTargetKeepsEntirelyStaysWhole(t *testing.T) {
	f := newPlacementFixture(t)
	keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 5)
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("n1", 100, "container:nginx"), snap("n2", 200, "container:nginx"))

	replicate(t, f, "containers")
	if got, want := copiedIDs(f, "b2:bucket:containers"), [][]string{{"whole"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("copied %v, want %v", got, want)
	}
}

func TestAnAppendOnlyTargetGetsEverySnapshot(t *testing.T) {
	f, target, s3, _ := trimScene(t)
	f.appendOnly(target.ID)

	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a1", "a2", "a3"}, {"c1", "c2", "c3"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("copied %v, want everything, since nothing is forgotten there", got)
	}
}

func TestAHeldItemIsCopiedWhole(t *testing.T) {
	f, _, s3, a := trimScene(t)
	holdSeries(t, f, a)

	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a1", "a2", "a3"}, {"c2", "c3"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("copied %v, want all of A, whose aging is paused", got)
	}
}
