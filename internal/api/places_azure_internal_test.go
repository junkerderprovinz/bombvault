package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// azuriteKey is the published key of Azure's storage emulator, not a secret.
const azuriteKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// azureFake answers List Containers with its containers, or with the error
// code it holds, and keeps the last request's Authorization header and query.
type azureFake struct {
	mu         sync.Mutex
	code       string
	containers []string
	lastAuth   string
	lastQuery  string
}

func (a *azureFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.lastAuth, a.lastQuery = r.Header.Get("Authorization"), r.URL.RawQuery
	code, containers := a.code, a.containers
	a.mu.Unlock()
	if code != "" {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?><Error><Code>`+code+`</Code></Error>`)
		return
	}
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?><EnumerationResults><Containers>`)
	for _, name := range containers {
		_, _ = io.WriteString(w, "<Container><Name>"+name+"</Name></Container>")
	}
	_, _ = io.WriteString(w, "</Containers><NextMarker/></EnumerationResults>")
}

func (a *azureFake) seen() (auth, query string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastAuth, a.lastQuery
}

func newAzureFake(t *testing.T, code string, containers ...string) *azureFake {
	t.Helper()
	fake := &azureFake{code: code, containers: containers}
	srv := httptest.NewTLSServer(fake)
	t.Cleanup(srv.Close)
	usePlaceProbeClient(t, srv)
	prev := azureBlobURL
	azureBlobURL = srv.URL + "/"
	t.Cleanup(func() { azureBlobURL = prev })
	return fake
}

func TestSignSharedKeyMatchesTheAzureSDK(t *testing.T) {
	// Signed by azblob v1.6.1's SharedKeyCredential for the same request.
	for date, want := range map[string]string{
		"Thu, 24 Sep 2026 23:25:52 GMT": "SharedKey devstoreaccount1:EUezbvrNA2ZuZBQqVnE0yZN3ZuuWmPKi1u6bGDtia4k=",
		"Thu, 24 Sep 2026 23:28:03 GMT": "SharedKey devstoreaccount1:a+QMm6S9E3ohz3MpcEAOAcLUN/8GtEcPBROGVdaLQVw=",
	} {
		req, err := http.NewRequest(http.MethodGet, "https://devstoreaccount1.blob.core.windows.net/?comp=list", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "application/xml")
		req.Header.Set("x-ms-date", date)
		req.Header.Set("x-ms-version", "2025-05-05")
		if err := signSharedKey(req, "devstoreaccount1", azuriteKey); err != nil {
			t.Fatal(err)
		}
		if got := req.Header.Get("Authorization"); got != want {
			t.Errorf("%s: Authorization = %q\nwant %q", date, got, want)
		}
	}
}

func TestAnAzureAccountOffersItsContainers(t *testing.T) {
	fake := newAzureFake(t, "", "alpha", "beta")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "azure", map[string]string{"account": "acct", "secret": azuriteKey})
	if !res.OK || !slices.Equal(res.Buckets, []string{"alpha", "beta"}) || res.Base != "" || res.Folders != nil {
		t.Fatalf("probe = %+v, want the containers offered and nothing probed yet", res)
	}
	if auth, query := fake.seen(); !strings.HasPrefix(auth, "SharedKey acct:") || query != "comp=list" {
		t.Fatalf("request = %q with %q", query, auth)
	}
}

func TestARefusedAzureKeyFailsTheProbe(t *testing.T) {
	newAzureFake(t, "AuthenticationFailed")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "azure", map[string]string{"account": "acct", "secret": azuriteKey})
	if res.OK || res.Code != "place-probe-failed" || !strings.Contains(res.Error, "AuthenticationFailed") {
		t.Fatalf("probe = %+v, want the key refused", res)
	}
}

func TestAnAzureAccountNameThatCannotBeOneFailsTheProbe(t *testing.T) {
	newAzureFake(t, "", "alpha")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	for _, account := range []string{"Acct", "acct.example.com/x?", "ab"} {
		if res := probeOf(t, f, "azure", map[string]string{"account": account, "secret": azuriteKey}); res.OK || res.Code != "place-probe-failed" {
			t.Errorf("%q: probe = %+v", account, res)
		}
	}
}

func TestAnAzureProbeWithoutTheKeyNamesIt(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	_, err := f.svc.ProbePlace(context.Background(), ProbeRequest{Provider: "azure", Fields: map[string]string{"account": "acct"}})
	var missing places.MissingField
	if !errors.As(err, &missing) || missing != "secret" {
		t.Fatalf("err = %v, want the key named", err)
	}
}

func TestATypedAzureContainerIsOpenedWithoutAListing(t *testing.T) {
	fake := newAzureFake(t, "AuthenticationFailed")
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := probeOf(t, f, "azure", map[string]string{"account": "acct", "secret": azuriteKey, "container": "backups"})
	if res.Base != "azure:backups:" || res.Folders == nil {
		t.Fatalf("probe = %+v, want the typed container's folders probed", res)
	}
	if auth, _ := fake.seen(); auth != "" {
		t.Fatalf("the account was listed with %q", auth)
	}
}
