package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func locationByID(t *testing.T, svc *Service, id string) storageLocation {
	t.Helper()
	loc, err := svc.StorageLocation(id)
	if err != nil {
		t.Fatalf("location %s: %v", id, err)
	}
	return loc
}

func locationsOf(t *testing.T, svc *Service, object string) []storageLocation {
	t.Helper()
	all, err := svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	var out []storageLocation
	for _, l := range all {
		if l.Object == object {
			out = append(out, l)
		}
	}
	return out
}

func mustDestination(t *testing.T, st *store.Repo, d store.OffsiteTarget, domains ...string) (store.OffsiteTarget, []store.OffsiteTarget) {
	t.Helper()
	saved, err := st.SaveDestination(d)
	if err != nil {
		t.Fatal(err)
	}
	var derived []store.OffsiteTarget
	for _, domain := range domains {
		target, _, err := st.EnsureDestinationTarget(saved.ID, domain, destinationLocation(saved.Repo, domain))
		if err != nil {
			t.Fatal(err)
		}
		derived = append(derived, target)
	}
	return saved, derived
}

func mustNamedRepo(t *testing.T, st *store.Repo, r store.OffsiteTarget) store.OffsiteTarget {
	t.Helper()
	r.Role, r.Enabled = store.RoleRepo, true
	saved, err := st.UpsertOffsiteTarget(r)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func sectionFor(t *testing.T, loc storageLocation, domain, use string) locationSection {
	t.Helper()
	for _, sec := range loc.Sections {
		if sec.Domain == domain && sec.Use == use {
			return sec
		}
	}
	t.Fatalf("%s has no %s section for %s: %+v", loc.ID, use, domain, loc.Sections)
	return locationSection{}
}

func TestEachKindOfObjectIsOneLocation(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	repo := mustNamedRepo(t, st, store.OffsiteTarget{Name: "Cold", Repo: "backups/cold"})
	dest, _ := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rclone:box:bv", Provider: "storagebox"}, "containers")
	loose, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "By hand", Repo: "s3:https://s3.example.com/bucket/vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	all, err := svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, l := range all {
		seen[l.Object]++
	}
	for _, object := range []string{locationPath, locationRepo, locationDestination, locationTarget} {
		if seen[object] != 1 {
			t.Errorf("%d %s locations, want one: %+v", seen[object], object, all)
		}
	}

	path := locationsOf(t, svc, locationPath)[0]
	if path.Kind != "local" || path.Backend != "local" || path.Where != "user/bombvault" || len(path.Sections) != len(offsiteConfigDomains) {
		t.Errorf("domain folder = %+v, want the six domains under user/bombvault", path)
	}
	if got := locationByID(t, svc, "repo:"+repo.ID); got.Name != "Cold" || got.Kind != "local" {
		t.Errorf("named repository = %+v", got)
	}
	if got := locationByID(t, svc, "destination:"+dest.ID); got.Provider != "storagebox" || got.Backend != "rclone" || got.Kind != "offsite" {
		t.Errorf("destination = %+v", got)
	}
	if got := locationByID(t, svc, "target:"+loose.ID); got.Backend != "s3" || sectionFor(t, got, "vms", useCopy).TargetID != loose.ID {
		t.Errorf("loose target = %+v", got)
	}
}

