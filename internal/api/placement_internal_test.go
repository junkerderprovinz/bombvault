package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func targetIDs(ts []store.OffsiteTarget) []string {
	out := []string{}
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

// breakRule stores a rule whose skip no reader can take.
func breakRule(t *testing.T, f *placementFixture, domain, identity string) {
	t.Helper()
	if _, err := f.db.Exec(`INSERT INTO offsite_copy_rules (domain, identity, skip) VALUES (?, ?, 'not json')`, domain, identity); err != nil {
		t.Fatal(err)
	}
}

func TestReadPlacementReadsEveryTargetOfTheDomainInOrder(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	hz.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(hz); err != nil {
		t.Fatal(err)
	}
	f.target("vms", "B2 vms", "b2:bucket:vms")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.readPlacement(settings, "containers")
	if err != nil || p.TargetsUncertain || !slices.Equal(targetIDs(p.Targets), []string{b2.ID, hz.ID}) {
		t.Fatalf("readPlacement = %+v, %v, want both containers targets in order", p, err)
	}
	if got := targetIDs(p.enabledTargets()); !slices.Equal(got, []string{b2.ID}) {
		t.Fatalf("enabledTargets = %v, want only B2", got)
	}
}

func TestOnlyTheSettingsFieldMakesTheTargetsUncertain(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FilesOffsite = "b2:bucket:files"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.readPlacement(settings, "files")
	if err != nil || !p.TargetsUncertain || len(p.Targets) != 1 || p.Targets[0].ID != "" || p.Targets[0].Repo != "b2:bucket:files" {
		t.Fatalf("readPlacement(files) = %+v, %v, want the target of the field, uncertain", p, err)
	}
	if p, err := f.svc.readPlacement(settings, "vms"); err != nil || p.TargetsUncertain || len(p.Targets) != 0 {
		t.Fatalf("readPlacement(vms) = %+v, %v, want no target and nothing uncertain", p, err)
	}
}

func TestAnUnreadableRuleMakesThePlacementUnreadable(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	breakRule(t, f, "containers", "container:nginx")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.readPlacement(settings, "containers")
	if !errors.Is(err, errPlacementUnreadable) {
		t.Fatalf("readPlacement = %v, want errPlacementUnreadable", err)
	}
	if !slices.Equal(targetIDs(p.Targets), []string{b2.ID}) {
		t.Fatalf("targets with the error = %v, want B2 for the failed rows", targetIDs(p.Targets))
	}
}

func TestTheSkipOfAnItemIsItsRuleOrTheDefault(t *testing.T) {
	p := placementRead{Domain: "containers", State: store.PlacementState{
		HasDefault: true, Default: store.PlacementDefault{Skip: []string{"t2"}, ConfirmedAt: 1},
		Rules: map[string]store.CopyRule{
			"container:nginx": {Skip: []string{store.SkipAll}},
			"stack:immich":    {Skip: []string{}},
		},
	}}
	for _, c := range []struct {
		identity string
		skip     []string
		own      bool
	}{
		{"container:nginx", []string{store.SkipAll}, true},
		{"container:plex", []string{"t2"}, false},
		{"stack:immich", []string{"t2"}, false},
	} {
		if skip, own := p.resolvedSkip(c.identity); !slices.Equal(skip, c.skip) || own != c.own {
			t.Errorf("resolvedSkip(%s) = %v, %v; want %v, %v", c.identity, skip, own, c.skip, c.own)
		}
	}
	if skip, own := (placementRead{}).resolvedSkip("container:x"); skip == nil || len(skip) != 0 || own {
		t.Errorf("without a default: %#v, %v; want an empty list", skip, own)
	}
}

func TestEffectiveTargetsAreTheEnabledOnesTheSkipLeavesIn(t *testing.T) {
	p := placementRead{
		Targets: []store.OffsiteTarget{{ID: "t1", Enabled: true}, {ID: "t2", Enabled: true}, {ID: "t3"}},
		State: store.PlacementState{Rules: map[string]store.CopyRule{
			"container:a": {Skip: []string{"t1"}},
			"container:b": {Skip: []string{store.SkipAll}},
		}},
	}
	for identity, want := range map[string][]string{
		"container:a": {"t2"},
		"container:b": {},
		"container:c": {"t1", "t2"},
	} {
		if got := targetIDs(p.effectiveTargets(identity)); !slices.Equal(got, want) {
			t.Errorf("effectiveTargets(%s) = %v, want %v", identity, got, want)
		}
	}
	if !p.anyCopiesTo("t1") || !p.anyCopiesTo("t2") {
		t.Error("the default copies to every enabled target")
	}
}

func TestASnapshotStaysAwayFromATargetOnlyWhenEveryOwnerLeavesItOut(t *testing.T) {
	p := placementRead{State: store.PlacementState{Rules: map[string]store.CopyRule{
		"vm:a":        {Skip: []string{store.SkipAll}},
		"vm:a:zvol:b": {Skip: []string{"t1"}},
	}}}
	if !p.copiesTo("t1", nil) {
		t.Error("a snapshot nobody owns is copied")
	}
	if p.copiesTo("t1", []string{"vm:a", "vm:a:zvol:b"}) {
		t.Error("both owners leave t1 out, yet it is copied there")
	}
	if !p.copiesTo("t2", []string{"vm:a", "vm:a:zvol:b"}) {
		t.Error("vm:a:zvol:b keeps t2, yet the snapshot stays away")
	}
	if !p.copiesTo("t1", []string{"vm:a", "vm:c"}) {
		t.Error("vm:c follows the default, yet the snapshot stays away")
	}
}

func TestTheRulesFingerprintFollowsRulesDefaultAndRetention(t *testing.T) {
	target := store.OffsiteTarget{ID: "t1", RetentionKeepLast: 7}
	base := placementRead{State: store.PlacementState{Rules: map[string]store.CopyRule{
		"container:a": {Skip: []string{store.SkipAll}},
	}}}
	rev := base.rulesRev(target)
	if again := base.rulesRev(target); again != rev {
		t.Fatalf("the fingerprint moved without a change: %s then %s", rev, again)
	}
	moreRules := placementRead{State: store.PlacementState{Rules: map[string]store.CopyRule{
		"container:a": {Skip: []string{store.SkipAll}},
		"container:b": {Skip: []string{"t1"}},
	}}}
	withDefault := base
	withDefault.State.HasDefault = true
	withDefault.State.Default = store.PlacementDefault{Skip: []string{"t1"}}
	longer := target
	longer.RetentionKeepDaily = 1
	for name, got := range map[string]string{
		"another rule":      moreRules.rulesRev(target),
		"another default":   withDefault.rulesRev(target),
		"another retention": base.rulesRev(longer),
	} {
		if got == rev {
			t.Errorf("%s left the fingerprint %s", name, rev)
		}
	}
}

func TestTheHomeKindOfAnItem(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	named := map[string]store.OffsiteTarget{nas.ID: nas, box.ID: box}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for repo, want := range map[string]homeKind{"": homeDomain, nas.ID: homeLocal, box.ID: homeRemote, "gone": homeMissing} {
		if got := f.svc.homeKindOf(settings, "containers", repo, named); got != want {
			t.Errorf("homeKindOf(%q) = %s, want %s", repo, got, want)
		}
	}
	settings.ContainersPath = "s3:https://s3.example/primary"
	if got := f.svc.homeKindOf(settings, "containers", "", named); got != homeDomainRemote {
		t.Errorf("a remote domain path is %s, want %s", got, homeDomainRemote)
	}
	for kind, want := range map[homeKind]bool{homeDomain: true, homeDomainRemote: true, homeLocal: true, homeRemote: false, homeMissing: false} {
		if kind.copySource() != want {
			t.Errorf("%s.copySource() = %v, want %v", kind, !want, want)
		}
	}
}

func TestItemIdentityIsTheSnapshotTag(t *testing.T) {
	f := newPlacementFixture(t)
	set := f.fileSet("Docs", "")
	for item, want := range map[store.ItemRef]string{
		{Domain: "containers", Key: "nginx"}: "container:nginx",
		{Domain: "vms", Key: "Windows 11"}:   "vm:Windows 11",
		{Domain: "files", Key: set.ID}:       "fileset:Docs",
	} {
		if got, err := f.svc.itemIdentity(item); err != nil || got != want {
			t.Errorf("itemIdentity(%+v) = %q, %v, want %q", item, got, err, want)
		}
	}
	if _, err := f.svc.itemIdentity(store.ItemRef{Domain: "files", Key: "0123abcd"}); !errors.Is(err, errFileSetNotFound) {
		t.Errorf("a missing set = %v, want errFileSetNotFound", err)
	}
}

func TestPlacementRefusalsCarryTheirCodes(t *testing.T) {
	want := map[error]string{
		errPlacementUnreadable:    "placement-unreadable",
		errInvalidPlacement:       "invalid-placement",
		store.ErrRuleDomain:       "invalid-placement",
		store.ErrUnknownDomain:    "invalid-placement",
		errCopiesNotAllowed:       "copies-not-allowed",
		errNotATarget:             "unknown-target",
		errUnknownOffsiteTarget:   "unknown-target",
		store.ErrStackCopyRule:    "stack-rule",
		store.ErrCopyRuleTaken:    "copy-rule-taken",
		errPlacementBusy:          "domain-busy",
		errHomeHasBackups:         "has-backups",
		errPlacementStale:         "stale",
		errForeignDomain:          "foreign-domain",
		errRepoInvalid:            "repo-invalid",
		errRepoInUse:              "repo-in-use",
		errDefaultRepoMissing:     "default-repo-missing",
		errNestedLocation:         "nested-location",
		errMirroredField:          "mirrored-field",
		store.ErrCompanionTaken:   "companion-taken",
		store.ErrNotOffsiteTarget: "unknown-target",
		errTargetInUse:            "target-in-use",
	}
	if len(placementCodes) != len(want) {
		t.Fatalf("placementCodes has %d rows, want %d: a sentinel is missing its row or its test", len(placementCodes), len(want))
	}
	for err, code := range want {
		if got := placementCode(fmt.Errorf("wrapped: %w", err)); got != code {
			t.Errorf("placementCode(%v) = %q, want %q", err, got, code)
		}
	}
	if got := placementCode(errors.New("something else")); got != "" {
		t.Errorf("an unknown error got the code %q", got)
	}
	rec := httptest.NewRecorder()
	placementFail(rec, fmt.Errorf("%w: disk", errPlacementUnreadable), map[string]any{"impact": 3})
	env := decodeEnvelope(t, rec)
	if rec.Code != http.StatusOK || env["ok"] != false || env["code"] != "placement-unreadable" || env["impact"] != float64(3) || env["error"] == "" {
		t.Fatalf("placementFail wrote %d %v", rec.Code, env)
	}
}

func TestPauseReasonsCoverExactlyWhatPausePlacementPasses(t *testing.T) {
	want := []pauseReason{reasonFoundHistory, reasonOlderSource, reasonDiscover}
	if len(pauseReasons) != len(want) {
		t.Fatalf("pauseReasons has %d entries, want exactly %v", len(pauseReasons), want)
	}
	for _, reason := range want {
		if pauseReasons[reason] == "" {
			t.Errorf("pauseReasons[%s] is empty", reason)
		}
	}
}

func TestItemParamChecksDomainAndName(t *testing.T) {
	h := &Handler{}
	var got []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /x/{domain}/{name}", func(w http.ResponseWriter, r *http.Request) {
		if domain, key, ok := h.itemParam(w, r, "containers", "vms", "files"); ok {
			got = append(got, domain+"/"+key)
			writeJSON(w, http.StatusOK, okEnvelope(nil))
		}
	})
	for _, c := range []struct {
		path string
		code int
	}{
		{"/x/containers/nginx", http.StatusOK},
		{"/x/vms/Windows%2011", http.StatusOK},
		{"/x/vms/a:zvol:b", http.StatusOK},
		{"/x/files/0123abcd", http.StatusOK},
		{"/x/flash/flash", http.StatusBadRequest},
		{"/x/containers/-rf", http.StatusBadRequest},
		{"/x/containers/a:b", http.StatusBadRequest},
		{"/x/nope/x", http.StatusBadRequest},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s = %d, want %d", c.path, rec.Code, c.code)
		}
	}
	if want := []string{"containers/nginx", "vms/Windows 11", "vms/a:zvol:b", "files/0123abcd"}; !slices.Equal(got, want) {
		t.Fatalf("accepted %v, want %v", got, want)
	}
}
