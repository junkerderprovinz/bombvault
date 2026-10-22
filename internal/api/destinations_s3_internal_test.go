package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestABucketAddressPastedAsTheEndpointReachesRcloneAsTheServer(t *testing.T) {
	p, _ := remotes.FindProvider("r2")
	d, err := new(Service).draftFor(p, map[string]string{"endpoint": "https://acct.r2.cloudflarestorage.com/bucket"})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Settings["endpoint"]; got != "https://acct.r2.cloudflarestorage.com" {
		t.Fatalf("endpoint = %q, want the server without the bucket", got)
	}
}

func TestAnAWSBucketIsCreatedInTheRegionTheFormNames(t *testing.T) {
	p, _ := remotes.FindProvider("aws")
	for region, want := range map[string]string{"eu-central-1": "eu-central-1", "us-east-1": "", "": ""} {
		d, err := new(Service).draftFor(p, map[string]string{"region": region})
		if err != nil {
			t.Fatal(err)
		}
		if got := d.Settings["location_constraint"]; got != want {
			t.Errorf("region %q: location_constraint = %q, want %q", region, got, want)
		}
	}
	d, _ := new(Service).draftFor(p, map[string]string{"region": "eu-central-1", "location_constraint": "EU"})
	if got := d.Settings["location_constraint"]; got != "EU" {
		t.Errorf("a typed location constraint became %q", got)
	}
}

