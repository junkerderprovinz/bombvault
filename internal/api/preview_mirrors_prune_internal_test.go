package api

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// previewRecorder notes which tags each preview asks about together.
type previewRecorder struct {
	*placementEngine
	mu    sync.Mutex
	asked []string
}

func (e *previewRecorder) ForgetPreview(ctx context.Context, repo string, p restic.RetentionPolicy, m restic.Mode, tags ...string) ([]restic.ForgetGroup, error) {
	e.mu.Lock()
	e.asked = append(e.asked, strings.Join(tags, " + "))
	e.mu.Unlock()
	return e.placementEngine.ForgetPreview(ctx, repo, p, m, tags...)
}

func keepLastOne(t *testing.T, f *placementFixture) {
	t.Helper()
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.RetentionKeepLast = 1
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestThePreviewAsksAboutARenamedEntryAsOneAsThePruneDoes(t *testing.T) {
	f := newPlacementFixture(t)
	keepLastOne(t, f)
	tg := f.container("radarr", "")
	if _, err := f.st.AddAliasAt("container", "radarr-movies", tg.ID, 1000); err != nil {
		t.Fatal(err)
	}
	own := f.domainPath("containers")
	f.makeRepo(own)
	f.hold(own,
		snap("a1", 100, "container:radarr-movies"),
		snap("a2", 200, "container:radarr-movies"),
		snap("b1", 3000, "container:radarr"))
	rec := &previewRecorder{placementEngine: f.eng}
	f.svc.engine = rec

	if _, err := f.svc.PreviewRetention(context.Background(), "containers", "local"); err != nil {
		t.Fatal(err)
	}
	if len(rec.asked) != 1 || !strings.Contains(rec.asked[0], "container:radarr-movies") || !strings.Contains(rec.asked[0], "container:radarr") {
		t.Fatalf("the preview asked %q, want both names of the entry together", rec.asked)
	}
}

func TestThePreviewKeepsATagThePruneLeavesOut(t *testing.T) {
	f := newPlacementFixture(t)
	keepLastOne(t, f)
	tg := f.container("radarr", "")
	if _, err := f.st.AddAliasAt("container", "radarr-movies", tg.ID, 1000); err != nil {
		t.Fatal(err)
	}
	own := f.domainPath("containers")
	f.makeRepo(own)
	f.hold(own,
		snap("a1", 100, "container:radarr-movies"),
		snap("a2", 2000, "container:radarr-movies"),
		snap("b1", 3000, "container:radarr"))
	rec := &previewRecorder{placementEngine: f.eng}
	f.svc.engine = rec

	p, err := f.svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(rec.asked, func(a string) bool { return strings.Contains(a, "radarr-movies") }) {
		t.Fatalf("the preview asked about a tag the prune leaves out: %q", rec.asked)
	}
	for _, repo := range p.Repos {
		for _, item := range repo.Items {
			if item.Tag == "container:radarr-movies" && (len(item.Keep) != 2 || len(item.Remove) != 0) {
				t.Fatalf("the left out tag reads %+v", item)
			}
		}
	}
}

func TestAStatusCountsTheRuleOfACopyMadeThroughALocation(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.keptLocation(store.RetentionKeep{KeepDaily: 7})
	if res := f.do("POST", "/api/offsite/destinations/"+d.ID+"/domains/containers", nil); res["ok"] != true {
		t.Fatal(res)
	}
	all, err := f.svc.DomainStatus()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		if s.Domain == "containers" && !s.PruneStrategySet {
			t.Fatal("a section whose copy prunes by its card reads No prune rule set")
		}
	}
}
