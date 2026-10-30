package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// walkPlaces are the remote places a walk puts storage locations at. Each one
// holds old backups of a section before the walk starts, so adding a location
// there again takes them up.
var walkPlaces = []string{"rclone:box:bv", "rclone:vault:bv", "rclone:mani:bv", "rest:https://nas:8000/bv"}

// locationWalk drives a fixture through random steps the way the storage
// pages do and checks the storage location model after each one.
type locationWalk struct {
	t     *testing.T
	f     *placementFixture
	rng   *rand.Rand
	steps []string
	n     int
}

func newLocationWalk(t *testing.T, seed uint64) *locationWalk {
	w := &locationWalk{t: t, rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	w.f = w.freshFixture()
	for i, place := range walkPlaces {
		domain := store.PlacementDomains[i%len(store.PlacementDomains)]
		w.f.hold(destinationLocation(place, domain), snap(fmt.Sprintf("old%d", i), 100, itemIdentityPrefix(domain)+"old"))
	}
	return w
}

// freshFixture is a fixture with the entries every walk works on.
func (w *locationWalk) freshFixture() *placementFixture {
	f := newPlacementFixture(w.t)
	for _, item := range []store.ItemRef{{Domain: "containers", Key: "web"}, {Domain: "containers", Key: "db"}, {Domain: "vms", Key: "win"}} {
		if _, err := f.st.WritePlacement(item, &store.HomeWrite{Choice: store.RepoOpen}, nil, nil); err != nil {
			w.t.Fatal(err)
		}
	}
	return f
}

func (w *locationWalk) pick(n int) int { return w.rng.IntN(n) }

func (w *locationWalk) domain() string {
	return store.PlacementDomains[w.pick(len(store.PlacementDomains))]
}

// call sends a request the way the storage pages do. A refusal is an answer
// like any other; only a server error stops the walk.
func (w *locationWalk) call(method, path string, body any) map[string]any {
	w.t.Helper()
	code, out := w.f.call(method, path, body)
	answer := "ok"
	if out["ok"] == false {
		answer = fmt.Sprintf("refused: %v", out["error"])
	}
	w.steps = append(w.steps, fmt.Sprintf("%s %s %s -> %s", method, path, describe(body), answer))
	if code >= http.StatusInternalServerError {
		w.fail(fmt.Sprintf("%s %s = %d: %v", method, path, code, out))
	}
	return out
}

func (w *locationWalk) note(step string) { w.steps = append(w.steps, step) }

func describe(body any) string {
	if body == nil {
		return ""
	}
	b, _ := json.Marshal(body)
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
}

func (w *locationWalk) fail(why string) {
	w.t.Helper()
	w.t.Fatalf("%s\nsteps:\n  %s", why, strings.Join(w.steps, "\n  "))
}

func (w *locationWalk) targets(domain string) []store.OffsiteTarget {
	out, err := w.f.st.OffsiteTargetsForDomain(domain)
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *locationWalk) allTargets() []store.OffsiteTarget {
	out, err := w.f.st.ListOffsiteTargets()
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *locationWalk) namedRepos() []store.OffsiteTarget {
	out, err := w.f.st.ListNamedRepos()
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *locationWalk) destinations() []store.OffsiteTarget {
	out, err := w.f.st.ListDestinations()
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *locationWalk) place() string { return walkPlaces[w.pick(len(walkPlaces))] }

// location is a random place for a repository or a copy of the section: the
// folder a storage location keeps for it, a folder of its own there, or a
// folder on this host, where the walk also moves own folders to.
func (w *locationWalk) location(domain string) string {
	switch w.pick(5) {
	case 0:
		return destinationLocation(w.place(), domain)
	case 1:
		return w.place() + "/own" + strconv.Itoa(w.n) + "-" + domain
	case 2:
		return "backups/" + domain
	case 3:
		return "moved/" + domain
	}
	return "elsewhere/deeper/" + domain
}

// skipList is a random answer to which copies a section leaves out.
func (w *locationWalk) skipList(domain string) []string {
	if w.pick(5) == 0 {
		return []string{store.SkipAll}
	}
	out := []string{}
	for _, t := range w.targets(domain) {
		if w.pick(2) == 0 {
			out = append(out, t.ID)
		}
	}
	return out
}

func (w *locationWalk) step() {
	switch w.pick(18) {
	case 0:
		w.addDestination()
	case 1, 2:
		if ds := w.destinations(); len(ds) > 0 {
			d := ds[w.pick(len(ds))]
			w.call("POST", "/api/offsite/destinations/"+d.ID+"/domains/"+w.domain(), nil)
		}
	case 3:
		domain := w.domain()
		w.call("POST", "/api/offsite/targets", map[string]any{
			"domain": domain, "name": "copy" + strconv.Itoa(w.n), "repo": w.location(domain), "enabled": true,
		})
	case 4:
		w.call("POST", "/api/repos", map[string]any{"name": "repo" + strconv.Itoa(w.n), "repo": w.location(w.domain())})
	case 5:
		if ts := w.allTargets(); len(ts) > 0 {
			t := ts[w.pick(len(ts))]
			w.call("POST", "/api/repos", map[string]any{"companionOf": t.ID, "name": "direct" + strconv.Itoa(w.n), "repo": directLocationFor(t).Location})
		}
	case 6, 7:
		w.removeLocation()
	case 8:
		w.moveOwnFolder()
	case 9, 10:
		w.setDefault()
	case 11:
		domain := []string{"flash", "config"}[w.pick(2)]
		w.call("PUT", "/api/storage/sections/"+domain+"/local", map[string]any{"on": w.pick(2) == 0})
	case 12:
		w.placeItem()
	case 13:
		w.removeSingle()
	case 14:
		if ds := w.destinations(); len(ds) > 0 {
			d := ds[w.pick(len(ds))]
			w.call("POST", "/api/offsite/destinations/"+d.ID+"/primary/"+w.domain(), nil)
		}
	case 15:
		w.editSettings()
	case 16:
		if w.pick(2) == 0 {
			w.exportImport()
		} else {
			w.restart()
		}
	case 17:
		w.moveCopy()
	}
}

// moveCopy moves a copy of a section into a storage location and waits for
// the move to end.
func (w *locationWalk) moveCopy() {
	ts, ds := w.allTargets(), w.destinations()
	if len(ts) == 0 || len(ds) == 0 {
		return
	}
	t := ts[w.pick(len(ts))]
	w.call("POST", "/api/offsite/targets/"+t.ID+"/move", map[string]any{"to": ds[w.pick(len(ds))].ID})
	w.f.waitRun("move:" + t.ID)
}

// editSettings saves the settings page with a section's own folder or its
// off-site field pointed somewhere else.
func (w *locationWalk) editSettings() {
	settings, err := w.f.st.GetSettings()
	if err != nil {
		w.t.Fatal(err)
	}
	domain := w.domain()
	loc := w.location(domain)
	if w.pick(3) == 0 {
		loc = ""
	}
	if w.pick(2) == 0 && loc != "" {
		setDomainPath(&settings, domain, loc)
	} else {
		setOffsiteRepoInSettings(&settings, domain, loc)
	}
	w.call("PUT", "/api/settings", toView(settings))
}

// addDestination saves a destination at a place the way the wizard does and
// takes up the backups already there.
func (w *locationWalk) addDestination() {
	place := w.place()
	w.note("add destination " + place)
	if err := w.f.svc.destinationClash(place); err != nil {
		return
	}
	d, err := w.f.st.SaveDestination(store.OffsiteTarget{Name: "dest" + strconv.Itoa(w.n), Repo: place, Provider: "box"})
	if err != nil {
		w.t.Fatal(err)
	}
	w.f.svc.takeUpOldBackups(context.Background(), d)
}

func (w *locationWalk) removeLocation() {
	locations, err := w.f.svc.StorageLocations()
	if err != nil {
		w.fail(err.Error())
	}
	if len(locations) == 0 {
		return
	}
	id := locations[w.pick(len(locations))].ID
	preview := w.call("GET", "/api/storage/locations/"+id+"/removal", nil)
	if preview["ok"] != true {
		return
	}
	home := []string{"backups", "moved", "elsewhere/deeper"}[w.pick(3)]
	w.call("POST", "/api/storage/locations/"+id+"/removal", map[string]any{"expect": preview["removal"], "home": home})
}

// removeSingle deletes a copy or a repository from its own list rather than
// from the storage page.
func (w *locationWalk) removeSingle() {
	if w.pick(2) == 0 {
		if ts := w.allTargets(); len(ts) > 0 {
			w.call("DELETE", "/api/offsite/targets/"+ts[w.pick(len(ts))].ID, nil)
		}
		return
	}
	if rs := w.namedRepos(); len(rs) > 0 {
		w.call("DELETE", "/api/repos/"+rs[w.pick(len(rs))].ID, nil)
	}
}

func (w *locationWalk) moveOwnFolder() {
	domains := []string{w.domain()}
	if w.pick(3) == 0 {
		domains = append(domains, "flash")
	}
	body := map[string]any{"domains": domains}
	locations, err := w.f.svc.StorageLocations()
	if err != nil {
		w.fail(err.Error())
	}
	var ids []string
	for _, l := range locations {
		if l.Object == locationDestination || l.Object == locationPath {
			ids = append(ids, l.ID)
		}
	}
	if len(ids) > 0 && w.pick(4) > 0 {
		body["location"] = ids[w.pick(len(ids))]
	} else {
		body["address"] = []string{"backups", "moved", w.place()}[w.pick(3)]
	}
	w.call("POST", "/api/storage/own-folders", body)
}

// setDefault saves a section's default: its home is the own folder, nowhere,
// a named repository or a copy's direct repository, which switches the own
// folder off.
func (w *locationWalk) setDefault() {
	domain := w.domain()
	body := map[string]any{}
	if w.pick(3) > 0 {
		homes := []string{"", "", store.HomeNowhere}
		for _, r := range w.namedRepos() {
			homes = append(homes, r.ID)
		}
		body["home"] = homes[w.pick(len(homes))]
	}
	if w.pick(3) > 0 {
		body["skip"] = w.skipList(domain)
	}
	preview := w.call("POST", "/api/placement/default/"+domain+"/preview", body)
	if preview["ok"] != true {
		return
	}
	body["expect"] = preview["impact"]
	w.call("PUT", "/api/placement/default/"+domain, body)
}

func (w *locationWalk) placeItem() {
	name := []string{"web", "db"}[w.pick(2)]
	var body map[string]any
	switch w.pick(4) {
	case 0:
		body = map[string]any{"copies": map[string]any{"skip": w.skipList("containers")}}
	case 1:
		body = map[string]any{"copies": map[string]any{"follow": true}}
	case 2:
		repos := []string{""}
		for _, r := range w.namedRepos() {
			repos = append(repos, r.ID)
		}
		body = map[string]any{"home": map[string]any{"repo": repos[w.pick(len(repos))]}}
	default:
		body = map[string]any{"home": map[string]any{"follow": true}}
	}
	w.call("PATCH", "/api/containers/"+name, body)
}

// exportImport carries the settings over to a fresh install, which the walk
// goes on with.
func (w *locationWalk) exportImport() {
	exp := w.call("GET", "/api/settings/export", nil)
	if exp["ok"] == false {
		w.fail(fmt.Sprintf("export = %v", exp))
	}
	next := w.freshFixture()
	next.eng.snaps = w.f.eng.snaps
	w.f = next
	if res := w.call("POST", "/api/settings/import?apply=true", exp); res["ok"] != true {
		w.fail(fmt.Sprintf("importing an export of this instance = %v", res))
	}
}

// restart runs what a start runs on the database before anything else.
func (w *locationWalk) restart() {
	w.note("restart")
	if err := store.Migrate(w.f.db); err != nil {
		w.fail(err.Error())
	}
	if _, err := w.f.st.RepairSectionPaths(); err != nil {
		w.fail(err.Error())
	}
	fixed, err := w.f.svc.RepairLocationModel()
	if err != nil {
		w.fail(err.Error())
	}
	if len(fixed) > 0 {
		w.fail(fmt.Sprintf("a start repaired a model the checks passed: %v", fixed))
	}
	if _, err := w.f.svc.MoveTargetsOffPrimarySlot(); err != nil {
		w.fail(err.Error())
	}
}

func (w *locationWalk) check() {
	w.t.Helper()
	faults, err := w.f.svc.locationFaults()
	if err != nil {
		w.fail(err.Error())
	}
	if len(faults) == 0 {
		return
	}
	var problems []string
	for _, f := range faults {
		problems = append(problems, f.problem)
	}
	w.fail("the storage location model is broken:\n  " + strings.Join(problems, "\n  "))
}

// walkSeeds are the seeds a run walks: BV_LOCATION_SEED replays one, and
// BV_LOCATION_SEEDS asks for more than the usual few hundred. BV_LOCATION_TRACE
// logs the steps of walks that pass as well.
func walkSeeds(t *testing.T) []uint64 {
	if one := os.Getenv("BV_LOCATION_SEED"); one != "" {
		seed, err := strconv.ParseUint(one, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return []uint64{seed}
	}
	n := 300
	if testing.Short() {
		n = 100
	}
	if more := os.Getenv("BV_LOCATION_SEEDS"); more != "" {
		var err error
		if n, err = strconv.Atoi(more); err != nil {
			t.Fatal(err)
		}
	}
	seeds := make([]uint64, n)
	for i := range seeds {
		seeds[i] = uint64(i + 1)
	}
	return seeds
}

func TestRandomStorageStepsKeepTheLocationModelSound(t *testing.T) {
	for _, seed := range walkSeeds(t) {
		t.Run("seed"+strconv.FormatUint(seed, 10), func(t *testing.T) {
			w := newLocationWalk(t, seed)
			for w.n = 0; w.n < 30; w.n++ {
				w.step()
				w.check()
			}
			if os.Getenv("BV_LOCATION_TRACE") != "" {
				t.Log(strings.Join(w.steps, "\n"))
			}
		})
	}
}

func TestAFreshInstallHasASoundLocationModel(t *testing.T) {
	f := newPlacementFixture(t)
	faults, err := f.svc.locationFaults()
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) != 0 {
		t.Fatalf("faults = %+v", faults)
	}
}

// breakLocationModel writes random broken rows straight into the database,
// the kind older releases left behind.
func breakLocationModel(t *testing.T, f *placementFixture, rng *rand.Rand) []string {
	t.Helper()
	var done []string
	exec := func(what, q string, args ...any) {
		if _, err := f.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		done = append(done, what)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	box := f.target("containers", "Box", "rclone:box:bv/containers")
	f.direct(box)
	for i := range 3 + rng.IntN(6) {
		gone := fmt.Sprintf("%032x", rng.Uint64())
		domain := store.PlacementDomains[rng.IntN(len(store.PlacementDomains))]
		switch rng.IntN(8) {
		case 0:
			exec("a default skips a gone copy", `INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES (?, '', ?, 1, 1)
				ON CONFLICT(domain) DO UPDATE SET skip = excluded.skip`, domain, `["`+gone+`"]`)
		case 1:
			exec("a copy rule skips a gone copy", `INSERT OR REPLACE INTO offsite_copy_rules (domain, identity, skip, updated_at) VALUES ('containers', ?, ?, 1)`,
				fmt.Sprintf("container:c%d", i), `["`+box.ID+`","`+gone+`"]`)
		case 2:
			exec("a default skips a copy of another section", `INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES ('vms', '', ?, 1, 1)
				ON CONFLICT(domain) DO UPDATE SET skip = excluded.skip`, `["`+box.ID+`"]`)
		case 3:
			exec("a default backs up to a gone repository", `INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES (?, ?, '[]', 1, 1)
				ON CONFLICT(domain) DO UPDATE SET home = excluded.home`, domain, gone)
		case 4:
			f.container(fmt.Sprintf("c%d", i), "")
			exec("an entry backs up to a gone repository", `UPDATE targets SET repo = ?, repo_chosen = 1 WHERE container_name = ?`, gone, fmt.Sprintf("c%d", i))
		case 5:
			exec("a former home is gone", `INSERT OR IGNORE INTO former_homes (domain, item_id, repo, left_at) VALUES ('containers', ?, ?, 1)`, gone, gone)
		case 6:
			exec("a direct repository outlives its copy", `INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order, companion_of)
				VALUES (?, '', 'stray', ?, 'repo', 1, 1, 9, ?)`, gone, "rclone:stray:"+gone, fmt.Sprintf("%032x", rng.Uint64()))
		default:
			own, err := f.svc.resolveRepo(domainPathRaw(domain, settings))
			if err != nil {
				t.Fatal(err)
			}
			if rng.IntN(2) == 0 {
				exec("a copy sits on its own folder", `INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order)
					VALUES (?, ?, 'self', ?, 'offsite', 1, 1, 7)`, gone, domain, own+"/")
				exec("a default skips the copy on the own folder", `INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES (?, '', ?, 1, 1)
					ON CONFLICT(domain) DO UPDATE SET skip = excluded.skip`, domain, `["`+gone+`"]`)
			} else {
				settings, err := f.st.GetSettings()
				if err != nil {
					t.Fatal(err)
				}
				setOffsiteRepoInSettings(&settings, domain, own)
				if err := f.st.UpdateSettings(settings); err != nil {
					t.Fatal(err)
				}
				done = append(done, "an off-site field names the own folder")
			}
		}
	}
	return done
}

func TestOneStartupRepairLeavesNoFaultAndASecondChangesNothing(t *testing.T) {
	for seed := range uint64(40) {
		t.Run("seed"+strconv.FormatUint(seed, 10), func(t *testing.T) {
			f := newPlacementFixture(t)
			broken := breakLocationModel(t, f, rand.New(rand.NewPCG(seed, seed+1)))

			fixed, err := f.svc.RepairLocationModel()
			if err != nil {
				t.Fatalf("repair after %v: %v", broken, err)
			}
			faults, err := f.svc.locationFaults()
			if err != nil {
				t.Fatal(err)
			}
			if len(faults) != 0 {
				t.Fatalf("after %v the repair %v left %+v", broken, fixed, faults)
			}
			if again, err := f.svc.RepairLocationModel(); err != nil || len(again) != 0 {
				t.Fatalf("a second repair = %v, %v, want nothing", again, err)
			}
		})
	}
}

func TestTheStartupRepairLeavesAHomeOfAnotherSectionAlone(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.target("vms", "Box", "rclone:box:bv/vms")
	direct := f.direct(box)
	f.setDefault("containers", direct.ID)

	fixed, err := f.svc.RepairLocationModel()
	if err != nil || len(fixed) != 0 {
		t.Fatalf("repair = %v, %v, want no fix", fixed, err)
	}
	faults, err := f.svc.locationFaults()
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) != 1 || faults[0].repair != nil || !strings.Contains(faults[0].problem, "of a vms copy") {
		t.Fatalf("faults = %+v, want the foreign home named and left", faults)
	}
	if d, _, _ := f.st.PlacementDefaultFor("containers"); d.Home != direct.ID {
		t.Fatalf("home = %q, want it kept", d.Home)
	}
}

func TestTheStartupRepairSendsWhatNamesAGoneRepositoryHome(t *testing.T) {
	f := newPlacementFixture(t)
	web := f.container("web", "")
	gone := strings.Repeat("e", 32)
	if _, err := f.db.Exec(`UPDATE targets SET repo = ? WHERE id = ?`, gone, web.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES ('containers', ?, '[]', 1, 1)`, gone); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.RepairLocationModel(); err != nil {
		t.Fatal(err)
	}

	home, err := f.st.ItemHome(store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || home.Repo != "" || home.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, %v, want it following its section", home, err)
	}
	if d, _, _ := f.st.PlacementDefaultFor("containers"); d.Home != "" {
		t.Fatalf("default home = %q, want the own folder", d.Home)
	}
	if again, err := f.svc.RepairLocationModel(); err != nil || len(again) != 0 {
		t.Fatalf("a second repair = %v, %v, want nothing", again, err)
	}
}
