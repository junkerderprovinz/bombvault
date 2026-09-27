package api

import (
	"maps"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnAddressSplitsIntoItsBaseAndLastFolder(t *testing.T) {
	for loc, want := range map[string]placeSplit{
		"backups/containers":                                      {places.KindLocal, "backups", "containers"},
		"remotes/nas/bombvault/vms":                               {places.KindLocal, "remotes/nas/bombvault", "vms"},
		"s3:https://s3.us-west-004.backblazeb2.com/bv/containers": {places.KindS3, "s3:https://s3.us-west-004.backblazeb2.com/bv", "containers"},
		"s3:https://minio.lan:9000/bv/prefix/containers":          {places.KindS3, "s3:https://minio.lan:9000/bv/prefix", "containers"},
		"s3:s3.amazonaws.com/bv/containers":                       {places.KindS3, "s3:s3.amazonaws.com/bv", "containers"},
		"rest:http://tower:8000/bombvault-containers/containers":  {places.KindREST, "rest:http://tower:8000/bombvault-containers", "containers"},
		"rest:http://tower:8000/containers":                       {places.KindREST, "rest:http://tower:8000", "containers"},
		"sftp:pi:flash":                                           {places.KindSFTP, "sftp:pi:", "flash"},
		"sftp:bv@nas:/srv/restic/flash":                           {places.KindSFTP, "sftp:bv@nas:/srv/restic", "flash"},
		"sftp://bv@nas:2222/srv/flash":                            {places.KindSFTP, "sftp://bv@nas:2222/srv", "flash"},
		"rclone:r:config":                                         {places.KindRclone, "rclone:r:", "config"},
		"rclone:r:bucket/config":                                  {places.KindRclone, "rclone:r:bucket", "config"},
	} {
		got, ok := splitPlaceAddress(loc)
		if !ok || got != want {
			t.Errorf("splitPlaceAddress(%q) = %+v, %v, want %+v", loc, got, ok, want)
		}
	}
}

func TestTheRootOfABucketOrServerIsARepositoryOfItsOwn(t *testing.T) {
	for _, loc := range []string{
		"s3:https://s3.example.com/bucket", "s3:https://s3.example.com/bucket/",
		"rest:http://tower:8000", "rest:http://tower:8000/", "sftp:pi:", "rclone:r:",
	} {
		got, ok := splitPlaceAddress(loc)
		if !ok || got.base != loc || got.folder != "" {
			t.Errorf("splitPlaceAddress(%q) = %+v, %v, want the address itself as base", loc, got, ok)
		}
	}
}

func TestAnAddressNoPlaceCanSpellStaysUnsplit(t *testing.T) {
	for _, loc := range []string{
		"b2:bucket:files", "gs:bucket:/files", "swift:container:/files", "azure:container:/files",
		"BackBlaze:bucket/files", "", "backups", "/backups", "backups/containers/",
		"s3:https://s3.example.com/bucket/containers/", "s3:https://s3.example.com/", "sftp:pi:/flash", "sftp:pi",
	} {
		if got, ok := splitPlaceAddress(loc); ok {
			t.Errorf("splitPlaceAddress(%q) = %+v, want no split", loc, got)
		}
	}
}

// migrationInput is a plan input over the default settings: every domain path
// local under user/bombvault and the global local rule keep-last 5.
func migrationInput() placesMigrationInput {
	return placesMigrationInput{
		settings: store.Settings{
			ContainersPath: "user/bombvault/container", VMsPath: "user/bombvault/vms", FlashPath: "user/bombvault/flash",
			ConfigPath: "user/bombvault/config", FilesPath: "user/bombvault/files", RetentionKeepLast: 5,
		},
		primaries: map[string]store.OffsiteTarget{},
		credSets:  map[string]CloudCredSet{},
		homes:     map[string]string{},
	}
}

func placeNames(plan placesPlan) []string {
	var out []string
	for _, m := range plan.places {
		out = append(out, m.Place.Name)
	}
	return out
}

// placeNamed returns the planned place with that name.
func placeNamed(t *testing.T, plan placesPlan, name string) store.MigratedPlace {
	t.Helper()
	for _, m := range plan.places {
		if m.Place.Name == name {
			return m
		}
	}
	t.Fatalf("no place %q among %q", name, placeNames(plan))
	return store.MigratedPlace{}
}

func TestLocalDomainPathsWithOneParentShareAPlace(t *testing.T) {
	in := migrationInput()
	in.settings.ContainersPath, in.settings.VMsPath, in.settings.FilesPath = "backups/containers", "backups/vms", "backups/files"
	plan := planPlaces(in)
	unraid := placeNamed(t, plan, "Unraid")
	if unraid.Place.Base != "backups" ||
		!maps.Equal(unraid.Place.Folders, map[string]string{"containers": "containers", "vms": "vms", "files": "files"}) ||
		!slices.Equal(unraid.HomeDomains, []string{"containers", "vms", "files"}) {
		t.Fatalf("Unraid = %+v", unraid)
	}
	second := placeNamed(t, plan, "Unraid 2")
	if second.Place.Base != "user/bombvault" || !maps.Equal(second.Place.Folders, map[string]string{"flash": "flash", "config": "config"}) {
		t.Fatalf("Unraid 2 = %+v", second)
	}
	for _, m := range []store.MigratedPlace{unraid, second} {
		p := m.Place
		if p.Kind != "local" || p.Provider != "unraid-folder" || p.OffPremises || !p.Enabled || p.RetentionKeepLast != 5 || p.CredsRef != "" || len(m.Rows) != 0 {
			t.Errorf("%s = %+v", p.Name, m)
		}
	}
	if len(plan.unplaced) != 0 {
		t.Errorf("unplaced = %v, want none", plan.unplaced)
	}
}

func TestADomainPathOnAShareIsAShare(t *testing.T) {
	in := migrationInput()
	in.settings.ContainersPath = "remotes/nas/bombvault/container"
	if p := placeNamed(t, planPlaces(in), "Unraid").Place; p.Base != "remotes/nas/bombvault" || p.Provider != "share" {
		t.Fatalf("Unraid = %+v, want the share under remotes/", p)
	}
}

func TestRemoteDomainPathsShareAPlaceOnlyWithTheSamePrimarySettings(t *testing.T) {
	in := migrationInput()
	in.settings.ContainersPath = "s3:https://s3.example.com/bv/containers"
	in.settings.VMsPath = "s3:https://s3.example.com/bv/vms"
	in.settings.FilesPath = "s3:https://s3.example.com/bv/files"
	in.primaries["containers"] = store.OffsiteTarget{CredsRef: "garage", LimitUpload: 500, Enabled: true}
	in.primaries["vms"] = store.OffsiteTarget{CredsRef: "garage", LimitUpload: 500, Enabled: true}
	in.primaries["files"] = store.OffsiteTarget{CredsRef: "garage", LimitUpload: 500, Immutable: true, Enabled: true}
	plan := planPlaces(in)
	shared := placeNamed(t, plan, "s3.example.com")
	p := shared.Place
	if !slices.Equal(shared.HomeDomains, []string{"containers", "vms"}) || p.Base != "s3:https://s3.example.com/bv" || p.Kind != "s3" ||
		p.CredsRef != "garage" || p.LimitUpload != 500 || p.Immutable || !p.OffPremises || p.RetentionKeepLast != 5 || p.Provider != "s3-other" {
		t.Fatalf("s3.example.com = %+v", shared)
	}
	if other := placeNamed(t, plan, "s3.example.com 2"); !slices.Equal(other.HomeDomains, []string{"files"}) || !other.Place.Immutable {
		t.Fatalf("s3.example.com 2 = %+v, want files alone, append-only", other)
	}
}

func TestASwitchedOffPrimaryRowLendsItsPlaceOnlyItsCredentials(t *testing.T) {
	in := migrationInput()
	in.settings.ConfigPath = "s3:https://s3.example.com/bv/config"
	in.primaries["config"] = store.OffsiteTarget{
		CredsRef: "garage", StorageClass: "STANDARD_IA", Immutable: true, LimitUpload: 500, LimitDownload: 250, GrowthBudgetGB: 50,
	}
	p := placeNamed(t, planPlaces(in), "s3.example.com").Place
	if p.CredsRef != "garage" || p.StorageClass != "" || p.Immutable || p.LimitUpload != 0 || p.LimitDownload != 0 || p.GrowthBudgetGB != 0 {
		t.Fatalf("place = %+v, want the credential set and nothing else of a switched-off row", p)
	}
}

func TestARemoteDomainPathAtABucketRootIsARepositoryPlace(t *testing.T) {
	in := migrationInput()
	in.settings.ContainersPath = "s3:https://garage.example/bucket"
	m := placeNamed(t, planPlaces(in), "garage.example")
	if m.Place.Base != "s3:https://garage.example/bucket" || !maps.Equal(m.Place.Folders, map[string]string{"containers": ""}) ||
		!slices.Equal(m.HomeDomains, []string{"containers"}) {
		t.Fatalf("garage.example = %+v", m)
	}
}

func TestDomainPathsWithoutAPlaceFormStayUnplaced(t *testing.T) {
	in := migrationInput()
	in.settings.FlashPath = "b2:bucket:flash"
	in.settings.ConfigPath = "backups"
	plan := planPlaces(in)
	if !slices.Contains(plan.unplaced, "flash path") || !slices.Contains(plan.unplaced, "config path") {
		t.Fatalf("unplaced = %v, want the flash and config paths", plan.unplaced)
	}
	for _, m := range plan.places {
		if slices.Contains(m.HomeDomains, "flash") || slices.Contains(m.HomeDomains, "config") {
			t.Fatalf("%s took a path it cannot spell: %+v", m.Place.Name, m)
		}
	}
}

func TestADomainThatAlreadyHasAHomePlaceIsLeftAlone(t *testing.T) {
	in := migrationInput()
	in.existing = []store.Place{{ID: "p1", Name: "Unraid", Kind: "local", Base: "user/bombvault", Folders: map[string]string{"containers": "container"}}}
	in.homes = map[string]string{"containers": "p1"}
	plan := planPlaces(in)
	m := placeNamed(t, plan, "Unraid 2")
	if !slices.Equal(m.HomeDomains, []string{"vms", "flash", "config", "files"}) {
		t.Fatalf("Unraid 2 = %+v, want the four other domains under a name the existing place leaves free", m)
	}
	if len(plan.places) != 1 {
		t.Fatalf("places = %q, want one", placeNames(plan))
	}
}

const b2Bucket = "s3:https://s3.us-west-004.backblazeb2.com/bv-bucket"

// offsiteRow is an enabled target on keep-last 3.
func offsiteRow(id, domain, name, repo string, created int64) store.OffsiteTarget {
	return store.OffsiteTarget{
		ID: id, Domain: domain, Name: name, Repo: repo, Role: store.RoleOffsite, Enabled: true, CreatedAt: created, RetentionKeepLast: 3,
	}
}

// placeOfRow returns the place a row went to and how it sits there.
func placeOfRow(plan placesPlan, rowID string) (store.MigratedPlace, store.PlaceRowRef, bool) {
	for _, m := range plan.places {
		for _, r := range m.Rows {
			if r.RowID == rowID {
				return m, r, true
			}
		}
	}
	return store.MigratedPlace{}, store.PlaceRowRef{}, false
}

func TestTargetsWithOneBaseAndTheSameSettingsShareAPlace(t *testing.T) {
	in := migrationInput()
	in.targets = []store.OffsiteTarget{
		offsiteRow("t-vms", "vms", "B2 VMs", b2Bucket+"/vms", 2),
		offsiteRow("t-cont", "containers", "B2", b2Bucket+"/containers", 1),
	}
	b2 := placeNamed(t, planPlaces(in), "B2")
	p := b2.Place
	if p.Base != b2Bucket || p.Folders["containers"] != "containers" || p.Folders["vms"] != "vms" || len(b2.Rows) != 2 ||
		p.RetentionKeepLast != 3 || !p.OffPremises || !p.Enabled || p.Provider != "b2" || p.Kind != "s3" || p.CredsRef != "" {
		t.Fatalf("B2 = %+v, want the shared credentials left as they are", b2)
	}
}

func TestATargetAlreadyOnAPlaceIsLeftAlone(t *testing.T) {
	placed := offsiteRow("placed", "containers", "B2", b2Bucket+"/containers", 1)
	placed.PlaceID = "p1"
	in := migrationInput()
	in.targets = []store.OffsiteTarget{placed}
	plan := planPlaces(in)
	if _, _, ok := placeOfRow(plan, "placed"); ok || slices.Contains(plan.unplaced, "placed") {
		t.Fatalf("a target that has a place was planned again: %+v", plan)
	}
}

func TestTargetsThatDifferInAnySettingGetPlacesOfTheirOwn(t *testing.T) {
	for name, change := range map[string]func(*store.OffsiteTarget){
		"credentials":    func(r *store.OffsiteTarget) { r.CredsRef = "other" },
		"storage class":  func(r *store.OffsiteTarget) { r.StorageClass = "STANDARD_IA" },
		"append-only":    func(r *store.OffsiteTarget) { r.Immutable = true },
		"keep daily":     func(r *store.OffsiteTarget) { r.RetentionKeepDaily = 7 },
		"upload limit":   func(r *store.OffsiteTarget) { r.LimitUpload = 100 },
		"download limit": func(r *store.OffsiteTarget) { r.LimitDownload = 100 },
		"growth budget":  func(r *store.OffsiteTarget) { r.GrowthBudgetGB = 10 },
	} {
		t.Run(name, func(t *testing.T) {
			other := offsiteRow("t2", "vms", "B2 VMs", b2Bucket+"/vms", 2)
			change(&other)
			in := migrationInput()
			in.targets = []store.OffsiteTarget{offsiteRow("t1", "containers", "B2", b2Bucket+"/containers", 1), other}
			if m, _, ok := placeOfRow(planPlaces(in), "t2"); !ok || m.Place.Name != "B2 VMs" || m.Place.Base != b2Bucket {
				t.Fatalf("the differing target sits on %+v, want a place of its own at the same base", m.Place)
			}
		})
	}
}

func TestTheLaterOfTwoTargetsOfOneDomainAtOnePlaceGetsItsOwn(t *testing.T) {
	in := migrationInput()
	in.targets = []store.OffsiteTarget{
		offsiteRow("new", "containers", "B2 again", b2Bucket+"/containers-2", 2),
		offsiteRow("old", "containers", "B2", b2Bucket+"/containers", 1),
		offsiteRow("vms", "vms", "B2 VMs", b2Bucket+"/vms", 3),
	}
	plan := planPlaces(in)
	if m, _, _ := placeOfRow(plan, "old"); m.Place.Name != "B2" {
		t.Fatalf("the older target sits on %q, want B2", m.Place.Name)
	}
	if m, _, _ := placeOfRow(plan, "new"); m.Place.Name != "B2 again" || m.Place.Folders["containers"] != "containers-2" || len(m.Rows) != 1 {
		t.Fatalf("the later target sits on %+v, want B2 again to itself", m)
	}
	if m, _, _ := placeOfRow(plan, "vms"); m.Place.Name != "B2" {
		t.Fatalf("the vms target sits on %q, want B2, the place of the older containers target", m.Place.Name)
	}
}

func TestATargetAtABucketRootNeverSharesAPlaceWithOneInsideIt(t *testing.T) {
	for name, c := range map[string]struct {
		rootCreated, innerCreated int64
		older, later              string
	}{
		"root first":   {1, 2, "root", "inner"},
		"folder first": {2, 1, "inner", "root"},
	} {
		t.Run(name, func(t *testing.T) {
			in := migrationInput()
			in.targets = []store.OffsiteTarget{
				offsiteRow("root", "containers", "Bucket", b2Bucket, c.rootCreated),
				offsiteRow("inner", "vms", "Bucket VMs", b2Bucket+"/vms", c.innerCreated),
			}
			plan := planPlaces(in)
			if _, _, ok := placeOfRow(plan, c.older); !ok {
				t.Fatalf("the older target %s has no place: %+v", c.older, plan)
			}
			if m, _, ok := placeOfRow(plan, c.later); ok || !slices.Contains(plan.unplaced, c.later) {
				t.Fatalf("the later target %s sits on %+v, want it without a place", c.later, m)
			}
		})
	}
}

func TestATargetJoinsAHomePlaceOnlyWhereItsDomainIsFree(t *testing.T) {
	in := migrationInput()
	in.settings.FlashPath = "boot-backups/flash"
	copyRow := func(id, domain, repo string) store.OffsiteTarget {
		r := offsiteRow(id, domain, id, repo, 1)
		r.RetentionKeepLast = 5
		return r
	}
	in.targets = []store.OffsiteTarget{
		copyRow("flash-copy", "flash", "user/bombvault/flash-copies"),
		copyRow("containers-copy", "containers", "user/bombvault/container-copies"),
	}
	plan := planPlaces(in)
	if m, _, _ := placeOfRow(plan, "flash-copy"); m.Place.Name != "Unraid" || m.Place.Folders["flash"] != "flash-copies" {
		t.Fatalf("the flash copy sits on %+v, want Unraid, which has no flash folder", m.Place)
	}
	if m, _, _ := placeOfRow(plan, "containers-copy"); m.Place.Name != "containers-copy" || m.Place.Base != "user/bombvault" || m.Place.OffPremises {
		t.Fatalf("the containers copy sits on %+v, want a local place of its own", m.Place)
	}
}

func TestATargetAtABucketRootIsAPlaceThatIsItsOwnRepository(t *testing.T) {
	in := migrationInput()
	in.targets = []store.OffsiteTarget{offsiteRow("root", "containers", "Bucket", b2Bucket, 1)}
	m := placeNamed(t, planPlaces(in), "Bucket")
	if m.Place.Base != b2Bucket || !maps.Equal(m.Place.Folders, map[string]string{"containers": ""}) {
		t.Fatalf("Bucket = %+v", m.Place)
	}
}

func TestALocalTargetPlaceStandsOnThePremises(t *testing.T) {
	in := migrationInput()
	in.targets = []store.OffsiteTarget{offsiteRow("nas", "containers", "NAS copies", "remotes/nas/copies/containers", 1)}
	if p := placeNamed(t, planPlaces(in), "NAS copies").Place; p.OffPremises || p.Provider != "share" || p.Kind != "local" {
		t.Fatalf("NAS copies = %+v", p)
	}
}

func TestAPlaceIsOnWhileAnyOfItsTargetsIs(t *testing.T) {
	off := offsiteRow("off", "vms", "B2 VMs", b2Bucket+"/vms", 2)
	off.Enabled = false
	alone := offsiteRow("alone", "flash", "Old", "s3:https://s3.example.com/old/flash", 3)
	alone.Enabled = false
	in := migrationInput()
	in.targets = []store.OffsiteTarget{offsiteRow("on", "containers", "B2", b2Bucket+"/containers", 1), off, alone}
	plan := planPlaces(in)
	if !placeNamed(t, plan, "B2").Place.Enabled {
		t.Error("B2 is off although one of its targets is on")
	}
	if placeNamed(t, plan, "Old").Place.Enabled {
		t.Error("a place whose only target is off is on")
	}
}

func TestEveryPlaceProviderSpeaksThePlacesKind(t *testing.T) {
	in := migrationInput()
	in.credSets["mesh-1"] = CloudCredSet{ID: "mesh-1", Name: "mesh: tower", CloudCreds: CloudCreds{RESTUser: "bombvault-flash"}}
	mesh := offsiteRow("mesh", "flash", "mesh: tower", "rest:http://tower:8000/bombvault-flash/flash", 4)
	mesh.CredsRef = "mesh-1"
	in.targets = []store.OffsiteTarget{
		offsiteRow("b2", "containers", "B2", b2Bucket+"/containers", 1),
		offsiteRow("wasabi", "files", "Wasabi", "s3:https://s3.eu-central-1.wasabisys.com/bv-files/files", 2),
		offsiteRow("rest", "vms", "Tower", "rest:http://tower:8000/bombvault-vms/vms", 3),
		mesh,
		offsiteRow("sftp", "flash", "Pi", "sftp:pi:flash", 5),
		offsiteRow("rclone", "config", "Drive", "rclone:r:config", 6),
		offsiteRow("box", "files", "Box", "rest:https://u1.your-storagebox.de/bv/files", 7),
	}
	plan := planPlaces(in)
	for name, provider := range map[string]string{
		"B2": "b2", "Wasabi": "wasabi", "Tower": "rest-server", "mesh: tower": "bombvault", "Pi": "sftp", "Drive": "rclone",
	} {
		if got := placeNamed(t, plan, name).Place.Provider; got != provider {
			t.Errorf("%s provider = %q, want %q", name, got, provider)
		}
	}
	for _, m := range plan.places {
		if found, ok := places.ProviderByID(m.Place.Provider); !ok || string(found.Kind) != m.Place.Kind {
			t.Errorf("%s: provider %q does not speak %s", m.Place.Name, m.Place.Provider, m.Place.Kind)
		}
	}
}

func TestPlaceNamesStayUnique(t *testing.T) {
	in := migrationInput()
	in.settings.FlashPath = "boot-backups/flash"
	in.targets = []store.OffsiteTarget{
		offsiteRow("a", "containers", "Unraid", b2Bucket+"/containers", 1),
		offsiteRow("b", "containers", "B2", "s3:https://s3.example.com/one/containers", 2),
		offsiteRow("c", "vms", "b2", "s3:https://s3.example.com/two/vms", 3),
	}
	if got, want := placeNames(planPlaces(in)), []string{"Unraid", "Unraid 2", "Unraid 3", "B2", "b2 2"}; !slices.Equal(got, want) {
		t.Fatalf("names = %q, want %q", got, want)
	}
}

func TestTargetsWithoutAPlaceFormStayUnplaced(t *testing.T) {
	in := migrationInput()
	in.targets = []store.OffsiteTarget{
		offsiteRow("native", "files", "B2 native", "b2:bucket:files", 1),
		offsiteRow("slash", "vms", "Slash", "sftp:nas:/vms", 2),
		offsiteRow("outer", "vms", "Outer", b2Bucket+"/bv", 3),
		offsiteRow("nested", "containers", "Nested", b2Bucket+"/bv/containers", 4),
	}
	plan := planPlaces(in)
	for _, id := range []string{"native", "slash", "nested"} {
		if _, _, ok := placeOfRow(plan, id); ok || !slices.Contains(plan.unplaced, id) {
			t.Errorf("%s was placed, want it without a place", id)
		}
	}
	if _, _, ok := placeOfRow(plan, "outer"); !ok {
		t.Error("the outer target lost its place to the one nested in it")
	}
}

// namedRow is an enabled named repository with its own keep columns at zero.
func namedRow(id, name, repo string) store.OffsiteTarget {
	return store.OffsiteTarget{ID: id, Role: store.RoleRepo, Name: name, Repo: repo, Enabled: true}
}

func TestATargetPlaceOffersEveryDomainUnderTheUsualFolders(t *testing.T) {
	in := migrationInput()
	in.targets = []store.OffsiteTarget{offsiteRow("b2", "containers", "B2", b2Bucket+"/containers", 1)}
	want := map[string]string{"containers": "containers", "vms": "vms", "flash": "flash", "config": "config", "files": "files"}
	if got := placeNamed(t, planPlaces(in), "B2").Place.Folders; !maps.Equal(got, want) {
		t.Fatalf("B2 folders = %v, want %v", got, want)
	}
}

func TestADefaultFolderThatMeetsAnAddressInUseIsLeftOut(t *testing.T) {
	in := migrationInput()
	in.settings.VMsPath = b2Bucket + "/vms"
	in.named = []store.OffsiteTarget{namedRow("cold", "Cold", b2Bucket+"/flash")}
	in.targets = []store.OffsiteTarget{offsiteRow("b2", "containers", "B2", b2Bucket+"/containers", 1)}
	folders := placeNamed(t, planPlaces(in), "B2").Place.Folders
	for _, d := range []string{"vms", "flash"} {
		if f, ok := folders[d]; ok {
			t.Errorf("B2 offers %s at %q, where something already lives", d, f)
		}
	}
	if folders["config"] != "config" || folders["files"] != "files" {
		t.Errorf("B2 folders = %v, want config and files still offered", folders)
	}
}

func TestPlacesWhoseCredentialsBelongToOneDomainOfferOnlyThatDomain(t *testing.T) {
	withCreds := func(row store.OffsiteTarget, ref string) store.OffsiteTarget {
		row.CredsRef = ref
		return row
	}
	for name, c := range map[string]struct {
		sets    []CloudCredSet
		targets []store.OffsiteTarget
		want    map[string]map[string]string // place -> folders
	}{
		"an accepted mesh offer": {
			sets:    []CloudCredSet{{ID: "m", Name: "mesh: tower", CloudCreds: CloudCreds{RESTUser: "peer"}}},
			targets: []store.OffsiteTarget{withCreds(offsiteRow("mesh", "flash", "mesh: tower", "rest:http://tower:8000/peer/flash", 1), "m")},
			want:    map[string]map[string]string{"mesh: tower": {"flash": "flash"}},
		},
		"a rest-server user named after the domain": {
			sets:    []CloudCredSet{{ID: "u", Name: "Tower", CloudCreds: CloudCreds{RESTUser: "bombvault-containers"}}},
			targets: []store.OffsiteTarget{withCreds(offsiteRow("tower", "containers", "Tower", "rest:http://tower:8000/bombvault-containers/containers", 1), "u")},
			want:    map[string]map[string]string{"Tower": {"containers": "containers"}},
		},
		"places split by nothing but their credentials": {
			sets: []CloudCredSet{{ID: "a", Name: "A", CloudCreds: CloudCreds{RESTUser: "x"}}, {ID: "b", Name: "B", CloudCreds: CloudCreds{RESTUser: "y"}}},
			targets: []store.OffsiteTarget{
				withCreds(offsiteRow("c", "containers", "Tower", "rest:http://tower:8000/containers", 1), "a"),
				withCreds(offsiteRow("v", "vms", "Tower VMs", "rest:http://tower:8000/vms", 2), "b"),
			},
			want: map[string]map[string]string{"Tower": {"containers": "containers"}, "Tower VMs": {"vms": "vms"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			in := migrationInput()
			for _, set := range c.sets {
				in.credSets[set.ID] = set
			}
			in.targets = c.targets
			plan := planPlaces(in)
			for place, folders := range c.want {
				if got := placeNamed(t, plan, place).Place.Folders; !maps.Equal(got, folders) {
					t.Errorf("%s folders = %v, want %v", place, got, folders)
				}
			}
		})
	}
}

func TestPlacesThatDifferInMoreThanCredentialsStillOfferFreeDomains(t *testing.T) {
	c := offsiteRow("c", "containers", "Tower", "rest:http://tower:8000/containers", 1)
	c.CredsRef = "a"
	v := offsiteRow("v", "vms", "Tower VMs", "rest:http://tower:8000/vms", 2)
	v.CredsRef, v.RetentionKeepLast = "b", 9
	in := migrationInput()
	in.targets = []store.OffsiteTarget{c, v}
	plan := planPlaces(in)
	if got := placeNamed(t, plan, "Tower").Place.Folders; !maps.Equal(got, map[string]string{"containers": "containers", "flash": "flash", "config": "config", "files": "files"}) {
		t.Errorf("Tower folders = %v, want its own and the free domains", got)
	}
	// Tower took flash, config and files first; the containers default,
	// "container", meets nothing, since Tower's own folder is "containers".
	if got := placeNamed(t, plan, "Tower VMs").Place.Folders; !maps.Equal(got, map[string]string{"vms": "vms", "containers": "container"}) {
		t.Errorf("Tower VMs folders = %v, want its own and the one default Tower left free", got)
	}
}

func TestHomeAndRepositoryPlacesGetNoDefaultFolders(t *testing.T) {
	in := migrationInput()
	in.settings.VMsPath, in.settings.FlashPath, in.settings.ConfigPath, in.settings.FilesPath = "b2:x:vms", "b2:x:flash", "b2:x:config", "b2:x:files"
	in.targets = []store.OffsiteTarget{offsiteRow("root", "containers", "Bucket", "s3:https://s3.example.com/bucket", 1)}
	plan := planPlaces(in)
	if got := placeNamed(t, plan, "Unraid").Place.Folders; !maps.Equal(got, map[string]string{"containers": "container"}) {
		t.Errorf("Unraid folders = %v, want the domain path alone", got)
	}
	if got := placeNamed(t, plan, "Bucket").Place.Folders; !maps.Equal(got, map[string]string{"containers": ""}) {
		t.Errorf("Bucket folders = %v, want the root alone", got)
	}
}

func TestARemoteHomePlaceATargetJoinsIsNamedAfterItAndOffersEveryDomain(t *testing.T) {
	in := migrationInput()
	in.settings.FlashPath = "rest:http://nas:8000/bvp/flash"
	files := offsiteRow("rest", "files", "bvp rest", "rest:http://nas:8000/bvp/files", 1)
	files.RetentionKeepLast = 5
	in.targets = []store.OffsiteTarget{files}
	plan := planPlaces(in)
	m, _, _ := placeOfRow(plan, "rest")
	want := map[string]string{"flash": "flash", "files": "files", "containers": "container", "vms": "vms", "config": "config"}
	if m.Place.Name != "bvp rest" || !slices.Equal(m.HomeDomains, []string{"flash"}) || !maps.Equal(m.Place.Folders, want) {
		t.Fatalf("place = %+v, want it named after the target, home to flash, with every domain", m)
	}
	if slices.Contains(placeNames(plan), "nas:8000") {
		t.Fatalf("places = %q, want no place named after the server", placeNames(plan))
	}
}

func TestARemoteHomePlaceKeepsItsServerNameWithoutATarget(t *testing.T) {
	in := migrationInput()
	in.settings.FlashPath = "rest:http://nas:8000/bvp/flash"
	if got := placeNamed(t, planPlaces(in), "nas:8000").Place.Folders; !maps.Equal(got, map[string]string{"flash": "flash"}) {
		t.Fatalf("folders = %v, want the domain path alone", got)
	}
}

// directRow is target's direct repository at repo, with every mirrored field
// the target has, the way CreateCompanionRepo writes it.
func directRow(id, name, repo string, target store.OffsiteTarget) store.OffsiteTarget {
	r := target
	r.ID, r.Name, r.Repo, r.Role, r.Domain, r.CompanionOf = id, name, repo, store.RoleRepo, "", target.ID
	return r
}

func TestADirectRepositoryBesideItsTargetJoinsTheTargetsPlace(t *testing.T) {
	b2 := offsiteRow("b2", "containers", "B2", b2Bucket+"/containers", 1)
	in := migrationInput()
	in.targets = []store.OffsiteTarget{b2}
	in.named = []store.OffsiteTarget{directRow("d", "B2 direct", b2.Repo+"-direct", b2)}
	m, ref, ok := placeOfRow(planPlaces(in), "d")
	if !ok || m.Place.Name != "B2" || ref.Domain != "containers" || ref.Suffix != "-direct" {
		t.Fatalf("the direct repository sits on %q as %+v, %v, want B2 as containers-direct", m.Place.Name, ref, ok)
	}
}

func TestADirectRepositoryThatIsOnKeepsItsPlaceOn(t *testing.T) {
	b2 := offsiteRow("b2", "containers", "B2", b2Bucket+"/containers", 1)
	b2.Enabled = false
	direct := directRow("d", "B2 direct", b2.Repo+"-direct", b2)
	direct.Enabled = true
	in := migrationInput()
	in.targets = []store.OffsiteTarget{b2}
	in.named = []store.OffsiteTarget{direct}
	if m, _, ok := placeOfRow(planPlaces(in), "d"); !ok || !m.Place.Enabled {
		t.Fatalf("B2 = %+v, %v, want it on while items still back up to its direct repository", m.Place, ok)
	}
}

func TestDirectRepositoriesAnywhereElseStayWithoutAPlace(t *testing.T) {
	b2 := offsiteRow("b2", "containers", "B2", b2Bucket+"/containers", 1)
	root := offsiteRow("root", "vms", "Root", "s3:https://s3.example.com/bucket", 2)
	native := offsiteRow("native", "files", "Native", "b2:bucket:files", 3)
	kept := directRow("d", "B2 direct", b2.Repo+"-direct", b2)
	kept.CredsRef = "kept-set"
	for name, c := range map[string]struct{ target, direct store.OffsiteTarget }{
		"in another bucket":           {b2, directRow("d", "B2 direct", "s3:https://s3.us-west-004.backblazeb2.com/other/containers", b2)},
		"beside a bucket root":        {root, directRow("d", "Root direct", root.Repo+"-direct", root)},
		"of a target without a place": {native, directRow("d", "Native direct", native.Repo+"-direct", native)},
		"on credentials it kept":      {b2, kept},
	} {
		t.Run(name, func(t *testing.T) {
			in := migrationInput()
			in.targets = []store.OffsiteTarget{c.target}
			in.named = []store.OffsiteTarget{c.direct}
			plan := planPlaces(in)
			if m, _, ok := placeOfRow(plan, "d"); ok || !slices.Contains(plan.unplaced, "d") {
				t.Fatalf("the direct repository sits on %q, want it without a place", m.Place.Name)
			}
		})
	}
}

func TestEachNamedRepositoryIsAPlaceOfItsOwn(t *testing.T) {
	off := namedRow("b", "NAS B", "remotes/nas/b")
	off.Enabled = false
	cold := namedRow("cold", "Cold", "s3:https://s3.example.com/bv-cold")
	cold.OffPremises, cold.LimitUpload = true, 200
	in := migrationInput()
	in.named = []store.OffsiteTarget{namedRow("a", "NAS A", "remotes/nas/a"), off, cold}
	plan := planPlaces(in)
	every := map[string]string{"containers": "", "vms": "", "flash": "", "config": "", "files": ""}
	for _, id := range []string{"a", "b", "cold"} {
		m, ref, ok := placeOfRow(plan, id)
		if !ok || ref.Domain != "" || ref.Suffix != "" || len(m.Rows) != 1 || m.Place.Base != ref.Repo ||
			!maps.Equal(m.Place.Folders, every) || m.Place.RetentionKeepLast != 5 {
			t.Errorf("%s sits on %+v as %+v, %v, want a place of its own on the global keep-last 5", id, m, ref, ok)
		}
	}
	if p := placeNamed(t, plan, "NAS A").Place; p.Provider != "share" || p.OffPremises || !p.Enabled {
		t.Errorf("NAS A = %+v", p)
	}
	if p := placeNamed(t, plan, "NAS B").Place; p.Enabled {
		t.Errorf("NAS B = %+v, want it off like its row", p)
	}
	if p := placeNamed(t, plan, "Cold").Place; !p.OffPremises || p.LimitUpload != 200 || p.Provider != "s3-other" {
		t.Errorf("Cold = %+v", p)
	}
}

func TestANamedRepositorysPlaceStandsWhereItsRowSays(t *testing.T) {
	away := namedRow("away", "Parents' NAS", "remotes/parents/bv")
	away.OffPremises = true
	in := migrationInput()
	in.named = []store.OffsiteTarget{away, namedRow("house", "MinIO", "s3:https://minio.lan:9000/bv")}
	plan := planPlaces(in)
	if p := placeNamed(t, plan, "Parents' NAS").Place; !p.OffPremises {
		t.Errorf("Parents' NAS = %+v, want it off the premises like its row", p)
	}
	if p := placeNamed(t, plan, "MinIO").Place; p.OffPremises {
		t.Errorf("MinIO = %+v, want it on the premises like its row", p)
	}
}

func TestANamedRepositoryWithoutAPlaceKindStaysUnplaced(t *testing.T) {
	in := migrationInput()
	in.named = []store.OffsiteTarget{namedRow("native", "Native", "b2:bucket:cold")}
	plan := planPlaces(in)
	if _, _, ok := placeOfRow(plan, "native"); ok || !slices.Contains(plan.unplaced, "native") {
		t.Fatal("a b2: repository was put on a place")
	}
}

func TestARowItsPlaceCannotSpellStaysWithoutAPlace(t *testing.T) {
	p := &placesPlanner{in: migrationInput()}
	pl := &plannedPlace{}
	pl.Place = store.Place{Name: "B2", Base: b2Bucket, Folders: map[string]string{"containers": "containers"}}
	pl.Rows = []store.PlaceRowRef{
		{RowID: "fits", Domain: "containers", Repo: b2Bucket + "/containers"},
		{RowID: "direct", Domain: "containers", Suffix: "-direct", Repo: b2Bucket + "/containers-direct"},
		{RowID: "whole", Repo: b2Bucket},
		{RowID: "elsewhere", Domain: "containers", Repo: b2Bucket + "/elsewhere"},
		{RowID: "absent", Domain: "vms", Repo: b2Bucket + "/vms"},
	}
	pl.HomeDomains = []string{"containers"}
	p.planned = []*plannedPlace{pl}
	p.checkInvariant()
	var kept []string
	for _, r := range pl.Rows {
		kept = append(kept, r.RowID)
	}
	if !slices.Equal(kept, []string{"fits", "direct", "whole"}) {
		t.Errorf("rows kept = %v, want the three the place spells", kept)
	}
	if want := []string{"elsewhere", "absent", "containers path"}; !slices.Equal(p.unplaced, want) {
		t.Errorf("unplaced = %v, want %v", p.unplaced, want)
	}
	if len(pl.HomeDomains) != 0 {
		t.Errorf("home domains = %v, want none: the containers path is user/bombvault/container", pl.HomeDomains)
	}
}
