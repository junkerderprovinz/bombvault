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
