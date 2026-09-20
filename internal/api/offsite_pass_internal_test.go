package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// keepLast gives a target a keep-last policy.
func keepLast(t *testing.T, f *placementFixture, target store.OffsiteTarget, n int) store.OffsiteTarget {
	t.Helper()
	target.RetentionKeepLast = n
	stored, err := f.st.UpsertOffsiteTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

// offsiteRuns lists a domain's off-site runs as "<target> ok=<0|1> <error>",
// without the old run replicated writes.
func offsiteRuns(t *testing.T, f *placementFixture, domain string) []string {
	t.Helper()
	rows, err := f.db.Query(`SELECT offsite_target_id, ok, error FROM offsite_runs
		WHERE domain = ? AND started_at > 1 ORDER BY rowid`, domain)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []string{}
	for rows.Next() {
		var target, text string
		var ok int
		if err := rows.Scan(&target, &ok, &text); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s ok=%d %s", target, ok, text))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// placementWebhook sends the notifications to a local webhook and returns what
// it has received so far.
func placementWebhook(t *testing.T, f *placementFixture) func() []string {
	t.Helper()
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)
	c := notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL, WebhookFormat: "generic"}
	if err := f.svc.SetNotifyConfig(c); err != nil {
		t.Fatal(err)
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

func TestAPausedDomainCopiesAndAgesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 3)
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	f.setDefault("containers", "")
	if _, err := f.st.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite on a paused domain = %v, want nil", err)
	}
	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), f.domainPath("containers"), "container:nginx")

	if len(f.eng.copies)+len(f.eng.forgets)+len(f.eng.prunes) != 0 {
		t.Fatalf("a paused domain copied %+v, forgot %+v, pruned %v", f.eng.copies, f.eng.forgets, f.eng.prunes)
	}
	if runs := offsiteRuns(t, f, "containers"); len(runs) != 0 {
		t.Fatalf("a paused domain wrote the runs %v", runs)
	}
}

func TestUnreadableRulesFailEveryTargetOfTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 3)
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	breakRule(t, f, "containers", "container:nginx")

	err := f.svc.ReplicateOffsite(context.Background(), "containers")
	if !errors.Is(err, errPlacementUnreadable) {
		t.Fatalf("ReplicateOffsite = %v, want errPlacementUnreadable", err)
	}
	if len(f.eng.copies)+len(f.eng.forgets)+len(f.eng.prunes) != 0 {
		t.Fatalf("unreadable rules copied %+v, forgot %+v, pruned %v", f.eng.copies, f.eng.forgets, f.eng.prunes)
	}
	want := []string{
		b2.ID + " ok=0 " + store.ReasonCopyRulesUnreadable,
		hz.ID + " ok=0 " + store.ReasonCopyRulesUnreadable,
	}
	if got := offsiteRuns(t, f, "containers"); !slices.Equal(got, want) {
		t.Fatalf("runs = %v, want %v", got, want)
	}
	runs, err := f.st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(runs, func(r store.Run) bool { return r.Kind == "offsite" })
	if i < 0 || runs[i].Status != "failed" {
		t.Fatalf("activity = %+v, want a failed off-site run", runs)
	}
}

func TestTheHookReportsRulesItCannotRead(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	breakRule(t, f, "containers", "container:plex")
	sent := placementWebhook(t, f)

	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), f.domainPath("containers"), "container:nginx")

	if len(f.eng.copies) != 0 {
		t.Fatalf("the hook copied %+v with rules it could not read", f.eng.copies)
	}
	if got, want := offsiteRuns(t, f, "containers"), []string{b2.ID + " ok=0 " + store.ReasonCopyRulesUnreadable}; !slices.Equal(got, want) {
		t.Fatalf("runs = %v, want %v", got, want)
	}
	if msgs := sent(); len(msgs) != 1 || !strings.Contains(msgs[0], "containers") {
		t.Fatalf("notifications = %q, want one naming the domain", msgs)
	}
}

