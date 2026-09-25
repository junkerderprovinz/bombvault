package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// b2Fake stands in for Backblaze: its sign-in answers with the allowed block it
// holds and names its own address as the S3 endpoint, whose root answers
// ListBuckets. Only the application key "app-key" signs in.
type b2Fake struct {
	allowed string
	buckets []string
	s3URL   string
}

func (f *b2Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/b2api/v4/b2_authorize_account" {
		if _, pass, _ := r.BasicAuth(); pass != "app-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"accountId":"acc","authorizationToken":"t","apiInfo":{"storageApi":{"apiUrl":"https://api000.example","s3ApiUrl":"`+f.s3URL+`","allowed":`+f.allowed+`}}}`)
		return
	}
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult><Buckets>`)
	for _, name := range f.buckets {
		_, _ = io.WriteString(w, "<Bucket><Name>"+name+"</Name></Bucket>")
	}
	_, _ = io.WriteString(w, "</Buckets></ListAllMyBucketsResult>")
}

func newB2Fake(t *testing.T, allowed string, buckets ...string) *httptest.Server {
	t.Helper()
	fake := &b2Fake{allowed: allowed, buckets: buckets}
	srv := httptest.NewUnstartedServer(fake)
	// Set before the server starts, so no request can read it half written.
	fake.s3URL = "https://" + srv.Listener.Addr().String()
	srv.StartTLS()
	t.Cleanup(srv.Close)
	usePlaceProbeClient(t, srv)
	prev := b2AuthorizeURL
	b2AuthorizeURL = srv.URL + "/b2api/v4/b2_authorize_account"
	t.Cleanup(func() { b2AuthorizeURL = prev })
	return srv
}

func TestAB2KeyForOneBucketNeedsNothingTyped(t *testing.T) {
	srv := newB2Fake(t, `{"buckets":[{"id":"b1","name":"tower-backups"}],"capabilities":["listFiles","writeFiles"],"namePrefix":"bombvault/"}`)
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	base := "s3:" + srv.URL + "/tower-backups/bombvault"
	for _, folder := range places.DefaultFolders() {
		f.eng.opens[base+"/"+folder] = false
	}
	res := probeOf(t, f, "b2", map[string]string{"keyId": "0012ab", "secret": "app-key"})
	if !res.OK || res.Base != base {
		t.Fatalf("probe = %+v, want %s", res, base)
	}
	if res.Fields["bucket"] != "tower-backups" || res.Fields["path"] != "bombvault" || res.Fields["endpoint"] != srv.URL {
		t.Errorf("fields = %v", res.Fields)
	}
	if _, leaked := res.Fields["secret"]; leaked {
		t.Error("the application key came back in the fields")
	}
	want := []places.ProbeFact{
		{Key: places.FactB2Prefix, Params: map[string]string{"prefix": "bombvault"}},
		{Key: places.FactB2Bucket, Params: map[string]string{"bucket": "tower-backups"}},
	}
	if !reflect.DeepEqual(res.Facts, want) {
		t.Errorf("facts = %v\nwant %v", res.Facts, want)
	}
	if env := eng.env(base + "/container"); !slices.Contains(env, "AWS_ACCESS_KEY_ID=0012ab") {
		t.Errorf("the folders were opened with %v", env)
	}
}

func TestAB2KeyForEveryBucketOffersTheList(t *testing.T) {
	newB2Fake(t, `{"buckets":null,"capabilities":["listBuckets"],"namePrefix":null}`, "alpha", "beta")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "b2", map[string]string{"keyId": "0012ab", "secret": "app-key"})
	if !res.OK || !slices.Equal(res.Buckets, []string{"alpha", "beta"}) || res.Base != "" {
		t.Fatalf("probe = %+v, want the buckets offered", res)
	}
}

func TestAB2KeyLimitedToOtherBucketsFailsTheProbe(t *testing.T) {
	newB2Fake(t, `{"buckets":[{"id":"b1","name":"tower-backups"}],"namePrefix":null}`)
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "b2", map[string]string{"keyId": "0012ab", "secret": "app-key", "bucket": "other"})
	if res.OK || res.Code != "place-probe-failed" {
		t.Fatalf("probe = %+v, want the bucket refused", res)
	}
}

func TestARefusedB2KeyFailsTheProbe(t *testing.T) {
	newB2Fake(t, `{"buckets":null}`)
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "b2", map[string]string{"keyId": "0012ab", "secret": "wrong"})
	if res.OK || res.Code != "place-probe-failed" {
		t.Fatalf("probe = %+v, want the key refused", res)
	}
}

func TestUnderPrefixKeepsAPathInsideTheKeysFolder(t *testing.T) {
	for _, c := range []struct{ prefix, path, want string }{
		{"bombvault", "", "bombvault"},
		{"bombvault", "bombvault/tower", "bombvault/tower"},
		{"bombvault", "/tower/", "bombvault/tower"},
		{"bombvault", "bombvault", "bombvault"},
	} {
		if got := underPrefix(c.prefix, c.path); got != c.want {
			t.Errorf("underPrefix(%q, %q) = %q, want %q", c.prefix, c.path, got, c.want)
		}
	}
}
