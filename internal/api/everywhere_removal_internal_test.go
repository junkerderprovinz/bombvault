package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const vaultwardenRemoval = "/api/items/containers/vaultwarden/removal"

// declinedVaultwarden is a container someone stopped backing up. Its home holds
// a backup and a database dump, B2 holds a copy of the backup and one that exists
// only there, and nginx shares both repositories.
func declinedVaultwarden(t *testing.T) (*placementFixture, store.Target, store.OffsiteTarget) {
	t.Helper()
	f, b2 := vaultwardenAtB2(t)
	tg, err := f.st.GetTargetByContainer("vaultwarden")
	if err != nil {
		t.Fatal(err)
	}
	f.hold(f.domainPath("containers"),
		snap("a1", 1_758_000_000, "container:vaultwarden"),
		snap("d1", 1_758_000_000, "dbdump:vaultwarden"),
		snap("x1", 1_758_000_000, "container:nginx"),
	)
	f.backupRun(tg.ID, 1_758_000_000)
	if err := f.st.DeclineBackup(store.BackupEntry{Kind: "container", Key: "vaultwarden"}); err != nil {
		t.Fatal(err)
	}
	f.signIn()
	return f, tg, b2
}

func placesOf(t *testing.T, m map[string]any) map[string]map[string]any {
	t.Helper()
	rows, ok := m["places"].([]any)
	if !ok {
		t.Fatalf("answer = %v, want places", m)
	}
	out := map[string]map[string]any{}
	for _, row := range rows {
		p := row.(map[string]any)
		out[p["place"].(string)] = p
	}
	return out
}

func resultsOf(t *testing.T, m map[string]any) map[string]map[string]any {
	t.Helper()
	rows, ok := m["results"].([]any)
	if !ok {
		t.Fatalf("answer = %v, want results", m)
	}
	out := map[string]map[string]any{}
	for _, row := range rows {
		p := row.(map[string]any)
		out[p["place"].(string)] = p
	}
	return out
}

func TestEverywherePreviewCountsTheHomeAndEveryTarget(t *testing.T) {
	f, _, b2 := declinedVaultwarden(t)
	m := f.do("GET", vaultwardenRemoval, nil)
	if m["ok"] != true || m["name"] != "vaultwarden" {
		t.Fatalf("preview = %v", m)
	}
	places := placesOf(t, m)
	if len(places) != 2 {
		t.Fatalf("places = %v, want the home and B2", places)
	}
	if places["local"]["count"] != float64(2) {
		t.Fatalf("home = %v, want the backup and the dump", places["local"])
	}
	at := places[offsiteSourcePrefix+b2.ID]
	if at["count"] != float64(2) || at["label"] != "B2" || at["appendOnly"] != false {
		t.Fatalf("B2 = %v", at)
	}
}

func TestEverywherePreviewLeavesOutATargetWithoutCopies(t *testing.T) {
	f, _, _ := declinedVaultwarden(t)
	f.target("containers", "Hetzner", "sftp:box:containers")
	if places := placesOf(t, f.do("GET", vaultwardenRemoval, nil)); len(places) != 2 {
		t.Fatalf("places = %v, want only where vaultwarden has backups", places)
	}
}

