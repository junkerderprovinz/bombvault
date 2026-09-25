package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Two start calls that arrive together must not both see the last free slot.
func TestSlidingWindowReserveIsAtomic(t *testing.T) {
	w := newSlidingWindow(time.Hour, 2)
	now := time.Now()

	var mu sync.Mutex
	var releases []func()
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(20)
	for range 20 {
		go func() {
			defer done.Done()
			start.Wait()
			release, ok, _ := w.reserve("k1", now)
			if !ok {
				return
			}
			mu.Lock()
			releases = append(releases, release)
			mu.Unlock()
		}()
	}
	start.Done()
	done.Wait()

	if len(releases) != 2 {
		t.Fatalf("%d of 20 callers got a slot, want the window's 2", len(releases))
	}
	if _, ok, retry := w.reserve("k1", now); ok || retry <= 0 {
		t.Fatalf("a full window reserved again: ok=%v retry=%v", ok, retry)
	}

	releases[0]()
	if _, ok, _ := w.reserve("k1", now); !ok {
		t.Fatal("the released slot was not given back")
	}
	if _, ok, _ := w.reserve("k1", now); ok {
		t.Fatal("the window handed out a third slot")
	}
}

// newMCPStartHandler is a handler whose files domain answers a start. The sets
// point at folders that are not there, so a launched backup fails in its own
// goroutine without a repository or an engine behind it, which is all these
// tests need: they are about what happens before the service is asked.
func newMCPStartHandler(t *testing.T, sets ...string) (*Handler, *store.Repo, map[string]store.FileSet) {
	t.Helper()
	h, _, repo, _ := newMCPGateHandler(t)
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FilesEnabled = true
	settings.FilesPath = "backups/files"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]store.FileSet, len(sets))
	for _, name := range sets {
		set, cErr := repo.CreateFileSet(store.FileSet{Name: name, Path: "data/" + name, Enabled: true})
		if cErr != nil {
			t.Fatal(cErr)
		}
		out[name] = set
	}
	return h, repo, out
}

// mcpStartCaller is the context the gate hands a tool.
func mcpStartCaller(keyID string, canStart bool) context.Context {
	return withMCPCaller(context.Background(), mcpCaller{
		KeyID: keyID, Label: "Laptop", Hint: "x9Qa", CanStartBackups: canStart,
	})
}

// startFileSet calls start_backup for one folder set the way the transport
// would.
func startFileSet(ctx context.Context, h *Handler, name string) *mcp.CallToolResult {
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "start_backup",
		Arguments: json.RawMessage(fmt.Sprintf(`{"domain":"files","item":%q}`, name)),
	}}
	res, _ := h.toolStartBackup(ctx, req)
	return res
}

// waitForFilesIdle blocks until the detached backup has given the shared guard
// back, so the next start is not refused for the wrong reason.
func waitForFilesIdle(t *testing.T, h *Handler) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !h.svc.BackupInProgress() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the detached backup never finished")
}

// A refusal has to say which of the two single-flight guards turned the call
// away, and neither may cost the key a start.
func TestMCPStartBackupBusyReasons(t *testing.T) {
	h, _, _ := newMCPStartHandler(t, "docs")
	ctx := mcpStartCaller("0b7e", true)

	h.svc.batchActive.Store(true)
	res := startFileSet(ctx, h, "docs")
	if code := mcpErrorCode(t, res); code != "busy" {
		t.Fatalf("with a backup in flight the code is %q, want busy", code)
	}
	if msg := mcpErrorMessage(t, res); msg != "a backup is already running" {
		t.Fatalf("message = %q", msg)
	}
	h.svc.batchActive.Store(false)

	unlock := h.svc.lockDomainFor("files", "prune")
	res = startFileSet(ctx, h, "docs")
	unlock()
	if code := mcpErrorCode(t, res); code != "busy" {
		t.Fatalf("with the domain busy the code is %q, want busy", code)
	}
	if msg := mcpErrorMessage(t, res); !strings.Contains(msg, "prune is running on files") {
		t.Fatalf("message = %q, want it to name the operation and the domain", msg)
	}

	if left := h.mcp.starts.remaining("0b7e", h.mcp.now()); left != mcpStartsPerHour {
		t.Fatalf("%d starts left of %d: a refused call kept a slot", left, mcpStartsPerHour)
	}
}

