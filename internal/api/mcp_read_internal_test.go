package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// The storage card tells the operator how much room each repository has left,
// so the tool has to say the same: the local disk asked on the spot, an rclone
// remote from its last stored reading, and a backend that answers no capacity
// question as unknown rather than as zero.
func TestMCPStorageStatsReportsTheRoomAroundEachRepository(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled = true
	settings.ContainersPath = "backups/containers"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	initLocalRepo(t, filepath.Join(h.cfg.HostMountRoot, "backups", "containers"))
	h.svc.diskStat = func(string) (diskStatResult, error) {
		return diskStatResult{Volume: "dev:801", Free: 50_000_000, Used: 30_000_000, Total: 80_000_000}, nil
	}

	const remote = "rclone:gdrive:bombvault"
	for i, row := range []store.OffsiteTarget{
		{ID: "cloud", Name: "Cloud", Repo: remote, Role: store.RoleRepo, Enabled: true},
		{ID: "nas", Name: "NAS", Repo: "sftp:backup@nas:/bombvault", Role: store.RoleRepo, Enabled: true},
	} {
		if _, err := repo.UpsertOffsiteTarget(row); err != nil {
			t.Fatal(err)
		}
		name := []string{"Nextcloud", "Immich"}[i]
		if _, err := repo.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.WritePlacement(store.ItemRef{Domain: "containers", Key: name}, &store.HomeWrite{Repo: row.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	remoteTotal := int64(8_000_000_000)
	for _, sample := range []store.VolumeSample{
		{Volume: "remote:" + repoLocationKey(remote), At: now - 7200, FreeBytes: 3_000_000_000, TotalBytes: &remoteTotal, Source: "rclone"},
		{Volume: "remote:" + repoLocationKey(remote), At: now - 3600, FreeBytes: 2_000_000_000, TotalBytes: &remoteTotal, Source: "rclone"},
	} {
		if err := repo.AddVolumeSample(sample); err != nil {
			t.Fatal(err)
		}
	}

	const week = int64(7 * 86400)
	for _, sample := range []store.RepoStat{
		{At: now - 2*week, RawSize: 1_000_000},
		{At: now, RawSize: 3_000_000},
	} {
		sample.Domain, sample.Source = "containers", "local"
		if err := repo.AddRepoStat(sample); err != nil {
			t.Fatal(err)
		}
	}

	ctx := withMCPCaller(context.Background(), mcpCaller{KeyID: "0b7e", Hint: "x9Qa"})
	res, _ := h.toolGetStorageStats(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "get_storage_stats",
		Arguments: json.RawMessage(`{"domain":"containers"}`),
	}})
	out := mcpStructured(t, res)

	// Fifty million bytes free and a million more each week.
	if got := out["weeksToFull"]; got != float64(50) {
		t.Fatalf("weeksToFull = %v, want 50", got)
	}
	repos, _ := out["repositories"].([]any)
	if len(repos) != 3 {
		t.Fatalf("repositories = %v, want the domain's own and the two named ones", out["repositories"])
	}
	want := []map[string]any{
		{"name": "folder containers", "primary": true, "remote": false,
			"usedBytes": float64(30_000_000), "freeBytes": float64(50_000_000), "totalBytes": float64(80_000_000)},
		{"name": "Cloud", "primary": false, "remote": true, "at": float64(now - 3600),
			"usedBytes": float64(6_000_000_000), "freeBytes": float64(2_000_000_000), "totalBytes": float64(8_000_000_000)},
		{"name": "NAS", "primary": false, "remote": true, "at": nil,
			"usedBytes": nil, "freeBytes": nil, "totalBytes": nil},
	}
	for i, fields := range want {
		got, _ := repos[i].(map[string]any)
		for field, value := range fields {
			if got[field] != value {
				t.Fatalf("repositories[%d].%s = %v, want %v (%v)", i, field, got[field], value, got)
			}
		}
	}
	if at, _ := repos[0].(map[string]any)["at"].(float64); int64(at) < now {
		t.Fatalf("the local disk was not read on the spot: at = %v", at)
	}
	for _, r := range repos {
		for _, field := range []string{"usedBytes", "freeBytes", "totalBytes", "at"} {
			if _, ok := r.(map[string]any)[field]; !ok {
				t.Fatalf("%v leaves out %s instead of sending null", r, field)
			}
		}
	}
	if text, _ := json.Marshal(out); strings.Contains(string(text), "gdrive") || strings.Contains(string(text), "backup@nas") {
		t.Fatalf("the answer carries a repository location: %s", text)
	}
}

// A container kept straight at a target sits in that target's direct
// repository, and every container is copied to the targets, so the report
// covers both.
func TestMCPStorageStatsListsDirectRepositoriesAndTargets(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled = true
	settings.ContainersPath = "backups/containers"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	initLocalRepo(t, filepath.Join(h.cfg.HostMountRoot, "backups", "containers"))
	usb := filepath.Join(h.cfg.HostMountRoot, "usb", "containers")
	initLocalRepo(t, usb)
	h.svc.diskStat = func(path string) (diskStatResult, error) {
		if strings.Contains(path, "usb") {
			return diskStatResult{Volume: "dev:811", Free: 7_000_000, Used: 1_000_000, Total: 8_000_000}, nil
		}
		return diskStatResult{Volume: "dev:801", Free: 50_000_000, Used: 30_000_000, Total: 80_000_000}, nil
	}

	cloud, err := repo.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Cloud", Repo: "s3:s3.example.com/bkt/containers", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "USB", Repo: "usb/containers", SortOrder: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	direct, err := repo.CreateCompanionRepo(cloud.ID, "Cloud direct", "s3:s3.example.com/bkt/direct")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.WritePlacement(store.ItemRef{Domain: "containers", Key: "Immich"}, &store.HomeWrite{Repo: direct.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}

	ctx := withMCPCaller(context.Background(), mcpCaller{KeyID: "0b7e", Hint: "x9Qa"})
	res, _ := h.toolGetStorageStats(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "get_storage_stats",
		Arguments: json.RawMessage(`{"domain":"containers"}`),
	}})
	repos, _ := mcpStructured(t, res)["repositories"].([]any)
	want := []map[string]any{
		{"name": "folder containers", "primary": true, "offsite": false, "remote": false, "freeBytes": float64(50_000_000)},
		{"name": "Cloud direct", "primary": false, "offsite": false, "remote": true, "freeBytes": nil},
		{"name": "Cloud", "primary": false, "offsite": true, "remote": true, "freeBytes": nil},
		{"name": "USB", "primary": false, "offsite": true, "remote": false, "freeBytes": float64(7_000_000)},
	}
	if len(repos) != len(want) {
		t.Fatalf("repositories = %v, want the domain's own, the direct one and both targets", repos)
	}
	for i, fields := range want {
		got, _ := repos[i].(map[string]any)
		for field, value := range fields {
			if got[field] != value {
				t.Fatalf("repositories[%d].%s = %v, want %v (%v)", i, field, got[field], value, got)
			}
		}
	}
}