func TestRemovingEverywhereDeletesAtEveryPlaceAndKeepsTheEntry(t *testing.T) {
	f, tg, b2 := declinedVaultwarden(t)
	m := f.do("DELETE", vaultwardenRemoval, map[string]any{
		"typedName": "vaultwarden",
		"places":    []string{"local", offsiteSourcePrefix + b2.ID},
	})
	if m["ok"] != true {
		t.Fatalf("delete = %v", m)
	}
	results := resultsOf(t, m)
	if results["local"]["deleted"] != float64(2) || results[offsiteSourcePrefix+b2.ID]["deleted"] != float64(2) {
		t.Fatalf("results = %v", results)
	}
	if got := forgottenAt(f, f.domainPath("containers")); !slices.Equal(got, []string{"a1", "d1"}) {
		t.Fatalf("forgotten at home = %v, want the backup and the dump", got)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); !slices.Equal(got, []string{"b1", "b9"}) {
		t.Fatalf("forgotten at B2 = %v", got)
	}
	if _, err := f.st.GetTargetByContainer("vaultwarden"); err != nil {
		t.Fatalf("the entry went with its backups: %v", err)
	}
	declined, err := f.st.DeclinedBackups()
	if err != nil {
		t.Fatal(err)
	}
	if !declined[store.BackupEntry{Kind: "container", Key: "vaultwarden"}] {
		t.Fatal("the entry lost its mark that it is not to be backed up")
	}
	var runs int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE target_id = ?`, tg.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("runs = %d, want the history of backups that no longer exist gone", runs)
	}
}

func TestRemovingEverywhereNeedsTheTypedName(t *testing.T) {
	f, _, b2 := declinedVaultwarden(t)
	m := f.do("DELETE", vaultwardenRemoval, map[string]any{
		"typedName": "vault",
		"places":    []string{"local", offsiteSourcePrefix + b2.ID},
	})
	if m["ok"] != false || m["code"] != "name-mismatch" {
		t.Fatalf("delete = %v, want name-mismatch", m)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("deleted without the name: %+v", f.eng.deletes)
	}
}

func TestRemovingEverywhereRefusesAPlaceTheWindowDidNotShow(t *testing.T) {
	f, _, _ := declinedVaultwarden(t)
	m := f.do("DELETE", vaultwardenRemoval, map[string]any{"typedName": "vaultwarden", "places": []string{"local"}})
	if m["ok"] != false || m["code"] != "places-changed" {
		t.Fatalf("delete = %v, want places-changed", m)
	}
	if places := placesOf(t, m["preview"].(map[string]any)); len(places) != 2 {
		t.Fatalf("fresh preview = %v, want B2 in it", places)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("deleted with a place unconfirmed: %+v", f.eng.deletes)
	}
}

func TestRemovingEverywhereLeavesAnAppendOnlyTargetAndSaysSo(t *testing.T) {
	f, _, b2 := declinedVaultwarden(t)
	f.appendOnly(b2.ID)
	if at := placesOf(t, f.do("GET", vaultwardenRemoval, nil))[offsiteSourcePrefix+b2.ID]; at["appendOnly"] != true {
		t.Fatalf("B2 = %v, want appendOnly", at)
	}
	m := f.do("DELETE", vaultwardenRemoval, map[string]any{
		"typedName": "vaultwarden",
		"places":    []string{"local", offsiteSourcePrefix + b2.ID},
	})
	if m["ok"] != true {
		t.Fatalf("delete = %v", m)
	}
	at := resultsOf(t, m)[offsiteSourcePrefix+b2.ID]
	if at["deleted"] != float64(0) || at["kept"] != "append-only" {
		t.Fatalf("B2 = %v, want it kept as append-only", at)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); len(got) != 0 {
		t.Fatalf("deleted at an append-only target: %v", got)
	}
	if got := forgottenAt(f, f.domainPath("containers")); !slices.Equal(got, []string{"a1", "d1"}) {
		t.Fatalf("forgotten at home = %v", got)
	}
}

func TestRemovingEverywhereWhileABackupRunsIsRefused(t *testing.T) {
	f, _, b2 := declinedVaultwarden(t)
	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("could not take the domain lock")
	}
	defer unlock()
	m := f.do("DELETE", vaultwardenRemoval, map[string]any{
		"typedName": "vaultwarden",
		"places":    []string{"local", offsiteSourcePrefix + b2.ID},
	})
	if m["ok"] != false || m["code"] != "domain-busy" {
		t.Fatalf("delete = %v, want domain-busy", m)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("deleted during a backup: %+v", f.eng.deletes)
	}
}

func TestRemovingAVMEverywhereTakesItsDiskImages(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	f.vm("win11", "")
	b2 := f.target("vms", "B2", "b2:bucket:vms")
	f.hold(f.domainPath("vms"),
		snap("h1", 1_758_000_000, "vm:win11", "vmrun:r1"),
		snap("h2", 1_758_000_000, "vm:win11:zvol:sda", "vmrun:r1"),
	)
	f.hold("b2:bucket:vms", copied("v1", "h1", 1_758_000_000, "vm:win11", "vmrun:r1"))
	path := "/api/items/vms/win11/removal"
	if places := placesOf(t, f.do("GET", path, nil)); places["local"]["count"] != float64(2) {
		t.Fatalf("places = %v, want the machine and its disk at home", places)
	}
	m := f.do("DELETE", path, map[string]any{"typedName": "win11", "places": []string{"local", offsiteSourcePrefix + b2.ID}})
	if m["ok"] != true {
		t.Fatalf("delete = %v", m)
	}
	if got := forgottenAt(f, f.domainPath("vms")); !slices.Equal(got, []string{"h1", "h2"}) {
		t.Fatalf("forgotten at home = %v", got)
	}
	if got := forgottenAt(f, "b2:bucket:vms"); !slices.Equal(got, []string{"v1"}) {
		t.Fatalf("forgotten at B2 = %v", got)
	}
	if _, err := f.st.GetVMTargetByName("win11"); err != nil {
		t.Fatalf("the entry went with its backups: %v", err)
	}
}

func TestRemovingAFolderEverywhereAsksForItsName(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	fs := f.fileSet("photos", "")
	b2 := f.target("files", "B2", "b2:bucket:files")
	f.hold(f.domainPath("files"), snap("p1", 1_758_000_000, "fileset:photos"))
	f.hold("b2:bucket:files", copied("q1", "p1", 1_758_000_000, "fileset:photos"))
	path := "/api/items/files/" + fs.ID + "/removal"
	if m := f.do("GET", path, nil); m["name"] != "photos" {
		t.Fatalf("preview = %v, want the folder's name to type", m)
	}
	m := f.do("DELETE", path, map[string]any{"typedName": "photos", "places": []string{"local", offsiteSourcePrefix + b2.ID}})
	if m["ok"] != true {
		t.Fatalf("delete = %v", m)
	}
	if got := append(forgottenAt(f, f.domainPath("files")), forgottenAt(f, "b2:bucket:files")...); !slices.Equal(got, []string{"p1", "q1"}) {
		t.Fatalf("forgotten = %v", got)
	}
}
