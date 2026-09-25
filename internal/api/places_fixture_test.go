package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// localPlace is a folder on this server offering every domain below base,
// each in a folder named after it.
func localPlace(name, base string) store.Place {
	return store.Place{Name: name, Provider: "unraid-folder", Kind: string(places.KindLocal), Base: base, Enabled: true,
		Folders: map[string]string{"containers": "containers", "vms": "vms", "flash": "flash", "config": "config", "files": "files"}}
}

// s3Place is a bucket offering every domain in its default folder.
func s3Place(name, base string) store.Place {
	return store.Place{Name: name, Provider: "s3-other", Kind: string(places.KindS3), Base: base, Enabled: true,
		Folders: places.DefaultFolders(), OffPremises: true}
}

// placeTarget adds an enabled target of the domain at the place as stored,
// and mirrors the place onto it.
func (f *placementFixture) placeTarget(p store.Place, domain, suffix string) store.OffsiteTarget {
	f.t.Helper()
	stored, err := f.st.GetPlace(p.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	addr, ok := places.Address(stored.Base, stored.Folders, domain, suffix)
	if !ok {
		f.t.Fatalf("place %s has no folder for %s", stored.Name, domain)
	}
	t := f.target(domain, stored.Name, addr)
	f.linkRow(t.ID, stored, domain, suffix)
	return f.storedTarget(t.ID)
}

// placeByName finds a place in a GET /api/places answer.
func placeByName(t *testing.T, res map[string]any, name string) map[string]any {
	t.Helper()
	for _, p := range rowsOf(res["places"]) {
		if p["name"] == name {
			return p
		}
	}
	t.Fatalf("no place %q in %v", name, res["places"])
	return nil
}

// rowsOfStrings is a JSON array of strings from an answer.
func rowsOfStrings(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// probeAnswers makes every place probe answer res and records what it was asked.
func (f *placementFixture) probeAnswers(res places.ProbeResult) *[]ProbeRequest {
	var asked []ProbeRequest
	f.svc.placeProber = func(_ context.Context, req ProbeRequest) (places.ProbeResult, error) {
		asked = append(asked, req)
		return res, nil
	}
	return &asked
}
