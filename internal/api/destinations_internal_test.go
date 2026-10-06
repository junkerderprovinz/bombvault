package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestADomainRepositorySitsUnderItsDestination(t *testing.T) {
	for _, c := range []struct{ base, want string }{
		{"rclone:onedrive:", "rclone:onedrive:vms"},
		{"rclone:onedrive:BombVault/", "rclone:onedrive:BombVault/vms"},
		{"s3:https://s3.example.com/bucket", "s3:https://s3.example.com/bucket/vms"},
		{"rest:https://box:8000/bombvault/", "rest:https://box:8000/bombvault/vms"},
		{"remotes/NAS2/bv", "remotes/NAS2/bv/vms"},
	} {
		if got := destinationLocation(c.base, "vms"); got != c.want {
			t.Errorf("destinationLocation(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}

func TestTheSelfBackupIsNotPutWhereARestServerKeepsItsConfig(t *testing.T) {
	if got := destinationLocation("rest:http://box:8000/", "config"); got != "rest:http://box:8000/selfbackup" {
		t.Errorf("self-backup location = %q, want it under selfbackup", got)
	}
}

func TestTheS3EndpointGetsAScheme(t *testing.T) {
	aws, _ := remotes.FindProvider("aws")
	wasabi, _ := remotes.FindProvider("wasabi")
	for _, c := range []struct {
		p        remotes.Provider
		settings map[string]string
		want     string
	}{
		{wasabi, map[string]string{"endpoint": "s3.eu-central-2.wasabisys.com/"}, "https://s3.eu-central-2.wasabisys.com"},
		{wasabi, map[string]string{"endpoint": "http://minio.lan:9000"}, "http://minio.lan:9000"},
		{aws, map[string]string{"region": "eu-west-1"}, "https://s3.eu-west-1.amazonaws.com"},
		{aws, nil, "https://s3.amazonaws.com"},
		{wasabi, map[string]string{"endpoint": "s3:http://192.168.2.51:9000/bombvault/dxp6800"}, "http://192.168.2.51:9000"},
		{wasabi, map[string]string{"endpoint": "s3:s3.eu-central-2.wasabisys.com/bucket"}, "https://s3.eu-central-2.wasabisys.com"},
		{wasabi, map[string]string{"endpoint": "https://gateway.lan/s3"}, "https://gateway.lan/s3"},
	} {
		got, err := s3Endpoint(c.p, c.settings)
		if err != nil || got != c.want {
			t.Errorf("s3Endpoint(%s, %v) = %q, %v, want %q", c.p.ID, c.settings, got, err, c.want)
		}
	}
	if _, err := s3Endpoint(wasabi, nil); err == nil {
		t.Error("a missing endpoint was accepted")
	}
}

func TestAResticAddressInTheEndpointFieldReachesRcloneAsTheServer(t *testing.T) {
	p, _ := remotes.FindProvider("s3")
	d, err := new(Service).draftFor(p, map[string]string{"endpoint": " s3:http://192.168.2.51:9000/bombvault/dxp6800 "})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Settings["endpoint"]; got != "http://192.168.2.51:9000" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestARemoteSectionCannotSmuggleInAnotherRemote(t *testing.T) {
	if _, err := settingsSection("nas", "webdav", map[string]string{"url": "https://x\n[evil]\ntype = local"}); err == nil {
		t.Fatal("a value with a line break was written")
	}
	if _, err := settingsSection("nas", "webdav", map[string]string{"type": "local"}); err == nil {
		t.Fatal("a setting named type was written")
	}
	got, err := settingsSection("nas", "webdav", map[string]string{"vendor": "nextcloud", "url": "https://cloud"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "[nas]\ntype = webdav\nurl = https://cloud\nvendor = nextcloud\n" {
		t.Fatalf("section = %q", got)
	}
}

func TestReplacingASectionKeepsTheOthers(t *testing.T) {
	conf := "[a]\ntype = s3\n\n[b]\ntype = webdav\n"
	if got := replaceRcloneSection(conf, "a", ""); got != "[b]\ntype = webdav\n" {
		t.Fatalf("removing a = %q", got)
	}
	if got := replaceRcloneSection(conf, "B", "[B]\ntype = smb\n"); got != "[a]\ntype = s3\n\n[B]\ntype = smb\n" {
		t.Fatalf("replacing b = %q", got)
	}
	if got := freeRemoteName(conf, "a"); got != "a-2" {
		t.Fatalf("freeRemoteName = %q, want a-2", got)
	}
}

func TestAPathDestinationIsDerivedPerDomain(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	if err := os.MkdirAll(filepath.Join(svc.cfg.HostMountRoot, "remotes", "NAS2"), 0o750); err != nil {
		t.Fatal(err)
	}
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "path", Name: "Second NAS", Settings: map[string]string{"path": "remotes/NAS2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{store: st, svc: svc}
	call := func() map[string]any {
		req := jsonReq(http.MethodPost, "/api/offsite/destinations/"+d.ID+"/domains/flash", nil)
		req.SetPathValue("id", d.ID)
		req.SetPathValue("domain", "flash")
		rec := httptest.NewRecorder()
		h.handleDestinationForDomain(rec, req)
		return decodeEnvelope(t, rec)
	}
	first := call()
	if first["ok"] != true || first["created"] != true {
		t.Fatalf("first answer = %v", first)
	}
	target := first["target"].(map[string]any)
	if target["repo"] != "remotes/NAS2/flash" || target["name"] != "Second NAS" {
		t.Fatalf("target = %v", target)
	}
	if again := call(); again["created"] != false || again["target"].(map[string]any)["id"] != target["id"] {
		t.Fatalf("second answer = %v, want the same target", again)
	}
	if err := svc.DeleteDestination(d.ID); err == nil || !strings.Contains(err.Error(), "still copy") {
		t.Fatalf("deleting a destination in use: %v", err)
	}
}

func TestAnS3DestinationBringsItsOwnCredentials(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "wasabi", Dir: "/bucket/bv/", StorageClass: "standard",
		Settings: map[string]string{"endpoint": "s3.eu-central-2.wasabisys.com", "access_key_id": "AKID", "secret_access_key": "SECRET"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Repo != "s3:https://s3.eu-central-2.wasabisys.com/bucket/bv" || d.StorageClass != "STANDARD" || d.Provider != "wasabi" {
		t.Fatalf("destination = %+v", d)
	}
	sets, err := svc.CloudCredSets()
	if err != nil || len(sets) != 1 || sets[0].ID != d.CredsRef || sets[0].S3KeyID != "AKID" || sets[0].S3Secret != "" {
		t.Fatalf("credential sets = %+v, %v, want one set named by the destination, its secret not shown", sets, err)
	}

	if err := svc.DeleteDestination(d.ID); err != nil {
		t.Fatal(err)
	}
	if sets, _ := svc.CloudCredSets(); len(sets) != 0 {
		t.Fatalf("credential sets after delete = %+v, want none", sets)
	}
	if _, ok, _ := st.GetDestination(d.ID); ok {
		t.Fatal("the destination is still there")
	}
}

func TestTheRecoveryKitListsEachDestinationsRepositories(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	d, err := st.SaveDestination(store.OffsiteTarget{Name: "Backblaze", Repo: "rclone:b2:bv"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.EnsureDestinationTarget(d.ID, "vms", "rclone:b2:bv/vms"); err != nil {
		t.Fatal(err)
	}
	kit, err := svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Destinations", "- Backblaze: rclone:b2:bv", "  vms: rclone:b2:bv/vms"} {
		if !strings.Contains(kit, want) {
			t.Errorf("the kit lacks %q", want)
		}
	}
}

func TestAnS3DestinationNeedsABucket(t *testing.T) {
	svc, _, _ := newProbeSvc(t, nil)
	_, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "wasabi", Settings: map[string]string{"endpoint": "s3.wasabisys.com", "access_key_id": "a", "secret_access_key": "b"},
	})
	if err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("err = %v, want the bucket to be asked for", err)
	}
	if sets, _ := svc.CloudCredSets(); len(sets) != 0 {
		t.Fatalf("a refused destination left credential sets behind: %+v", sets)
	}
}

func TestAPresetBeatsTheForm(t *testing.T) {
	svc, _, _ := newProbeSvc(t, nil)
	p, _ := remotes.FindProvider("nextcloud")
	d, err := svc.draftFor(p, map[string]string{"vendor": "other", "url": " https://cloud "})
	if err != nil {
		t.Fatal(err)
	}
	if d.Settings["vendor"] != "nextcloud" || d.Settings["url"] != "https://cloud" || d.Backend != "webdav" {
		t.Fatalf("draft = %+v", d)
	}
	box, _ := remotes.FindProvider("storagebox")
	d, err = svc.draftFor(box, map[string]string{"pass": "typed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Settings["pass"]; ok || d.Settings["key_file"] != svc.sshKeyPath() || d.Settings["port"] != "23" {
		t.Fatalf("storage box draft = %+v, want BombVault's key and port 23, no password", d)
	}
	rest, _ := remotes.FindProvider("rest")
	if _, err := svc.draftFor(rest, nil); err == nil {
		t.Fatal("a rest-server was turned into an rclone remote")
	}
}

func TestARepositoryIsUnderADestinationOnlyBelowIt(t *testing.T) {
	for _, c := range []struct {
		base, repo string
		want       bool
	}{
		{"s3:http://nas:9000/bv/dxp", "s3:http://nas:9000/bv/dxp/container", true},
		{"s3:http://nas:9000/bv/dxp/", "s3:http://nas:9000/bv/dxp/a/b", true},
		{"rclone:b2:", "rclone:b2:containers", true},
		{"s3:http://nas:9000/bv/dxp", "s3:http://nas:9000/bv/dxp", false},
		{"s3:http://nas:9000/bv/dxp", "s3:http://nas:9000/bv/dxp6800/container", false},
		{"rclone:b2:", "rclone:b2:", false},
	} {
		if got := repoUnder(c.base, c.repo); got != c.want {
			t.Errorf("repoUnder(%q, %q) = %v, want %v", c.base, c.repo, got, c.want)
		}
	}
}

func TestAHandTypedPrimaryUnderADestinationIsOfferedAndTakenOver(t *testing.T) {
	const repo = "s3:http://nas:9000/bv/dxp/container"
	svc, st, _ := newProbeSvc(t, map[string]bool{repo: true})
	if _, err := st.MutateSettings(func(s *store.Settings) error { s.ContainersOffsite = repo; return nil }); err != nil {
		t.Fatal(err)
	}
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary", Repo: repo, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.SaveDestination(store.OffsiteTarget{Name: "QNAP", Repo: "s3:http://nas:9000/bv/dxp"})
	if err != nil {
		t.Fatal(err)
	}

	v, err := svc.destinationView(d)
	if err != nil || len(v.Adoptable) != 1 || v.Adoptable[0].ID != primary.ID || !v.Adoptable[0].Primary {
		t.Fatalf("adoptable = %+v, %v, want the primary", v.Adoptable, err)
	}
	if _, err := svc.AdoptIntoDestination(context.Background(), d.ID, primary.ID); err != nil {
		t.Fatal(err)
	}
	v, err = svc.destinationView(d)
	if err != nil || len(v.Adoptable) != 0 || !slices.Equal(v.Domains, []string{"containers"}) {
		t.Fatalf("after adoption: domains %v, adoptable %+v, %v", v.Domains, v.Adoptable, err)
	}
	if s, _ := st.GetSettings(); s.ContainersOffsite != "" {
		t.Fatalf("containers off-site field = %q, want it cleared", s.ContainersOffsite)
	}
	if got := svc.offsiteReplicationTargets("containers", store.Settings{}); len(got) != 1 || got[0].Repo != repo {
		t.Fatalf("replication targets = %+v, want the adopted target on its repository", got)
	}
	// A page that still holds the old field must not turn the target back
	// into the primary.
	if msg, err := svc.rejectAdoptionOverOwnSettings(store.Settings{ContainersOffsite: repo}); err != nil || !strings.Contains(msg, "follows a destination") {
		t.Fatalf("saving the old field again: %q, %v", msg, err)
	}
}

func TestATakeoverThatWouldLoseProtectionOrCannotSignInIsRefused(t *testing.T) {
	const repo = "rest:http://box:8000/bv/vms"
	svc, st, _ := newProbeSvc(t, map[string]bool{})
	guarded, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "box", Repo: repo, Enabled: true, Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := st.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: "rest:http://box:8000/bv"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdoptIntoDestination(context.Background(), plain.ID, guarded.ID); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("taking an append-only target into a plain destination: %v", err)
	}
	locked, err := st.SaveDestination(store.OffsiteTarget{Name: "Locked box", Repo: "rest:http://box:8000/bv", Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdoptIntoDestination(context.Background(), locked.ID, guarded.ID); err == nil || !strings.Contains(err.Error(), "does not open") {
		t.Fatalf("taking over a repository the sign-in cannot open: %v", err)
	}
	if got, _, _ := st.GetOffsiteTarget(guarded.ID); got.DestinationID != "" || got.Name != "box" {
		t.Fatalf("a refused takeover changed the target: %+v", got)
	}
}