// The retention guard only sees finished restore points, so a Backup Everything
// pass and a narrower start must never be in flight together: the pass would
// back up again what the other one is still backing up, and neither guard
// would have counted it.
func TestMCPStartsDoNotOverlapABackupEverythingPass(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t, "docs")
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled, s.VMsEnabled, s.FlashEnabled, s.ConfigEnabled = false, false, false, false
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	ctx := mcpStartCaller("0b7e", true)
	everything := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name: "start_backup_everything", Arguments: json.RawMessage(`{}`),
	}}

	h.svc.batchActive.Store(true)
	res, _ := h.toolStartBackupEverything(ctx, everything)
	h.svc.batchActive.Store(false)
	if code := mcpErrorCode(t, res); code != "busy" {
		t.Fatalf("with a backup in flight start_backup_everything gives %q, want busy", code)
	}
	if h.svc.EverythingInProgress() {
		t.Fatal("the refused call started a pass anyway")
	}

	h.svc.everythingActive.Store(true)
	res = startFileSet(ctx, h, "docs")
	h.svc.everythingActive.Store(false)
	if code := mcpErrorCode(t, res); code != "busy" {
		t.Fatalf("with a pass in flight start_backup gives %q, want busy", code)
	}
	if msg := mcpErrorMessage(t, res); !strings.Contains(msg, "Backup Everything") {
		t.Fatalf("message = %q, want it to name the pass", msg)
	}
	runs, err := repo.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("%d runs were recorded, want none", len(runs))
	}
	if left := h.mcp.starts.remaining("0b7e", h.mcp.now()); left != mcpStartsPerHour {
		t.Fatalf("%d starts left of %d: a refused call kept a slot", left, mcpStartsPerHour)
	}
}

// The budget counts what a key really launched, so an assistant cannot keep the
// server busy by retrying, and one key's spending is not another's.
func TestMCPStartBudget(t *testing.T) {
	h, _, _ := newMCPStartHandler(t, "docs", "media", "music")
	h.mcp.starts = newSlidingWindow(time.Hour, 2)
	ctx := mcpStartCaller("0b7e", true)

	for _, name := range []string{"docs", "media"} {
		if res := startFileSet(ctx, h, name); res.IsError {
			t.Fatalf("start of %s: %v", name, res.StructuredContent)
		}
		waitForFilesIdle(t, h)
	}

	res := startFileSet(ctx, h, "music")
	if code := mcpErrorCode(t, res); code != "rate_limited" {
		t.Fatalf("the third start gives %q, want rate_limited", code)
	}
	if secs := mcpErrorNumber(t, res, "retryAfterSeconds"); secs <= 0 {
		t.Fatalf("retryAfterSeconds = %v, want the wait until a slot frees", secs)
	}

	if res := startFileSet(mcpStartCaller("c41d", true), h, "music"); res.IsError {
		t.Fatalf("another key was held to the first one's budget: %v", res.StructuredContent)
	}
	waitForFilesIdle(t, h)
}

// A backup an assistant started a moment ago is not started again, whatever it
// was told in between; the web interface stays unrestricted.
func TestMCPStartCooldown(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "docs")
	base := time.Now()
	h.mcp.now = func() time.Time { return base }
	ctx := mcpStartCaller("0b7e", true)

	seedBackup(t, repo, sets["docs"].ID, "mcp", "0b7e")

	res := startFileSet(ctx, h, "docs")
	if code := mcpErrorCode(t, res); code != "cooldown" {
		t.Fatalf("code = %q, want cooldown", code)
	}

	h.mcp.now = func() time.Time { return base.Add(5 * time.Minute) }
	res = startFileSet(ctx, h, "docs")
	if code := mcpErrorCode(t, res); code != "cooldown" {
		t.Fatalf("five minutes on the code is %q, want cooldown", code)
	}
	if secs := mcpErrorNumber(t, res, "retryAfterSeconds"); secs != 600 {
		t.Fatalf("retryAfterSeconds = %v, want the 600 left of the cooldown", secs)
	}

	// What the web interface does, which no cooldown covers.
	seedBackup(t, repo, sets["docs"].ID, "", "")

	h.mcp.now = func() time.Time { return base.Add(mcpStartCooldown + time.Minute) }
	if res := startFileSet(ctx, h, "docs"); res.IsError {
		t.Fatalf("the cooldown has run out but the start was refused: %v", res.StructuredContent)
	}
	waitForFilesIdle(t, h)
}

