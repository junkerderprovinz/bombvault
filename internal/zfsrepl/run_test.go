package zfsrepl

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	base       = "tank/bombvault-replica/bottich"
	rootTarget = base + "/cache/appdata"
)

type rig struct {
	t        *testing.T
	src, dst *fakeHost
	now      time.Time
	entry    Entry
}

// newRig is a source with cache/appdata and one child, and a target that has
// only its pool.
func newRig(t *testing.T) *rig {
	w := &fakeWorld{}
	r := &rig{t: t, src: newFakeHost(w), dst: newFakeHost(w), now: time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)}
	r.src.add("cache", "filesystem", false)
	r.src.add("cache/appdata", "filesystem", false)
	r.src.add("cache/appdata/plex", "filesystem", false)
	r.dst.add("tank", "filesystem", false)
	r.entry = Entry{Root: "cache/appdata", TargetBase: base, Now: func() time.Time { return r.now }}
	return r
}

// run replicates once, keeps what the run created the way the store will, and
// moves the clock on by a day.
func (r *rig) run() Result {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := Run(ctx, r.src, r.dst, r.entry)
	if err != nil {
		r.t.Fatalf("Run: %v", err)
	}
	r.entry.Placeholders = append(r.entry.Placeholders, res.Created...)
	r.now = r.now.Add(24 * time.Hour)
	return res
}

func (r *rig) ok(res Result) {
	r.t.Helper()
	for _, m := range res.Members {
		if m.Code != "" || m.Snapshot != res.Snapshot {
			r.t.Fatalf("%s: code %q, snapshot %q, err %v", m.Dataset, m.Code, m.Snapshot, m.Err)
		}
	}
}

func member(t *testing.T, res Result, dataset string) MemberResult {
	t.Helper()
	for _, m := range res.Members {
		if m.Dataset == dataset {
			return m
		}
	}
	t.Fatalf("no result for %s in %+v", dataset, res.Members)
	return MemberResult{}
}