func TestOnlyTheSettingsFieldsTargetAndARuleFailLoudly(t *testing.T) {
	f := newPlacementFixture(t)
	settings := settingsOf(t, f.svc)
	settings.ContainersOffsite = "b2:bucket:containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.replicated("containers")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))

	err := f.svc.ReplicateOffsite(context.Background(), "containers")
	if !errors.Is(err, errTargetsUncertain) {
		t.Fatalf("ReplicateOffsite = %v, want errTargetsUncertain", err)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v to a target no rule can name", f.eng.copies)
	}
	if got, want := offsiteRuns(t, f, "containers"), []string{" ok=0 " + truncateRunErr(errTargetsUncertain)}; !slices.Equal(got, want) {
		t.Fatalf("runs = %v, want %v", got, want)
	}
}

func TestTheHookVisitsOnlyTheTargetsItsItemIsCopiedTo(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	f.replicated("containers")
	f.rule("containers", "container:nginx", hz.ID)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), f.domainPath("containers"), "container:nginx")

	var dests []string
	for _, c := range f.eng.copies {
		dests = append(dests, c.Dest)
	}
	if !slices.Equal(dests, []string{"b2:bucket:containers"}) {
		t.Fatalf("the hook copied to %v, want only B2", dests)
	}
	if got, want := offsiteRuns(t, f, "containers"), []string{b2.ID + " ok=1 "}; !slices.Equal(got, want) {
		t.Fatalf("runs = %v, want %v", got, want)
	}
}

func TestAHookForAnItemOnLocalLeavesNoRun(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), f.domainPath("containers"), "container:nginx")

	if len(f.eng.copies) != 0 || len(offsiteRuns(t, f, "containers")) != 0 {
		t.Fatalf("an item on Local copied %+v and wrote %v", f.eng.copies, offsiteRuns(t, f, "containers"))
	}
}

// TestTheHookSamplesTheTargetItActuallyCopiedTo pins that a hook pass narrowed to
// one target (by a rule that skips the domain's other targets) samples that
// target's own size. The domain's bare "offsite" source resolves to the first
// enabled target regardless of which one a pass actually reaches, so treating a
// narrowed pass as single-destination attributes the sample to the wrong
// repository and leaves the copied-to target's own trend stale.
func TestTheHookSamplesTheTargetItActuallyCopiedTo(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	b2.GrowthBudgetGB = 1
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	hz.GrowthBudgetGB = 1
	if _, err := f.st.UpsertOffsiteTarget(hz); err != nil {
		t.Fatal(err)
	}
	settings := settingsOf(t, f.svc)
	settings.OffsiteGrowthBudgetGB = 1
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.replicated("containers")
	f.rule("containers", "container:plex", b2.ID) // plex skips B2, so only Hetzner is copied to
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold("b2:bucket:containers", snap("old", 50, "container:plex")) // B2 already holds an earlier copy

	beforeDomain, err := f.st.ListRepoStats("containers", "offsite", 0)
	if err != nil {
		t.Fatal(err)
	}
	beforeHZ, err := f.st.ListRepoStats("containers", offsiteStatSource(hz.ID), 0)
	if err != nil {
		t.Fatal(err)
	}

	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), f.domainPath("containers"), "container:plex")

	afterDomain, err := f.st.ListRepoStats("containers", "offsite", 0)
	if err != nil {
		t.Fatal(err)
	}
	afterHZ, err := f.st.ListRepoStats("containers", offsiteStatSource(hz.ID), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterHZ) != len(beforeHZ)+1 {
		t.Fatalf("Hetzner's own size samples = %d, want %d: the target this pass copied to must get its own sample", len(afterHZ), len(beforeHZ)+1)
	}
	if len(afterDomain) != len(beforeDomain) {
		t.Fatalf("the domain's bare offsite source samples = %d, want %d unchanged: B2 was never copied to by this pass", len(afterDomain), len(beforeDomain))
	}
}