// Under a policy that keeps a fixed number of restore points, MCP must never
// fill the window on its own: one of the kept points always comes from the
// schedule or from the operator.
func TestMCPRetentionGuardKeepsOlderRestorePoints(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "docs")
	set := sets["docs"]
	now := time.Now()
	keepLast := func(t *testing.T, n int) store.Settings {
		t.Helper()
		s, err := repo.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		s.RetentionKeepLast = n
		s.RetentionKeepDaily = 0
		if err := repo.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	item := mcpItem{Domain: "files", ID: set.ID, Name: set.Name}
	held := func(t *testing.T, s store.Settings) *mcpHeldBack {
		t.Helper()
		hold, err := h.mcpRetentionHold(s, item, now)
		if err != nil {
			t.Fatal(err)
		}
		return hold
	}

	s := keepLast(t, 3)
	for range 3 {
		seedBackup(t, repo, set.ID, "", "")
	}
	if hold := held(t, s); hold != nil {
		t.Fatalf("the first MCP backup was held back: %v", hold.detail)
	}

	seedBackup(t, repo, set.ID, "mcp", "0b7e")
	if hold := held(t, s); hold != nil {
		t.Fatalf("the second MCP backup was held back: %v", hold.detail)
	}

	seedBackup(t, repo, set.ID, "mcp", "0b7e")
	hold := held(t, s)
	if hold == nil {
		t.Fatal("a third MCP backup would leave only MCP-made restore points and was allowed")
	}
	if hold.detail["keepLast"] != 3 || hold.detail["mcpInWindow"] != 2 {
		t.Fatalf("detail = %v, want keepLast 3 and mcpInWindow 2", hold.detail)
	}

	seedBackup(t, repo, set.ID, "", "")
	if hold := held(t, s); hold != nil {
		t.Fatalf("a backup from elsewhere made no room: %v", hold.detail)
	}

	t.Run("a rule that keeps days holds the older ones itself", func(t *testing.T) {
		h, repo, sets := newMCPStartHandler(t, "docs")
		set := sets["docs"]
		s, err := repo.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		s.RetentionKeepLast = 3
		s.RetentionKeepDaily = 7
		if err := repo.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		seedBackup(t, repo, set.ID, "mcp", "0b7e")
		seedBackup(t, repo, set.ID, "mcp", "0b7e")

		hold, err := h.mcpRetentionHold(s, mcpItem{Domain: "files", ID: set.ID, Name: set.Name}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if hold != nil {
			t.Fatalf("the daily rule keeps the older days, but the start was held back: %v", hold.detail)
		}
	})

	t.Run("a window of one leaves no room at all", func(t *testing.T) {
		s := keepLast(t, 1)
		if hold := held(t, s); hold == nil {
			t.Fatal("with one kept restore point MCP may never start this item, but it was allowed")
		}
	})

	t.Run("the daily limit counts attempts, not restore points", func(t *testing.T) {
		h, repo, sets := newMCPStartHandler(t, "media")
		set := sets["media"]
		for range mcpItemStartsPerDay / 2 {
			seedBackup(t, repo, set.ID, "mcp", "0b7e")
			seedFailedBackup(t, repo, set.ID, "0b7e")
		}
		s, err := repo.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		hold, err := h.mcpRetentionHold(s, mcpItem{Domain: "files", ID: set.ID, Name: set.Name}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if hold == nil {
			t.Fatalf("a fifth MCP backup of the day was allowed under a keep-everything policy")
		}
		if hold.detail["mcpStartsToday"] != mcpItemStartsPerDay {
			t.Fatalf("detail = %v, want the day's count", hold.detail)
		}
	})

	t.Run("a start that was cancelled again still counts against the day", func(t *testing.T) {
		h, repo, sets := newMCPStartHandler(t, "media")
		set := sets["media"]
		for range mcpItemStartsPerDay {
			id, err := repo.StartRunWith(set.ID, "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: "0b7e"})
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.FinishRun(id, "cancelled", "", 0, store.ReasonCancelled); err != nil {
				t.Fatal(err)
			}
		}
		s, err := repo.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		hold, err := h.mcpRetentionHold(s, mcpItem{Domain: "files", ID: set.ID, Name: set.Name}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if hold == nil {
			t.Fatal("start and cancel took the item down more often than the daily limit allows")
		}
	})

	t.Run("a domain start leaves the guarded item out", func(t *testing.T) {
		h, repo, sets := newMCPStartHandler(t, "docs", "media")
		s, err := repo.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		s.RetentionKeepLast = 2
		if err := repo.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		seedBackup(t, repo, sets["docs"].ID, "mcp", "0b7e")
		// Past the cooldown of that backup, so the guard is what answers here.
		h.mcp.now = func() time.Time { return time.Now().Add(mcpStartCooldown + time.Minute) }

		req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
			Name:      "start_domain_backup",
			Arguments: json.RawMessage(`{"domain":"files"}`),
		}}
		res, _ := h.toolStartDomainBackup(mcpStartCaller("0b7e", true), req)
		if res.IsError {
			t.Fatalf("start_domain_backup: %v", res.StructuredContent)
		}
		out, _ := res.StructuredContent.(map[string]any)
		items, _ := out["items"].([]mcpStartItem)
		skipped, _ := out["skipped"].([]mcpSkipped)
		if len(items) != 1 || items[0].Name != "media" {
			t.Fatalf("started %v, want the set the guard leaves alone", items)
		}
		if len(skipped) != 1 || skipped[0].Name != "docs" || skipped[0].Reason != "retention_guard" {
			t.Fatalf("skipped = %v, want the guarded set with its reason", skipped)
		}
		waitForFilesIdle(t, h)
	})
}

// A ZFS run that could not read one dataset fails, but the datasets it did
// read wrote snapshots that retention ages all the same, so the guard counts
// those snapshots and not the successful runs.
func TestMCPRetentionGuardCountsTheSnapshotsOfEachZFSDataset(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t)
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.RetentionKeepLast = 3
	s.RetentionKeepDaily = 0
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	d, err := repo.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	item := mcpItem{Domain: zfsDomain, ID: d.ID, Name: d.Dataset}
	halfRun := func(via string) {
		t.Helper()
		id, err := repo.StartRunWith(d.ID, "backup", store.RunMeta{StartedVia: via, StartedViaKey: "0b7e"})
		if err != nil {
			t.Fatal(err)
		}
		for dataset, outcome := range map[string]string{"cache/appdata": "backed-up", "cache/appdata/nextcloud": "backup-failed"} {
			if err := repo.AddZFSRunMember(store.ZFSRunMember{RunID: id, Dataset: dataset, Outcome: outcome}); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.FinishRun(id, "failed", "", 0, "zfs backup: cache/appdata/nextcloud [backup-failed]"); err != nil {
			t.Fatal(err)
		}
	}

	halfRun("")
	halfRun("mcp")
	if hold, err := h.mcpRetentionHold(s, item, time.Now()); err != nil || hold != nil {
		t.Fatalf("one MCP snapshot in the window was held back: %v, %v", hold, err)
	}
	halfRun("mcp")
	hold, err := h.mcpRetentionHold(s, item, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if hold == nil {
		t.Fatal("the scheduled snapshot of cache/appdata would rotate out behind MCP ones, and the start was allowed")
	}
	if hold.detail["keepLast"] != 3 || hold.detail["mcpInWindow"] != 2 {
		t.Fatalf("detail = %v, want keepLast 3 and mcpInWindow 2", hold.detail)
	}
}

// Off-site retention runs per destination, so the guard has to hold a start to
// the tightest destination the item replicates to, not to the global columns
// that only seed the first one.
func TestMCPRetentionGuardReadsEachOffsiteDestination(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "docs")
	set := sets["docs"]
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.RetentionKeepLast = 0
	s.RetentionKeepDaily = 7
	s.OffsiteRetentionKeepLast = 0
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	for _, dest := range []store.OffsiteTarget{
		{Domain: "files", Name: "NAS", Repo: "rest:http://nas/files", RetentionKeepLast: 5, Enabled: true},
		{Domain: "files", Name: "B2", Repo: "b2:bucket:files", RetentionKeepLast: 2, Enabled: true},
		{Domain: "files", Name: "Vault", Repo: "rest:http://vault/files", RetentionKeepLast: 1, Immutable: true, Enabled: true},
		{Domain: "files", Name: "Old", Repo: "rest:http://old/files", RetentionKeepLast: 1, Enabled: false},
	} {
		if _, err := repo.UpsertOffsiteTarget(dest); err != nil {
			t.Fatal(err)
		}
	}
	item := mcpItem{Domain: "files", ID: set.ID, Name: set.Name}

	seedBackup(t, repo, set.ID, "", "")
	if hold, err := h.mcpRetentionHold(s, item, time.Now()); err != nil || hold != nil {
		t.Fatalf("the first MCP backup was held back: %v, %v", hold, err)
	}

	seedBackup(t, repo, set.ID, "mcp", "0b7e")
	hold, err := h.mcpRetentionHold(s, item, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if hold == nil {
		t.Fatal("B2 keeps the last 2, one of them came through MCP, and a second MCP backup was allowed")
	}
	if hold.detail["keepLast"] != 2 {
		t.Fatalf("detail = %v, want the keepLast of B2, the tightest destination that prunes", hold.detail)
	}
}

// The tool list is one list for every key, so the permission is the handler's
// to check on every call.
func TestMCPStartPermissionRecheckedInHandler(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t, "docs")

	res := startFileSet(mcpStartCaller("0b7e", false), h, "docs")
	if code := mcpErrorCode(t, res); code != "not_permitted" {
		t.Fatalf("code = %q, want not_permitted", code)
	}
	runs, err := repo.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("%d runs were recorded, want none", len(runs))
	}
}

// A cancel reaches a backup only under the exact progress key the service
// registered it with, so the two have to be read side by side: the derived key
// here, the literal in the service there.
func TestMCPCancelDerivesTheServiceKey(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "docs")
	target, err := repo.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	vm, err := repo.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := repo.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata"})
	if err != nil {
		t.Fatal(err)
	}
	rawService, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	rawZFSRun, err := os.ReadFile("zfs_run.go")
	if err != nil {
		t.Fatal(err)
	}
	service, zfsRun := string(rawService), string(rawZFSRun)

	for _, c := range []struct {
		domain   string
		targetID string
		want     string
		src      string
		register string
	}{
		{"containers", target.ID, "container:plex", service, `s.registerBackupCancel("container:"+name, cancel)`},
		{"vms", vm.ID, "vm:win11", service, `s.registerBackupCancel("vm:"+name, cancel)`},
		{"files", sets["docs"].ID, "files:docs", service, `s.registerBackupCancel("files:"+set.Name, cancel)`},
		{"zfs", dataset.ID, "zfs:cache/appdata", zfsRun, `key := zfsDomain + ":" + d.Dataset`},
		{"flash", store.FlashTargetID, "flash", service, `s.registerBackupCancel("flash", cancel)`},
		{"config", store.ConfigTargetID, "config", service, `s.registerBackupCancel("config", cancel)`},
	} {
		key, item, ok := h.mcpCancelKey(store.Run{TargetID: c.targetID})
		if !ok {
			t.Fatalf("%s: no progress key for target %q", c.domain, c.targetID)
		}
		if key != c.want {
			t.Fatalf("%s: key = %q, want %q", c.domain, key, c.want)
		}
		if item.Domain != c.domain || item.ID != c.targetID {
			t.Fatalf("%s: item = %+v", c.domain, item)
		}
		if !strings.Contains(c.src, c.register) {
			t.Fatalf("%s backups no longer register under %s, so a cancel of one reaches nothing", c.domain, c.register)
		}
	}

	if _, _, ok := h.mcpCancelKey(store.Run{TargetID: "gone"}); ok {
		t.Fatal("a target that is not set up any more must have no progress key")
	}
}