// hasSeq reports whether args holds want as one contiguous run.
func hasSeq(args []string, want ...string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		if slices.Equal(args[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

// callFor returns the one recorded argv of sub whose last argument starts with
// name, leaving out the dry runs of an estimate.
func callFor(t *testing.T, h *fakeHost, sub, name string) []string {
	t.Helper()
	var found [][]string
	for _, c := range h.callsOf(sub) {
		last := c[len(c)-1]
		if slices.Contains(c, "-n") {
			continue
		}
		if last == name || strings.HasPrefix(last, name+"@") {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d %s calls for %s, want 1: %q", len(found), sub, name, found)
	}
	return found[0]
}

func streamsOf(h *fakeHost) [][]string {
	var out [][]string
	for _, c := range h.callsOf("send") {
		if !slices.Contains(c, "-n") {
			out = append(out, c)
		}
	}
	return out
}

func TestFirstRunSendsEveryMemberInFull(t *testing.T) {
	r := newRig(t)
	res := r.run()
	r.ok(res)

	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		m := member(t, res, ds)
		if m.Base != "" || m.Target != base+"/"+ds || m.Bytes == 0 {
			t.Errorf("%s: %+v", ds, m)
		}
		send := callFor(t, r.src, "send", ds)
		if !hasSeq(send, "-c", "-L", "-e", "-p") || slices.Contains(send, "-i") {
			t.Errorf("full send of %s = %q", ds, send)
		}
		recv := callFor(t, r.dst, "receive", base+"/"+ds)
		for _, want := range [][]string{{"-s", "-u"}, {"-o", "readonly=on"}, {"-o", "canmount=noauto"}, {"-x", "mountpoint"}, {"-x", "sharenfs"}, {"-x", "sharesmb"}, {"-x", "reservation"}, {"-x", "refreservation"}} {
			if !hasSeq(recv, want...) {
				t.Errorf("receive into %s lacks %q: %q", ds, want, recv)
			}
		}
		if slices.Contains(recv, "-F") {
			t.Errorf("a first receive into a new dataset carries -F: %q", recv)
		}
		got := r.dst.get(base + "/" + ds)
		if got == nil || len(got.snaps) != 1 || got.snaps[0].guid != m.GUID {
			t.Errorf("%s on the target: %+v, want the snapshot with guid %d", ds, got, m.GUID)
		}
		src := r.src.get(ds)
		if !src.holds[res.Snapshot] || !slices.Equal(r.src.markNames(ds), []string{res.Snapshot}) {
			t.Errorf("%s on the source: holds %v, bookmarks %v", ds, src.holds, r.src.markNames(ds))
		}
	}
}

func TestParentsAreCreatedTopDownAndNeverMount(t *testing.T) {
	r := newRig(t)
	res := r.run()
	r.ok(res)

	want := []string{"tank/bombvault-replica", base, base + "/cache"}
	if !slices.Equal(res.Created, want) {
		t.Fatalf("Created = %q, want %q", res.Created, want)
	}
	creates := r.dst.callsOf("create")
	if len(creates) != len(want) {
		t.Fatalf("create calls = %q", creates)
	}
	for i, c := range creates {
		if !hasSeq(c, "create", "-p", "-u", "-o", "canmount=off", want[i]) {
			t.Errorf("create %d = %q, want %s with canmount=off", i, c, want[i])
		}
	}

	r.dst.resetCalls()
	r.ok(r.run())
	if c := r.dst.callsOf("create"); c != nil {
		t.Errorf("a second run created parents again: %q", c)
	}
}

func TestEveryCreatedParentCarriesTheOwner(t *testing.T) {
	r := newRig(t)
	r.entry.Owner = "4f2a"
	res := r.run()
	r.ok(res)
	for _, ds := range res.Created {
		if got := r.dst.get(ds).props["bombvault:source"]; got != "4f2a" {
			t.Errorf("%s is marked %q, want the owner", ds, got)
		}
	}
	if got := r.dst.get(rootTarget).props["bombvault:source"]; got != "" {
		t.Errorf("the received root is marked %q itself", got)
	}
}

func TestFinishedHearsEveryMemberBeforeRunReturns(t *testing.T) {
	r := newRig(t)
	var heard []string
	r.entry.Finished = func(m MemberResult) { heard = append(heard, m.Dataset+":"+m.Code) }
	r.ok(r.run())
	if want := []string{"cache/appdata:", "cache/appdata/plex:"}; !slices.Equal(heard, want) {
		t.Errorf("Finished heard %q, want %q", heard, want)
	}
}

func TestAPoolRootReplacesThePlaceholderItsPathHolds(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())

	r.entry.Root = "cache"
	r.entry.Excluded = []string{"cache/appdata"}
	res := r.run()
	r.ok(res)

	recv := callFor(t, r.dst, "receive", base+"/cache")
	if !hasSeq(recv, "-s", "-u", "-F", "-o", "readonly=on") {
		t.Errorf("the first receive over the placeholder = %q, want -F on a full stream", recv)
	}
	if r.dst.get(rootTarget) == nil || len(r.dst.get(base+"/cache").snaps) != 1 {
		t.Error("the pool root did not land, or took the replica below it along")
	}
}

func TestAnExistingTargetBombVaultDidNotCreateIsNeverOverwritten(t *testing.T) {
	r := newRig(t)
	r.dst.add("tank/bombvault-replica", "filesystem", false)
	r.dst.add(base, "filesystem", false)
	r.dst.add(base+"/cache", "filesystem", false)
	r.dst.add(rootTarget, "filesystem", false)

	res := r.run()
	m := member(t, res, "cache/appdata")
	if m.Code != "no-common-base" {
		t.Fatalf("code = %q, err %v, want no-common-base", m.Code, m.Err)
	}
	if r.dst.callsOf("receive") != nil && slices.ContainsFunc(r.dst.callsOf("receive"), func(c []string) bool { return c[len(c)-1] == rootTarget }) {
		t.Error("something was received into a dataset BombVault did not create")
	}
}

func TestIncrementalStartsFromTheNewestCommonSnapshot(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	second := r.run()
	r.ok(second)
	r.src.resetCalls()
	r.dst.resetCalls()
	third := r.run()
	r.ok(third)

	m := member(t, third, "cache/appdata")
	if m.Base != second.Snapshot || m.FromBookmark {
		t.Fatalf("base = %q (bookmark %v), want the snapshot %s", m.Base, m.FromBookmark, second.Snapshot)
	}
	send := callFor(t, r.src, "send", "cache/appdata")
	if !hasSeq(send, "-p", "-i", "@"+second.Snapshot) || slices.Contains(send, "-I") {
		t.Errorf("send = %q", send)
	}
	if recv := callFor(t, r.dst, "receive", rootTarget); !slices.Contains(recv, "-F") || slices.Contains(recv, "readonly=on") {
		t.Errorf("an increment onto an unchanged replica = %q, want -F and no creation properties", recv)
	}

	src := r.src.get("cache/appdata")
	if got := r.src.snapNames("cache/appdata"); !slices.Equal(got, []string{third.Snapshot}) {
		t.Errorf("source snapshots = %q, want only the newest", got)
	}
	if len(src.holds) != 1 || !src.holds[third.Snapshot] {
		t.Errorf("source holds = %v", src.holds)
	}
	if got := r.src.markNames("cache/appdata"); !slices.Equal(got, []string{first.Snapshot, second.Snapshot, third.Snapshot}) {
		t.Errorf("bookmarks = %q, want every replicated state", got)
	}
	if got := r.dst.snapNames(rootTarget); !slices.Equal(got, []string{first.Snapshot, second.Snapshot, third.Snapshot}) {
		t.Errorf("target snapshots = %q", got)
	}
}

func TestIncrementalStartsFromTheBookmarkOnceTheSnapshotIsGone(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	r.src.dropSnapshot("cache/appdata", first.Snapshot)

	r.src.resetCalls()
	second := r.run()
	r.ok(second)
	m := member(t, second, "cache/appdata")
	if m.Base != first.Snapshot || !m.FromBookmark {
		t.Fatalf("base = %q (bookmark %v)", m.Base, m.FromBookmark)
	}
	if send := callFor(t, r.src, "send", "cache/appdata"); !hasSeq(send, "-i", "#"+first.Snapshot) {
		t.Errorf("send = %q", send)
	}
}

func TestACutStreamIsResumedOnTheNextRun(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	r.src.payload["cache/appdata"] = 8192
	r.src.cutAfter["cache/appdata"] = 3000
	cut := r.run()
	m := member(t, cut, "cache/appdata")
	if m.Code != "stream-cut" || r.dst.get(rootTarget).token == "" {
		t.Fatalf("the cut stream: code %q, token %q", m.Code, r.dst.get(rootTarget).token)
	}
	if !slices.Contains(r.src.snapNames("cache/appdata"), cut.Snapshot) {
		t.Fatal("the source dropped the snapshot the target can still resume towards")
	}

	delete(r.src.cutAfter, "cache/appdata")
	r.src.resetCalls()
	next := r.run()
	r.ok(next)
	m = member(t, next, "cache/appdata")
	if !m.Resumed || m.Base != cut.Snapshot {
		t.Fatalf("resumed %v, base %q, want a resume and then an increment from %s", m.Resumed, m.Base, cut.Snapshot)
	}
	streams := streamsOf(r.src)
	if len(streams) < 2 || !slices.Contains(streams[0], "-t") || !hasSeq(streams[1], "-i", "@"+cut.Snapshot) {
		t.Errorf("streams = %q, want the resume first", streams)
	}
	if got := r.dst.snapNames(rootTarget); len(got) != 3 || got[1] != cut.Snapshot || got[2] != next.Snapshot {
		t.Errorf("target snapshots = %q", got)
	}
}

func TestAReceiveKilledMidStreamIsReportedAsCutAndResumed(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	r.src.payload["cache/appdata"] = 8192
	r.dst.killReceive[rootTarget] = 3000
	cut := r.run()
	if m := member(t, cut, "cache/appdata"); m.Code != "stream-cut" {
		t.Fatalf("code = %q, err %v, want stream-cut", m.Code, m.Err)
	}
	if !slices.Contains(r.src.snapNames("cache/appdata"), cut.Snapshot) {
		t.Fatal("the source dropped the snapshot the target can still resume towards")
	}

	delete(r.dst.killReceive, rootTarget)
	next := r.run()
	r.ok(next)
	if m := member(t, next, "cache/appdata"); !m.Resumed || m.Base != cut.Snapshot {
		t.Fatalf("resumed %v, base %q, want a resume and then an increment from %s", m.Resumed, m.Base, cut.Snapshot)
	}
}

func TestACutFirstStreamIsResumedRatherThanSentAgain(t *testing.T) {
	r := newRig(t)
	r.src.payload["cache/appdata"] = 8192
	r.src.cutAfter["cache/appdata"] = 3000
	cut := r.run()
	partial := r.dst.get(rootTarget)
	if member(t, cut, "cache/appdata").Code == "" || partial == nil || partial.token == "" || len(partial.snaps) != 0 {
		t.Fatalf("a cut first stream left %+v", partial)
	}

	delete(r.src.cutAfter, "cache/appdata")
	r.src.resetCalls()
	r.dst.resetCalls()
	next := r.run()
	r.ok(next)
	if !member(t, next, "cache/appdata").Resumed {
		t.Fatal("not resumed")
	}
	for _, s := range streamsOf(r.src) {
		if s[len(s)-1] == "cache/appdata@"+next.Snapshot && !slices.Contains(s, "-i") {
			t.Errorf("a full stream went over the partial receive: %q", s)
		}
	}
	resumeRecv := r.dst.callsOf("receive")[0]
	if !hasSeq(resumeRecv, "-o", "readonly=on") {
		t.Errorf("the resumed first receive = %q, want the properties of a first receive", resumeRecv)
	}
}

func TestAStaleTokenIsAbortedAndTheMemberCarriesOn(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	r.src.cutAfter["cache/appdata"] = 1000
	cut := r.run()
	delete(r.src.cutAfter, "cache/appdata")
	r.src.dropSnapshot("cache/appdata", cut.Snapshot)

	r.dst.resetCalls()
	next := r.run()
	r.ok(next)
	m := member(t, next, "cache/appdata")
	if m.Resumed || m.Base != first.Snapshot {
		t.Errorf("resumed %v, base %q, want an increment from %s", m.Resumed, m.Base, first.Snapshot)
	}
	if !slices.ContainsFunc(r.dst.callsOf("receive"), func(c []string) bool { return hasSeq(c, "receive", "-A", rootTarget) }) {
		t.Errorf("the stale state was not aborted: %q", r.dst.callsOf("receive"))
	}
}

func TestAStaleTokenOnAFirstStreamStartsItAgain(t *testing.T) {
	r := newRig(t)
	r.src.cutAfter["cache/appdata"] = 1000
	cut := r.run()
	delete(r.src.cutAfter, "cache/appdata")
	r.src.dropSnapshot("cache/appdata", cut.Snapshot)

	next := r.run()
	r.ok(next)
	if m := member(t, next, "cache/appdata"); m.Resumed || m.Base != "" {
		t.Errorf("%+v, want a new full stream after the abort", m)
	}
}

func TestAWriteOnTheReplicaIsRolledBack(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	r.dst.get(rootTarget).modified = true
	r.dst.resetCalls()
	r.ok(r.run())
	if recv := callFor(t, r.dst, "receive", rootTarget); !slices.Contains(recv, "-F") {
		t.Errorf("receive = %q, want -F", recv)
	}
}

func TestAForeignSnapshotOnTheTargetSurvivesAnIncrement(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	r.dst.snapshot(rootTarget, "autosnap_daily")
	r.dst.resetCalls()
	res := r.run()
	r.ok(res)
	if recv := callFor(t, r.dst, "receive", rootTarget); slices.Contains(recv, "-F") {
		t.Errorf("receive = %q, which would destroy the newer snapshot", recv)
	}
	if !slices.Contains(r.dst.snapNames(rootTarget), "autosnap_daily") {
		t.Error("the foreign snapshot is gone")
	}
}

func TestAChangedTargetIsReportedAndKeepsItsSnapshots(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vault", "filesystem", true)
	r.ok(r.run())
	vault := base + "/cache/appdata/vault"
	r.dst.snapshot(vault, "manual")

	res := r.run()
	m := member(t, res, "cache/appdata/vault")
	if m.Code != "target-changed" {
		t.Fatalf("code = %q, err %v, want target-changed", m.Code, m.Err)
	}
	if !slices.Contains(r.dst.snapNames(vault), "manual") {
		t.Error("the snapshot on the target was destroyed")
	}
	if slices.Contains(r.src.snapNames("cache/appdata/vault"), res.Snapshot) {
		t.Error("the failed member kept its new snapshot on the source")
	}
	if member(t, res, "cache/appdata").Code != "" {
		t.Error("one member's failure stopped the others")
	}
}

func TestNoCommonBaseRefusesInsteadOfSendingInFull(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		for _, name := range r.src.snapNames(ds) {
			r.src.get(ds).holds = map[string]bool{}
			r.src.dropSnapshot(ds, name)
		}
		r.src.get(ds).marks = nil
	}
	r.src.resetCalls()
	r.dst.resetCalls()

	res := r.run()
	for _, m := range res.Members {
		if m.Code != "no-common-base" {
			t.Errorf("%s: code %q, want no-common-base", m.Dataset, m.Code)
		}
	}
	if s := r.src.callsOf("send"); s != nil {
		t.Errorf("something was sent: %q", s)
	}
	if rc := r.dst.callsOf("receive"); rc != nil {
		t.Errorf("something was received: %q", rc)
	}
	if len(r.dst.snapNames(rootTarget)) != 1 {
		t.Error("the target lost its data")
	}
}

func TestAReceiveThatFailsMidStreamDoesNotHang(t *testing.T) {
	r := newRig(t)
	r.src.payload["cache/appdata"] = 4 << 20
	r.dst.refuseReceive[rootTarget] = "cannot receive new filesystem stream: permission denied"

	done := make(chan Result, 1)
	go func() {
		res, err := Run(context.Background(), r.src, r.dst, r.entry)
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		done <- res
	}()
	select {
	case res := <-done:
		if m := member(t, res, "cache/appdata"); m.Code != "zfs-permission" {
			t.Errorf("code = %q, err %v, want the receive's own reason", m.Code, m.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the run hung on the send after the receive gave up")
	}
}

func TestASendThatFailsAfterACleanReceiveFailsTheMember(t *testing.T) {
	r := newRig(t)
	r.src.failSendWait["cache/appdata"] = true
	res := r.run()
	if m := member(t, res, "cache/appdata"); m.Code == "" || m.Snapshot != "" {
		t.Errorf("%+v, want a failure", m)
	}
}

func TestAnEncryptedMemberTravelsRaw(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vault", "filesystem", true)
	r.ok(r.run())
	r.src.resetCalls()
	r.ok(r.run())

	send := callFor(t, r.src, "send", "cache/appdata/vault")
	if !hasSeq(send, "send", "-w", "-p", "-i") || slices.Contains(send, "-c") {
		t.Errorf("send = %q, want a raw increment", send)
	}
	if !r.dst.get(base + "/cache/appdata/vault").encrypted {
		t.Error("the replica is not encrypted")
	}
	if send := callFor(t, r.src, "send", "cache/appdata/plex"); slices.Contains(send, "-w") {
		t.Errorf("a plain member went raw: %q", send)
	}
}

func TestAVolumeLandsWithoutFilesystemPropertiesOrReservations(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/vm", "volume", false)
	res := r.run()
	r.ok(res)
	if !member(t, res, "cache/appdata/vm").Volume {
		t.Error("the member is not marked as a volume")
	}
	recv := callFor(t, r.dst, "receive", base+"/cache/appdata/vm")
	if !hasSeq(recv, "-o", "readonly=on") || slices.Contains(recv, "canmount=noauto") || slices.Contains(recv, "mountpoint") ||
		!hasSeq(recv, "-x", "reservation", "-x", "refreservation") {
		t.Errorf("receive of a volume = %q", recv)
	}
	if r.dst.get(base+"/cache/appdata/vm").typ != "volume" {
		t.Error("the replica is not a volume")
	}
}

func TestExcludedChildrenAreNotSentAndKeepNoSnapshot(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/tmp", "filesystem", false)
	r.src.add("cache/appdata/tmp/cache", "filesystem", false)
	r.entry.Excluded = []string{"cache/appdata/tmp"}
	res := r.run()
	r.ok(res)
	if len(res.Members) != 2 {
		t.Errorf("members = %+v", res.Members)
	}
	for _, ds := range []string{"cache/appdata/tmp", "cache/appdata/tmp/cache"} {
		if got := r.src.snapNames(ds); len(got) != 0 {
			t.Errorf("%s keeps %q", ds, got)
		}
		if r.dst.get(base+"/"+ds) != nil {
			t.Errorf("%s reached the target", ds)
		}
	}
}

func TestARunRemovesWhatAnInterruptedRunLeftOnAnExcludedDataset(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/tmp", "filesystem", false)
	r.entry.Excluded = []string{"cache/appdata/tmp"}
	r.ok(r.run())

	// A run that died early left its recursive snapshot everywhere, and
	// another entry left one of its own on the excluded dataset.
	stopped := zfs.ReplicaSnapshotName(r.now.Add(-time.Hour))
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex", "cache/appdata/tmp"} {
		r.src.snapshot(ds, stopped)
	}
	foreign := zfs.ReplicaSnapshotName(r.now.Add(-2 * time.Hour))
	r.src.snapshot("cache/appdata/tmp", foreign)

	r.ok(r.run())
	if got := r.src.snapNames("cache/appdata/tmp"); !slices.Equal(got, []string{foreign}) {
		t.Errorf("cache/appdata/tmp keeps %q, want only %q", got, foreign)
	}
}

func TestAnExcludedDatasetKeepsTheBaseOfItsOwnEntry(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)

	pool := r.entry
	pool.Root, pool.Excluded = "cache", []string{"cache/appdata"}
	if _, err := Run(context.Background(), r.src, r.dst, pool); err != nil {
		t.Fatal(err)
	}
	if got := r.src.snapNames("cache/appdata"); !slices.Equal(got, []string{first.Snapshot}) {
		t.Fatalf("cache/appdata after the pool's run = %q, want only its own entry's base", got)
	}

	r.now = r.now.Add(time.Hour)
	next := r.run()
	r.ok(next)
	if m := member(t, next, "cache/appdata"); m.Base != first.Snapshot || m.FromBookmark {
		t.Errorf("base = %q (bookmark %v), want the snapshot", m.Base, m.FromBookmark)
	}
}

func TestRetentionPrunesOnlyReplicaSnapshotsOnTheTarget(t *testing.T) {
	r := newRig(t)
	r.entry.Keep = store.RetentionKeep{KeepLast: 2}
	var runs []Result
	for range 4 {
		res := r.run()
		r.ok(res)
		runs = append(runs, res)
		if len(runs) == 1 {
			r.dst.snapshot(rootTarget, "manual")
		}
	}
	got := r.dst.snapNames(rootTarget)
	want := []string{"manual", runs[2].Snapshot, runs[3].Snapshot}
	if !slices.Equal(got, want) {
		t.Errorf("target snapshots = %q, want %q", got, want)
	}
	if p := member(t, runs[3], "cache/appdata").Pruned; !slices.Equal(p, []string{runs[1].Snapshot}) {
		t.Errorf("pruned = %q", p)
	}
}

func TestProgressCountsTheStreamAgainstTheEstimate(t *testing.T) {
	r := newRig(t)
	r.src.payload["cache/appdata"] = 10000
	var done, total int64
	r.entry.Progress = func(m string, d, tot int64) {
		if m == "cache/appdata" {
			done, total = d, tot
		}
	}
	res := r.run()
	r.ok(res)
	if total != 10000 || done < 10000 || done != member(t, res, "cache/appdata").Bytes {
		t.Errorf("progress %d of %d, bytes %d", done, total, member(t, res, "cache/appdata").Bytes)
	}
}

func TestACancelledRunLeavesNoNewSnapshots(t *testing.T) {
	r := newRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := Run(ctx, r.src, r.dst, r.entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Members {
		if m.Code != "not-reached" {
			t.Errorf("%s: %q", m.Dataset, m.Code)
		}
		if got := r.src.snapNames(m.Dataset); len(got) != 0 {
			t.Errorf("%s keeps %q", m.Dataset, got)
		}
	}
}

func TestATreeWithANameTooLongForTheReplicaTakesNoSnapshot(t *testing.T) {
	r := newRig(t)
	long := "cache/appdata/" + strings.Repeat("n", zfs.MaxSnapshotNameLen-len("cache/appdata/@"+zfs.ReplicaPrefix)-14+1)
	r.src.add(long, "filesystem", false)
	r.entry.Excluded = []string{long}
	_, err := Run(context.Background(), r.src, r.dst, r.entry)
	if Code(err) != "name-too-long" {
		t.Fatalf("err = %v, want name-too-long", err)
	}
	if r.src.callsOf("snapshot") != nil {
		t.Error("the snapshot was taken anyway")
	}
}

func TestBookmarksGoOnceTheTargetPrunedTheirSnapshot(t *testing.T) {
	r := newRig(t)
	r.entry.Keep = store.RetentionKeep{KeepLast: 2}
	var runs []Result
	for range 4 {
		res := r.run()
		r.ok(res)
		runs = append(runs, res)
	}
	want := []string{runs[2].Snapshot, runs[3].Snapshot}
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		if got := r.src.markNames(ds); !slices.Equal(got, want) {
			t.Errorf("%s bookmarks = %q, want %q", ds, got, want)
		}
	}
}
