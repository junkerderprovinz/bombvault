package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newMCPRestorePointHandler wires a handler whose flash repository exists and
// whose every listing blocks in eng.
func newMCPRestorePointHandler(t *testing.T) (*Handler, *blockingSnapshotsEngine) {
	t.Helper()
	h, _, repo, _ := newMCPGateHandler(t)
	eng := newBlockingSnapshotsEngine()
	h.svc = NewService(h.cfg, repo, nil, nil, eng)

	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashEnabled = true
	settings.FlashPath = "backups/flash"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(h.cfg.HostMountRoot, "backups", "flash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return h, eng
}

// listFlashRestorePoints calls the tool the way the transport would, on the
// context the transport would hand it.
func listFlashRestorePoints(ctx context.Context, h *Handler) *mcp.CallToolResult {
	ctx = withMCPCaller(ctx, mcpCaller{KeyID: "0b7e", Hint: "x9Qa", CanStartBackups: true})
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "list_restore_points",
		Arguments: json.RawMessage(`{"domain":"flash"}`),
	}}
	res, _ := h.toolListRestorePoints(ctx, req)
	return res
}

// A legacy-era handler context is detached from its client, so restic would
// keep reading a remote repository long after the assistant gave up.
func TestMCPRestorePointsTimeoutCancelsRestic(t *testing.T) {
	h, eng := newMCPRestorePointHandler(t)
	mcpResticTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mcpResticTimeout = time.Minute })

	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- listFlashRestorePoints(context.Background(), h) }()

	select {
	case res := <-done:
		if got := mcpErrorCode(t, res); got != "timeout" {
			t.Fatalf("code = %q, want timeout", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the listing is still running a second after its deadline")
	}
	if !eng.sawCancellation() {
		t.Fatal("restic was left running after the tool gave up")
	}
}

func TestMCPRestorePointsSemaphoreBusy(t *testing.T) {
	h, eng := newMCPRestorePointHandler(t)

	first := make(chan *mcp.CallToolResult, 1)
	go func() { first <- listFlashRestorePoints(context.Background(), h) }()
	select {
	case <-eng.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first listing never reached the engine")
	}

	busy := listFlashRestorePoints(context.Background(), h)
	if got := mcpErrorCode(t, busy); got != "busy" {
		t.Fatalf("a second listing gives %q, want busy", got)
	}

	close(eng.release)
	select {
	case res := <-first:
		if res.IsError {
			t.Fatalf("the released listing failed: %v", res.StructuredContent)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the released listing never finished")
	}

	if third := listFlashRestorePoints(context.Background(), h); third.IsError {
		t.Fatalf("the slot was never given back: %v", third.StructuredContent)
	}
	if got := eng.callCount(); got != 2 {
		t.Fatalf("restic ran %d times, want the two listings that held the slot", got)
	}
}

// runningDocker reports the named containers as up and answers nothing else,
// which is all list_items asks Docker for.
type runningDocker struct {
	dockercli.Docker
	running []string
}

func (d runningDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	out := make([]dockercli.ContainerInfo, 0, len(d.running))
	for _, name := range d.running {
		out = append(out, dockercli.ContainerInfo{Name: name, State: "running"})
	}
	return out, nil
}

// mcpStructured is a tool result's structured content the way a client reads
// it, as decoded JSON.
func mcpStructured(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	if res.IsError {
		t.Fatalf("the call was refused: %v", res.StructuredContent)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

// ZFS items are listed from the store. The host may be switched off, and a
// listing that opened SSH for every call would be the most expensive read an
// assistant can repeat.
func TestMCPListItemsZFSDatasetsWithoutAskingTheHost(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	host := &fakeZFSHost{}
	h.svc.zfs = host
	h.docker = runningDocker{running: []string{"plex"}}
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ZFSEnabled = true
	settings.ZFSSchedule = "daily 03:00"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	plex, err := repo.CreateZFSDataset(store.ZFSDataset{
		Dataset: "cache/appdata/plex", Enabled: true, StopContainers: []string{"plex", "redis"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetZFSCheck(plex.ID, "not-mounted", "", "", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/archive"}); err != nil {
		t.Fatal(err)
	}
	seedBackup(t, repo, plex.ID, "", "")
	plex, err = repo.GetZFSDataset(plex.ID)
	if err != nil {
		t.Fatal(err)
	}

	res, _ := h.toolListItems(mcpStartCaller("0b7e", false), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name: "list_items", Arguments: json.RawMessage(`{"domain":"zfs"}`),
	}})
	domains, _ := mcpStructured(t, res)["domains"].([]any)
	if len(domains) != 1 {
		t.Fatalf("list_items for zfs returned %v", domains)
	}
	row, _ := domains[0].(map[string]any)
	if row["domain"] != "zfs" || row["enabled"] != true {
		t.Fatalf("the zfs domain reads %v", row)
	}
	items := map[string]map[string]any{}
	for _, raw := range row["items"].([]any) {
		item, _ := raw.(map[string]any)
		name, _ := item["name"].(string)
		items[name] = item
	}

	got := items["cache/appdata/plex"]
	if got["id"] != plex.ID || got["included"] != true || got["lastCheckCode"] != "not-mounted" {
		t.Fatalf("the dataset reads %v", got)
	}
	if want := schedule.EffectiveZFSDatasetSchedule(plex, settings).Kind; got["schedule"] != want {
		t.Fatalf("schedule = %v, want %q", got["schedule"], want)
	}
	stops, _ := got["stops"].(map[string]any)
	also, _ := stops["containers"].([]any)
	if stops["self"] != false || stops["known"] != true || len(also) != 1 || also[0] != "plex" {
		t.Fatalf("stops = %v, want the one configured container that is running", stops)
	}
	if got["lastSuccessAt"] == float64(0) {
		t.Fatalf("the seeded backup is not stamped on the dataset: %v", got)
	}
	if archive := items["cache/archive"]; archive["included"] != false || archive["lastCheckCode"] != "" {
		t.Fatalf("the switched-off dataset reads %v", archive)
	}
	if calls := host.recorded(); len(calls) != 0 {
		t.Fatalf("listing the items asked the host %v", calls)
	}
}

// get_activity names the run behind a ZFS backup the same way it does for the
// other domains, so an assistant can pass it to cancel_backup.
func TestMCPActivityNamesTheRunOfAZFSBackup(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	d, err := repo.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := repo.StartRunWith(d.ID, "backup", store.RunMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if got := h.runningRunID(ActivityItem{Domain: zfsDomain, Item: d.Dataset}); got != runID {
		t.Fatalf("runId = %q, want the running backup %q", got, runID)
	}
}

// A client that hangs up mid-call cancels the tool context. Reporting that as a
// slow repository sends an operator after a fault that is not there and drops
// what restic really said on the way.
func TestMCPRestorePointsCancelledClientIsNotATimeout(t *testing.T) {
	h, eng := newMCPRestorePointHandler(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- listFlashRestorePoints(ctx, h) }()
	select {
	case <-eng.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the listing never reached the engine")
	}
	cancel()

	select {
	case res := <-done:
		if got := mcpErrorCode(t, res); got != "failed" {
			t.Fatalf("code = %q, want failed", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled listing never came back")
	}
}