// The progress key belongs to the item, not to the run. Once the key's run has
// ended, the next backup under it is somebody else's, and a cancel aimed at the
// finished run must not reach it.
func TestMCPCancelReachesOnlyTheRunItNames(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "docs")
	mine, err := repo.StartRunWith(sets["docs"].ID, "backup",
		store.RunMeta{StartedVia: "mcp", StartedViaKey: "0b7e"})
	if err != nil {
		t.Fatal(err)
	}
	next, err := repo.StartRunWith(sets["docs"].ID, "backup", store.RunMeta{})
	if err != nil {
		t.Fatal(err)
	}
	cancelled := false
	h.svc.registerBackupCancel("files:docs", func() { cancelled = true })
	h.svc.bindBackupRun("files:docs", next)

	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "cancel_backup",
		Arguments: json.RawMessage(fmt.Sprintf(`{"runId":%q}`, mine)),
	}}
	res, _ := h.toolCancelBackup(mcpStartCaller("0b7e", true), req)
	if cancelled {
		t.Fatal("the cancel reached the operator's backup that runs under the same key")
	}
	if code := mcpErrorCode(t, res); code != "not_running" {
		t.Fatalf("code = %q, want not_running", code)
	}
}

// A backup can finish in the moment the cancel arrives. The answer then has to
// say that nothing was stopped.
func TestMCPCancelReportsARunThatEndedFirst(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "docs")
	runID, err := repo.StartRunWith(sets["docs"].ID, "backup",
		store.RunMeta{StartedVia: "mcp", StartedViaKey: "0b7e"})
	if err != nil {
		t.Fatal(err)
	}
	h.svc.registerBackupCancel("files:docs", func() {
		if fErr := repo.FinishRun(runID, "success", "snap1", 1, ""); fErr != nil {
			t.Error(fErr)
		}
	})
	h.svc.bindBackupRun("files:docs", runID)

	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "cancel_backup",
		Arguments: json.RawMessage(fmt.Sprintf(`{"runId":%q}`, runID)),
	}}
	res, _ := h.toolCancelBackup(mcpStartCaller("0b7e", true), req)
	if res.IsError {
		t.Fatalf("cancel_backup: %v", res.StructuredContent)
	}
	out, _ := res.StructuredContent.(map[string]any)
	if out["cancelled"] != false {
		t.Fatalf("the answer claims %v, want cancelled false", out["cancelled"])
	}
	if _, ok := out["warning"].(string); !ok {
		t.Fatalf("the answer carries no warning: %v", out)
	}
	assertCancelLoggedTooLate(t, h)
}