func TestTheRegionComesFromAnEndpointThatNamesIt(t *testing.T) {
	b2, _ := remotes.FindProvider("b2")
	d, err := new(Service).draftFor(b2, map[string]string{"endpoint": "s3.eu-central-003.backblazeb2.com"})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Settings["region"]; got != "eu-central-003" {
		t.Fatalf("region = %q, want eu-central-003", got)
	}
	d, _ = new(Service).draftFor(b2, map[string]string{"endpoint": "s3.eu-central-003.backblazeb2.com", "region": "typed"})
	if got := d.Settings["region"]; got != "typed" {
		t.Fatalf("a typed region became %q", got)
	}

	for host, want := range map[string]string{
		"s3.us-west-004.backblazeb2.com": "us-west-004",
		"fsn1.your-objectstorage.com":    "fsn1",
		"s3.fr-par.scw.cloud":            "fr-par",
		"s3.gra.io.cloud.ovh.net":        "gra",
		"s3.eu-central-2.wasabisys.com":  "eu-central-2",
		"s3.eu-west-1.amazonaws.com":     "eu-west-1",
		"garage.lan":                     "",
		"s3.wasabisys.com":               "",
	} {
		if got := s3RegionOf(host); got != want {
			t.Errorf("s3RegionOf(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestANewS3DestinationStoresTheRegionItsEndpointNames(t *testing.T) {
	svc, _, _ := newProbeSvc(t, nil)
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "b2", Dir: "bucket",
		Settings: map[string]string{"endpoint": "s3.eu-central-003.backblazeb2.com", "access_key_id": "K", "secret_access_key": "S"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := signInOf(t, svc, d); got.S3Region != "eu-central-003" {
		t.Fatalf("sign-in region = %q, want eu-central-003", got.S3Region)
	}
}

func TestAStorageClassTheProviderDoesNotTakeIsRefused(t *testing.T) {
	svc, _, _ := newProbeSvc(t, nil)
	save := func(provider, class string) error {
		_, err := svc.SaveNewDestination(context.Background(), draftRequest{
			Provider: provider, Dir: strings.ToLower(provider + "-" + class), StorageClass: class,
			Settings: map[string]string{"endpoint": "s3.example.com", "access_key_id": "K", "secret_access_key": "S"},
		})
		return err
	}
	for _, c := range []struct {
		provider, class string
		ok              bool
	}{
		{"silo", "STANDARD_IA", false},
		{"r2", "GLACIER_IR", false},
		{"r2", "STANDARD_IA", true},
		{"aws", "GLACIER_IR", true},
		{"silo", "", true},
	} {
		if err := save(c.provider, c.class); (err == nil) != c.ok {
			t.Errorf("%s with %q: err = %v, want accepted %v", c.provider, c.class, err, c.ok)
		}
	}
}

func TestEveryProvidersStorageClassIsOneResticRestoresFrom(t *testing.T) {
	for _, p := range remotes.Providers() {
		for _, c := range p.StorageClasses {
			if !restic.StorageClassAllowed(c) {
				t.Errorf("%s offers %s", p.ID, c)
			}
		}
	}
}

func TestADestinationKeepsAClassItHadWhenItsOtherSettingsChange(t *testing.T) {
	f := newPlacementFixture(t)
	d, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Silo", Provider: "silo", Repo: "s3:https://silo.lan/bucket", StorageClass: "STANDARD_IA"})
	if err != nil {
		t.Fatal(err)
	}
	put := func(body map[string]any) map[string]any {
		_, out := f.call(http.MethodPut, "/api/offsite/destinations/"+d.ID, body)
		return out
	}
	if got := put(map[string]any{"name": "Silo 2", "storageClass": "STANDARD_IA"}); got["ok"] != true {
		t.Fatalf("renaming = %v, want the stored class kept", got)
	}
	if got := put(map[string]any{"name": "Silo 2", "storageClass": "GLACIER_IR"}); got["ok"] == true || !strings.Contains(got["error"].(string), "Silo") {
		t.Fatalf("a new class Silo does not take = %v, want it refused", got)
	}
	if got := put(map[string]any{"name": "Silo 2", "storageClass": ""}); got["ok"] != true {
		t.Fatalf("back to the default = %v", got)
	}
}

func TestADestinationsTargetUsesTheProvidersDefaultWhenTheDestinationDoes(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	ref, err := svc.addCredSet("Wasabi", CloudCreds{S3KeyID: "K", S3Secret: "S", S3StorageClass: "STANDARD_IA"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.SaveDestination(store.OffsiteTarget{Name: "Wasabi", Provider: "wasabi", Repo: "s3:https://s3.wasabisys.com/bucket", CredsRef: ref})
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := st.EnsureDestinationTarget(d.ID, "files", destinationLocation(d.Repo, "files"))
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.offsiteModeForTarget(settingsOf(t, svc), target).StorageClass; got != "" {
		t.Fatalf("storage class = %q, want the provider's default", got)
	}

	own, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "Typed", Repo: "s3:https://s3.wasabisys.com/b2", CredsRef: ref, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.offsiteModeForTarget(settingsOf(t, svc), own).StorageClass; got != "STANDARD_IA" {
		t.Fatalf("a target of its own uses %q, want its sign-in's class", got)
	}
}

func TestANewS3DestinationKeepsItsClassOutOfItsSignIn(t *testing.T) {
	svc, _, _ := newProbeSvc(t, nil)
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "aws", Dir: "bucket", StorageClass: "STANDARD_IA",
		Settings: map[string]string{"region": "eu-west-1", "access_key_id": "K", "secret_access_key": "S"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := signInOf(t, svc, d); got.S3StorageClass != "" {
		t.Fatalf("sign-in class = %q, want none", got.S3StorageClass)
	}
}

func TestASavedSignInDoesNotSignInForAnotherRegion(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "aws", Name: "AWS", CloudCreds: CloudCreds{S3KeyID: "K", S3Secret: "S", S3Region: "us-east-1"}}}); err != nil {
		t.Fatal(err)
	}
	other := draftRequest{Provider: "aws", CredsRef: "aws", Settings: map[string]string{"region": "eu-central-1"}}
	if err := s.signInFromSaved(&other); err == nil || !strings.Contains(err.Error(), "us-east-1") {
		t.Fatalf("err = %v, want the saved region named", err)
	}
	same := draftRequest{Provider: "aws", CredsRef: "aws", Settings: map[string]string{"region": "US-EAST-1"}}
	if err := s.signInFromSaved(&same); err != nil {
		t.Fatal(err)
	}
	empty := draftRequest{Provider: "aws", CredsRef: "aws"}
	if err := s.signInFromSaved(&empty); err != nil || empty.Settings["region"] != "us-east-1" {
		t.Fatalf("region = %q, %v, want the saved one", empty.Settings["region"], err)
	}
}

func TestEditingASharedSignInOnADestinationLeavesTheOthersAlone(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, func(string, []string) error { return nil })
	if err := svc.SetCloudCredSets([]CloudCredSet{{ID: "hetzner", Name: "Hetzner", CloudCreds: CloudCreds{S3KeyID: "K", S3Secret: "S", S3Region: "fsn1"}}}); err != nil {
		t.Fatal(err)
	}
	save := func(name, host string) store.OffsiteTarget {
		d, err := svc.SaveNewDestination(context.Background(), draftRequest{
			Provider: "hetznerobj", Name: name, Dir: "bucket", CredsRef: "hetzner",
			Settings: map[string]string{"endpoint": host, "access_key_id": "K", "secret_access_key": "S"},
		})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	a, b := save("A", "fsn1.your-objectstorage.com"), save("B", "hel1.your-objectstorage.com")
	if _, _, err := st.EnsureDestinationTarget(a.ID, "files", destinationLocation(a.Repo, "files")); err != nil {
		t.Fatal(err)
	}

	edited, _, err := svc.EditDestinationConnection(context.Background(), a.ID, connectionEdit{
		Settings: map[string]string{"access_key_id": "K2", "secret_access_key": "S2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if edited.CredsRef == "hetzner" || signInOf(t, svc, edited).S3KeyID != "K2" {
		t.Fatalf("A signs in with %q, want a sign-in of its own with the new key", edited.CredsRef)
	}
	if got := signInOf(t, svc, b); got.S3KeyID != "K" || got.S3Region != "fsn1" {
		t.Fatalf("B's sign-in = %+v, want it as it was", got)
	}

	again, _, err := svc.EditDestinationConnection(context.Background(), edited.ID, connectionEdit{
		Settings: map[string]string{"secret_access_key": "S3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.CredsRef != edited.CredsRef || len(savedSets(t, svc)) != 2 {
		t.Fatalf("a sign-in only A uses was copied again: %q, %d sets", again.CredsRef, len(savedSets(t, svc)))
	}
}

func TestAnS3SourceInAnotherRegionThanTheDestinationIsReadThroughRclone(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := namedSource(t, svc, st, "s3:https://s3.eu-west-1.amazonaws.com/a/bv", CloudCreds{S3KeyID: "K", S3Secret: "S", S3Region: "eu-west-1"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=K", "AWS_SECRET_ACCESS_KEY=S", "AWS_DEFAULT_REGION=us-east-2"}}

	loc, mode, err := svc.sourceAccess(settings, "containers", src, "s3:https://s3.us-east-2.amazonaws.com/b/bv", dest)
	if err != nil {
		t.Fatal(err)
	}
	if loc != "rclone:BVSRC:a/bv" || !slices.Contains(mode.Env, "RCLONE_CONFIG_BVSRC_REGION=eu-west-1") {
		t.Fatalf("got %q %v, want the source through rclone with its own region", loc, mode.Env)
	}
}

func TestAnAWSSourceWithNoRegionLetsRcloneFollowTheBucket(t *testing.T) {
	_, env, err := s3ThroughRclone("s3:https://s3.amazonaws.com/bucket/bv", CloudCreds{S3KeyID: "K", S3Secret: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "RCLONE_CONFIG_BVSRC_PROVIDER=AWS") {
		t.Fatalf("env = %v, want the AWS provider", env)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "RCLONE_CONFIG_BVSRC_ENDPOINT=") || strings.HasPrefix(e, "RCLONE_CONFIG_BVSRC_REGION=") {
			t.Fatalf("env = %v, want neither an endpoint nor a region, so rclone switches to the bucket's", env)
		}
	}
}

func TestAnS3SourceIsReadWithoutAskingForItsBucket(t *testing.T) {
	_, env, err := s3ThroughRclone("s3:https://acct.r2.cloudflarestorage.com/bucket/bv", CloudCreds{S3KeyID: "K", S3Secret: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "RCLONE_CONFIG_BVSRC_NO_CHECK_BUCKET=true") {
		t.Fatalf("env = %v, want the bucket check off", env)
	}
}

func TestADestinationListsTheStorageClassesItsProviderTakes(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	for provider, want := range map[string][]string{
		"r2":       {"STANDARD", "STANDARD_IA"},
		"silo":     {},
		"vanished": restic.AllowedStorageClasses,
		"onedrive": restic.AllowedStorageClasses,
	} {
		d, err := st.SaveDestination(store.OffsiteTarget{Name: provider, Provider: provider, Repo: "s3:https://" + provider + ".lan/bucket"})
		if err != nil {
			t.Fatal(err)
		}
		v, err := svc.destinationView(d)
		if err != nil || !slices.Equal(v.StorageClasses, want) {
			t.Errorf("%s: classes = %v, %v, want %v", provider, v.StorageClasses, err, want)
		}
	}
}
