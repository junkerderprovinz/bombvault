package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// s3Fake answers ListBuckets with its buckets, or with the S3 error code it
// holds, and keeps the Authorization header of the last request.
type s3Fake struct {
	mu       sync.Mutex
	code     string
	buckets  []string
	lastAuth string
}

func (s *s3Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.lastAuth = r.Header.Get("Authorization")
	code, buckets := s.code, s.buckets
	s.mu.Unlock()
	if code != "" {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>`+code+`</Code></Error>`)
		return
	}
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult><Buckets>`)
	for _, name := range buckets {
		_, _ = io.WriteString(w, "<Bucket><Name>"+name+"</Name></Bucket>")
	}
	_, _ = io.WriteString(w, "</Buckets></ListAllMyBucketsResult>")
}

func (s *s3Fake) auth() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAuth
}

// usePlaceProbeClient sends the probes' own requests to srv for one test.
func usePlaceProbeClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	prev := placeProbeHTTPClient
	placeProbeHTTPClient = srv.Client()
	t.Cleanup(func() { placeProbeHTTPClient = prev })
}

func newS3Fake(t *testing.T, code string, buckets ...string) (*httptest.Server, *s3Fake) {
	t.Helper()
	fake := &s3Fake{code: code, buckets: buckets}
	srv := httptest.NewTLSServer(fake)
	t.Cleanup(srv.Close)
	usePlaceProbeClient(t, srv)
	return srv, fake
}

func TestSignV4MatchesAWSsWorkedExample(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-9")
	signV4(req, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20130524/us-east-1/s3/aws4_request, " +
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, " +
		"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization = %q\nwant %q", got, want)
	}
}

func TestAKeyThatMayListOffersItsBuckets(t *testing.T) {
	srv, fake := newS3Fake(t, "", "alpha", "beta")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "garage", map[string]string{"endpoint": srv.URL, "keyId": "KEY", "secret": "SECRET", "region": "garage"})
	if !res.OK || !slices.Equal(res.Buckets, []string{"alpha", "beta"}) || res.Base != "" || res.Folders != nil {
		t.Fatalf("probe = %+v, want the buckets offered and nothing probed yet", res)
	}
	if auth := fake.auth(); !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=KEY/") || !strings.Contains(auth, "/garage/s3/aws4_request") {
		t.Fatalf("Authorization = %q", auth)
	}
}

func TestAKeyThatMayNotListIsAskedForItsBucket(t *testing.T) {
	srv, _ := newS3Fake(t, "AccessDenied")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	fields := map[string]string{"endpoint": srv.URL, "keyId": "KEY", "secret": "SECRET"}
	res := probeOf(t, f, "minio", fields)
	if !res.OK || res.Buckets != nil || !reflect.DeepEqual(res.Facts, []places.ProbeFact{{Key: places.FactBucketsHidden}}) {
		t.Fatalf("probe = %+v, want the bucket asked for", res)
	}
	fields["bucket"] = "bv"
	res = probeOf(t, f, "minio", fields)
	if res.Base != "s3:"+srv.URL+"/bv" || res.Folders == nil {
		t.Fatalf("probe = %+v, want the typed bucket probed", res)
	}
}

func TestARefusedKeyFailsTheProbe(t *testing.T) {
	srv, _ := newS3Fake(t, "InvalidAccessKeyId")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "minio", map[string]string{"endpoint": srv.URL, "keyId": "KEY", "secret": "SECRET", "bucket": "bv"})
	if res.OK || res.Code != "place-probe-failed" || res.Folders != nil {
		t.Fatalf("probe = %+v, want the key refused", res)
	}
}

func TestABucketTheKeyDoesNotSeeIsNamedAsNew(t *testing.T) {
	// A new account's key lists no bucket at all.
	for _, seen := range [][]string{{"alpha"}, nil} {
		srv, _ := newS3Fake(t, "", seen...)
		f := newPlacementFixture(t)
		newEnvEngine(f)
		res := probeOf(t, f, "minio", map[string]string{"endpoint": srv.URL, "keyId": "KEY", "secret": "SECRET", "bucket": "fresh"})
		want := places.ProbeFact{Key: places.FactBucketNew, Params: map[string]string{"bucket": "fresh"}}
		if !slices.ContainsFunc(res.Facts, func(f places.ProbeFact) bool { return f.Key == want.Key && f.Params["bucket"] == "fresh" }) {
			t.Fatalf("key seeing %v: facts = %v, want %v", seen, res.Facts, want)
		}
		if res.Base != "s3:"+srv.URL+"/fresh" {
			t.Fatalf("base = %q", res.Base)
		}
	}
}