func TestADestinationWithThreeTargetsIsOneLocation(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	dest, derived := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rest:https://box:8000/bv", Immutable: true},
		"vms", "containers", "flash")

	if got := locationsOf(t, svc, locationTarget); len(got) != 0 {
		t.Fatalf("derived targets listed on their own: %+v", got)
	}
	loc := locationByID(t, svc, "destination:"+dest.ID)
	if len(loc.Sections) != 3 {
		t.Fatalf("sections = %+v, want one per domain", loc.Sections)
	}
	for i, domain := range []string{"containers", "vms", "flash"} {
		sec := loc.Sections[i]
		if sec.Domain != domain || sec.Use != useCopy || sec.Where != "rest:https://box:8000/bv/"+domain || !sec.Immutable {
			t.Errorf("section %d = %+v, want the copy of %s", i, sec, domain)
		}
	}
	if !loc.Protection.Immutable || !loc.Protection.Testable || loc.Protection.LastTamper != nil {
		t.Errorf("protection = %+v, want append-only, testable and untested", loc.Protection)
	}

	if err := st.RecordTamperTestForTarget("vms", derived[0].ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordTamperTestForTarget("containers", derived[1].ID, false, "the server accepted a delete (HTTP 200)"); err != nil {
		t.Fatal(err)
	}
	loc = locationByID(t, svc, "destination:"+dest.ID)
	if got := sectionFor(t, loc, "containers", useCopy).LastTamper; got == nil || got.Protected || got.Detail == "" {
		t.Errorf("containers tamper test = %+v, want the failed one", got)
	}
	if got := loc.Protection.LastTamper; got == nil || got.Protected {
		t.Errorf("folded tamper test = %+v, want unprotected as soon as one section is", got)
	}
}

func TestADirectRepositoryIsListedWithItsTarget(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	target, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "b2:bucket/copies", Enabled: true, RetentionKeepDaily: 7})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := st.CreateCompanionRepo(target.ID, "", "b2:bucket/direct")
	if err != nil {
		t.Fatal(err)
	}
	if got := locationsOf(t, svc, locationRepo); len(got) != 0 {
		t.Fatalf("the direct repository is a location of its own: %+v", got)
	}
	home := sectionFor(t, locationByID(t, svc, "target:"+target.ID), "containers", useHome)
	if home.RepoID != direct.ID || home.TargetID != target.ID || home.Retention.KeepDaily != 7 {
		t.Errorf("home = %+v, want the direct repository with its target's keep-policy", home)
	}
}

func TestANamedRepositoryListsTheDomainsThatUseIt(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	repo := mustNamedRepo(t, st, store.OffsiteTarget{Name: "Cold", Repo: "backups/cold", Compression: "max"})
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", Repo: repo.ID, RepoChosen: store.RepoChosen}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutPlacementDefault("vms", repo.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.RetentionKeepDaily = 7
		s.SetOwnRetention(map[string]store.RetentionKeep{"vms": {KeepWeekly: 4}})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	loc := locationByID(t, svc, "repo:"+repo.ID)
	if len(loc.Sections) != 2 || loc.Retention.KeepDaily != 7 {
		t.Fatalf("location = %+v, want containers and vms under the shared policy", loc)
	}
	if got := sectionFor(t, loc, "containers", useHome); got.Retention.KeepDaily != 7 || got.Compression != "max" || got.RepoID != repo.ID {
		t.Errorf("containers = %+v", got)
	}
	if got := sectionFor(t, loc, "vms", useHome); got.Retention != (store.RetentionKeep{KeepWeekly: 4}) {
		t.Errorf("vms = %+v, want its own keep-policy", got)
	}
}

func TestDomainPathsAreGroupedByTheirFolder(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.VMsPath = "disk2/vms"
		s.FlashPath = "rest:https://backup:hunter2@nas:8000/tower/flash"
		s.ConfigPath = "rest:https://backup:hunter2@nas:8000/tower/config"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	paths := locationsOf(t, svc, locationPath)
	if len(paths) != 3 {
		t.Fatalf("%d domain folders, want user/bombvault, disk2 and the rest-server: %+v", len(paths), paths)
	}
	remote := paths[2]
	if remote.Kind != "offsite" || remote.Backend != "rest" || len(remote.Sections) != 2 || !remote.OffPremises {
		t.Errorf("remote folder = %+v, want flash and config off the premises", remote)
	}
	if strings.Contains(remote.Where, "hunter2") || !strings.HasSuffix(remote.Where, "nas:8000/tower") {
		t.Errorf("remote folder is at %q, want the address without its login", remote.Where)
	}
	if again := locationsOf(t, svc, locationPath); again[2].ID != remote.ID || again[0].ID == again[1].ID {
		t.Errorf("ids changed between two reads or collide: %q %q %q", again[0].ID, again[1].ID, again[2].ID)
	}
}