// initLocalRepo leaves the marker localRepoMissing looks for.
func initLocalRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ownStorageStats calls get_storage_stats for the containers domain, kept in
// backups/containers, and returns what it says about that one repository.
func ownStorageStats(t *testing.T, h *Handler) map[string]any {
	t.Helper()
	settings, err := h.svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled = true
	settings.ContainersPath = "backups/containers"
	if err := h.svc.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	ctx := withMCPCaller(context.Background(), mcpCaller{KeyID: "0b7e", Hint: "x9Qa"})
	res, _ := h.toolGetStorageStats(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "get_storage_stats",
		Arguments: json.RawMessage(`{"domain":"containers"}`),
	}})
	repos, _ := mcpStructured(t, res)["repositories"].([]any)
	if len(repos) != 1 {
		t.Fatalf("repositories = %v, want the domain's own", repos)
	}
	got, _ := repos[0].(map[string]any)
	return got
}

// A filesystem keeps blocks back for root. They are neither free to the backup
// nor filled by anything, so they do not count as used.
func TestMCPStorageStatsLeavesReservedBlocksOutOfUsed(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	initLocalRepo(t, filepath.Join(h.cfg.HostMountRoot, "backups", "containers"))
	h.svc.diskStat = func(string) (diskStatResult, error) {
		return diskStatResult{Volume: "dev:801", Free: 50_000_000, Used: 25_000_000, Total: 80_000_000}, nil
	}
	got := ownStorageStats(t, h)
	if got["usedBytes"] != float64(25_000_000) {
		t.Fatalf("usedBytes = %v, want 25000000 (%v)", got["usedBytes"], got)
	}
}

// A folder with no repository in it yet says nothing about the disk the
// repository will be on, so the figures stay unknown.
func TestMCPStorageStatsLeavesAMissingLocalRepositoryUnknown(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	probes := 0
	h.svc.diskStat = func(string) (diskStatResult, error) {
		probes++
		return diskStatResult{Volume: "dev:801", Free: 50_000_000, Used: 30_000_000, Total: 80_000_000}, nil
	}
	got := ownStorageStats(t, h)
	for _, field := range []string{"usedBytes", "freeBytes", "totalBytes", "at"} {
		if v, ok := got[field]; !ok || v != nil {
			t.Fatalf("%s = %v, want null (%v)", field, v, got)
		}
	}
	if probes != 0 {
		t.Fatalf("the disk was asked %d times about a repository that does not exist", probes)
	}
}