// A cancel that stopped nothing must not show up as done in the key's log.
func assertCancelLoggedTooLate(t *testing.T, h *Handler) {
	t.Helper()
	calls := h.mcp.toolCalls
	if n := calls[mcpToolOutcome{tool: "cancel_backup", outcome: "ok"}]; n != 0 {
		t.Fatalf("a cancel that stopped nothing was logged as ok %d times", n)
	}
	if n := calls[mcpToolOutcome{tool: "cancel_backup", outcome: "too_late"}]; n != 1 {
		t.Fatalf("too_late logged %d times, want once", n)
	}
}

// Once the restore point is written only the restart is left, and a cancel
// cannot take the backup back. The answer has to say so.
func TestMCPCancelRefusesABackupThatWroteItsRestorePoint(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t)
	target, err := repo.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := repo.StartRunWith(target.ID, "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: "0b7e"})
	if err != nil {
		t.Fatal(err)
	}
	cancelled := false
	h.svc.registerBackupCancel("container:plex", func() { cancelled = true })
	h.svc.bindBackupRun("container:plex", runID)
	h.svc.commitBackup("container:plex", 0)

	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "cancel_backup",
		Arguments: json.RawMessage(fmt.Sprintf(`{"runId":%q}`, runID)),
	}}
	res, _ := h.toolCancelBackup(mcpStartCaller("0b7e", true), req)
	if res.IsError {
		t.Fatalf("cancel_backup: %v", res.StructuredContent)
	}
	if cancelled {
		t.Fatal("the cancel reached a backup that only starts its containers again")
	}
	out, _ := res.StructuredContent.(map[string]any)
	if out["cancelled"] != false {
		t.Fatalf("the answer claims %v, want cancelled false", out["cancelled"])
	}
	if w, _ := out["warning"].(string); !strings.Contains(w, "restore point") {
		t.Fatalf("the warning does not say the restore point is written: %v", out)
	}
	assertCancelLoggedTooLate(t, h)
}

// seedBackup writes a finished backup of an item, which is the history the
// cooldown and the retention guard read. An empty origin is a backup the
// schedule or the operator made.
func seedBackup(t *testing.T, repo *store.Repo, targetID, via, keyID string) {
	t.Helper()
	id, err := repo.StartRunWith(targetID, "backup", store.RunMeta{StartedVia: via, StartedViaKey: keyID})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRun(id, "success", "", 0, ""); err != nil {
		t.Fatal(err)
	}
}

// seedFailedBackup writes an MCP backup of an item that failed. It stopped the
// container before it failed, which is why the daily budget counts it.
func seedFailedBackup(t *testing.T, repo *store.Repo, targetID, keyID string) {
	t.Helper()
	id, err := repo.StartRunWith(targetID, "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: keyID})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRun(id, "failed", "", 0, "the pre-hook did not run"); err != nil {
		t.Fatal(err)
	}
}

// mcpErrorNumber reads one number out of a refusal's extra fields.
func mcpErrorNumber(t *testing.T, res *mcp.CallToolResult, field string) float64 {
	t.Helper()
	body := mcpErrorBody(t, res)
	n, ok := body[field].(float64)
	if !ok {
		t.Fatalf("the refusal carries no %s: %v", field, body)
	}
	return n
}