func TestTheFolderOfADomainPath(t *testing.T) {
	for loc, want := range map[string]string{
		"user/bombvault/container":      "user/bombvault",
		"backups/":                      "",
		"rclone:box:bv/containers":      "rclone:box:bv",
		"rclone:box:containers":         "rclone:box:",
		"s3:https://s3.host/bucket/vms": "s3:https://s3.host/bucket",
		"rest:https://nas:8000":         "rest:https://nas:8000",
		"sftp:user@host:/srv/bv/flash":  "sftp:user@host:/srv/bv",
	} {
		if got := pathRoot(loc); got != want {
			t.Errorf("pathRoot(%q) = %q, want %q", loc, got, want)
		}
	}
}

func TestALeftoverPrimaryIsNoLocation(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "Primary", Repo: "s3:old", SortOrder: 0}); err != nil {
		t.Fatal(err)
	}
	if got := locationsOf(t, svc, locationTarget); len(got) != 0 {
		t.Fatalf("the row of a cleared off-site field is listed: %+v", got)
	}
}

func TestALocalLocationIsMeasuredOnTheSpot(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	repo := filepath.Join(svc.cfg.HostMountRoot, "user", "bombvault", "container")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	svc.diskStat = func(string) (diskStatResult, error) {
		return diskStatResult{Free: 600, Used: 400, Total: 1000, Volume: "dev:1", FSType: "xfs"}, nil
	}
	now := time.Now().Unix()
	for _, sample := range []store.RepoStat{
		{Domain: "containers", Source: "local", At: now - 14*86400, RawSize: 100},
		{Domain: "containers", Source: "local", At: now, RawSize: 300},
	} {
		if err := st.AddRepoStat(sample); err != nil {
			t.Fatal(err)
		}
	}

	c := locationsOf(t, svc, locationPath)[0].Capacity
	if c.Free == nil || *c.Free != 600 || *c.Used != 400 || *c.Total != 1000 || c.Unsupported {
		t.Fatalf("capacity = %+v, want what statfs answered", c)
	}
	if c.StoredBytes == nil || *c.StoredBytes != 300 || c.GrowthBytesPerWeek == nil || *c.GrowthBytesPerWeek != 100 {
		t.Errorf("stored and growth = %v, %v, want 300 and 100 a week", c.StoredBytes, c.GrowthBytesPerWeek)
	}
	if c.WeeksToFull == nil || *c.WeeksToFull != 6 {
		t.Errorf("weeks to full = %v, want 6", c.WeeksToFull)
	}
}

func TestABackendWithoutCapacitySaysSo(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	asked := 0
	svc.rcloneAbout = func(context.Context, string) (aboutResult, error) {
		asked++
		return aboutResult{}, nil
	}
	dest, _ := mustDestination(t, st, store.OffsiteTarget{Name: "Bucket", Repo: "s3:https://s3.example.com/bucket"}, "containers")

	id := "destination:" + dest.ID
	if err := svc.RefreshLocationCapacity(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	c := locationByID(t, svc, id).Capacity
	if !c.Unsupported || c.Free != nil || c.Total != nil {
		t.Fatalf("capacity = %+v, want unsupported and no figures", c)
	}
	if asked != 0 {
		t.Errorf("an S3 bucket was asked for its room %d times", asked)
	}
}

func TestARemoteIsAskedOnlyOnRefresh(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	svc.anomalies = newAnomalyEngine(svc, time.Now)
	var asked []string
	total := int64(1000)
	svc.rcloneAbout = func(_ context.Context, remote string) (aboutResult, error) {
		asked = append(asked, remote)
		return aboutResult{Free: 250, Total: &total}, nil
	}
	dest, _ := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rclone:box:bv"}, "containers", "vms")
	id := "destination:" + dest.ID

	if c := locationByID(t, svc, id).Capacity; c.Free != nil || c.Unsupported || len(asked) != 0 {
		t.Fatalf("before a refresh: capacity %+v, asked %v, want nothing known and nobody asked", c, asked)
	}
	for range 2 {
		if err := svc.RefreshLocationCapacity(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if len(asked) != 1 || asked[0] != "box:bv" {
		t.Fatalf("asked %v, want box:bv once for two refreshes in a row", asked)
	}
	c := locationByID(t, svc, id).Capacity
	if c.Free == nil || *c.Free != 250 || *c.Used != 750 || *c.Total != 1000 || c.Source != "rclone" {
		t.Fatalf("capacity = %+v, want the remote's answer", c)
	}
	samples, err := st.ListVolumeSamples(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 0 {
		t.Errorf("the reading went into the capacity rule's samples: %+v", samples)
	}
}

func TestARefreshOfAFollowedVolumeAddsASample(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	svc.rcloneAbout = func(context.Context, string) (aboutResult, error) { return aboutResult{Free: 90}, nil }
	repo := mustNamedRepo(t, st, store.OffsiteTarget{Name: "Remote home", Repo: "rclone:box:home"})
	if err := st.AddVolumeSample(store.VolumeSample{
		Volume: remoteVolumeKey("rclone:box:home"), At: time.Now().Unix() - 3600, FreeBytes: 100, Domains: []string{"vms"}, Source: "rclone",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefreshLocationCapacity(context.Background(), "repo:"+repo.ID); err != nil {
		t.Fatal(err)
	}
	samples, err := st.ListVolumeSamples(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 || samples[1].FreeBytes != 90 || len(samples[1].Domains) != 1 {
		t.Fatalf("samples = %+v, want the new reading next to the old one, for the same domain", samples)
	}
	if c := locationByID(t, svc, "repo:"+repo.ID).Capacity; c.Free == nil || *c.Free != 90 {
		t.Errorf("capacity = %+v, want the new reading", c)
	}
}

func TestTheLastCopyOfASection(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	dest, derived := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rclone:box:bv"}, "containers", "vms")
	run := func(target store.OffsiteTarget, at int64, ok bool) {
		t.Helper()
		id, err := st.RecordOffsiteRunForTarget(target.Domain, target.ID, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.FinishOffsiteRun(id, ok, ""); err != nil {
			t.Fatal(err)
		}
	}
	run(derived[0], 1000, true)
	run(derived[0], 2000, false)
	run(derived[1], 1500, true)

	loc := locationByID(t, svc, "destination:"+dest.ID)
	if got := sectionFor(t, loc, "containers", useCopy).LastCopy; got == nil || got.At != 1000 || got.OK || got.FailingSince != 2000 {
		t.Errorf("containers = %+v, want the copy of 1000 and failing since 2000", got)
	}
	if got := sectionFor(t, loc, "vms", useCopy).LastCopy; got == nil || got.At != 1500 || !got.OK {
		t.Errorf("vms = %+v, want the copy of 1500", got)
	}
}

func TestARunWithoutATargetCountsForTheOnlyTargetOfItsDomain(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	only, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "Only", Repo: "rclone:box:flash", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.RecordOffsiteRun("flash", 1200)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
	if got := sectionFor(t, locationByID(t, svc, "target:"+only.ID), "flash", useCopy).LastCopy; got == nil || got.At != 1200 || !got.OK {
		t.Fatalf("last copy = %+v, want the run of 1200", got)
	}
	if _, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "Second", Repo: "rclone:two:flash", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if got := sectionFor(t, locationByID(t, svc, "target:"+only.ID), "flash", useCopy).LastCopy; got != nil {
		t.Fatalf("last copy = %+v, want none once two targets could have taken that run", got)
	}
}

var credentialKey = regexp.MustCompile(`(?i)pass|secret|token|key`)

// credentialKeys walks a decoded JSON value and returns every object key that
// reads like a credential field.
func credentialKeys(v any, path string) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			if credentialKey.MatchString(k) {
				out = append(out, path+"."+k)
			}
			out = append(out, credentialKeys(child, path+"."+k)...)
		}
	case []any:
		for _, child := range v {
			out = append(out, credentialKeys(child, path+"[]")...)
		}
	}
	return out
}

func TestNoLocationCarriesASecret(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	svc.cfg.DataDir = t.TempDir()
	const password, s3Secret, restPassword = "hunter2-in-the-url", "wJalrXUtnFEMI-secret", "rest-login-secret"
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.FlashPath = "rest:https://backup:" + password + "@nas:8000/tower/flash"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCloudCredSets([]CloudCredSet{{ID: "set-1", Name: "Bucket login", CloudCreds: CloudCreds{
		S3KeyID: "AKIA-key-id", S3Secret: s3Secret, RESTUser: "backup", RESTPassword: restPassword,
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRcloneConf("[box]\ntype = sftp\nhost = box.example.com\npass = " + password + "\n"); err != nil {
		t.Fatal(err)
	}
	mustNamedRepo(t, st, store.OffsiteTarget{Name: "Cold", Repo: "rest:https://cold:" + password + "@nas:8000/cold", CredsRef: "set-1"})
	dest, derived := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rest:https://box:" + password + "@box:8000/bv", CredsRef: "set-1"}, "containers")
	if _, err := st.CreateCompanionRepo(derived[0].ID, "", "rest:https://box:"+password+"@box:8000/bv/direct"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "By hand", Repo: "s3:https://key:" + password + "@s3.example.com/bucket", CredsRef: "set-1", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("flash", store.OffsiteTarget{Repo: "rest:https://backup:" + password + "@nas:8000/tower/flash", CredsRef: "set-1", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	h := &Handler{store: st, svc: svc}
	bodies := map[string]string{}
	rec := httptest.NewRecorder()
	h.handleListStorageLocations(rec, jsonReq(http.MethodGet, "/api/storage/locations", nil))
	bodies["list"] = rec.Body.String()
	req := jsonReq(http.MethodGet, "/api/storage/locations/destination:"+dest.ID, nil)
	req.SetPathValue("id", "destination:"+dest.ID)
	rec = httptest.NewRecorder()
	h.handleGetStorageLocation(rec, req)
	bodies["one"] = rec.Body.String()

	for name, body := range bodies {
		for _, secret := range []string{password, s3Secret, restPassword, "AKIA-key-id"} {
			if strings.Contains(body, secret) {
				t.Errorf("the %s answer carries %q: %s", name, secret, body)
			}
		}
		var decoded any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatal(err)
		}
		if keys := credentialKeys(decoded, ""); len(keys) != 0 {
			t.Errorf("the %s answer has fields that read like credentials: %v", name, keys)
		}
		if !strings.Contains(body, `"credsRef":"set-1"`) {
			t.Errorf("the %s answer does not name the credential set: %s", name, body)
		}
	}
}

func TestAnUnknownLocationIsNotFound(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	h := &Handler{store: st, svc: svc}
	req := jsonReq(http.MethodGet, "/api/storage/locations/destination:nope?refresh=capacity", nil)
	req.SetPathValue("id", "destination:nope")
	rec := httptest.NewRecorder()
	h.handleGetStorageLocation(rec, req)
	if rec.Code != http.StatusNotFound || decodeEnvelope(t, rec)["ok"] != false {
		t.Fatalf("answer = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAFailedProbeKeepsTheLocationAndNamesTheError(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	svc.rcloneAbout = func(context.Context, string) (aboutResult, error) {
		return aboutResult{}, context.DeadlineExceeded
	}
	dest, _ := mustDestination(t, st, store.OffsiteTarget{Name: "Box", Repo: "rclone:box:bv"}, "containers")
	h := &Handler{store: st, svc: svc}
	req := jsonReq(http.MethodGet, "/api/storage/locations/destination:"+dest.ID+"?refresh=capacity", nil)
	req.SetPathValue("id", "destination:"+dest.ID)
	rec := httptest.NewRecorder()
	h.handleGetStorageLocation(rec, req)
	env := decodeEnvelope(t, rec)
	if env["ok"] != true || env["capacityError"] == nil || env["location"] == nil {
		t.Fatalf("answer = %v, want the location and the probe's error", env)
	}
}
