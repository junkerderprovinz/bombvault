package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestVolumeSamplesRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	total := int64(4 << 40)
	samples := []store.VolumeSample{
		{Volume: "pool:tank", At: 300, FreeBytes: 700, TotalBytes: &total, Domains: []string{"containers", "vms"}, Source: "statfs"},
		{Volume: "pool:tank", At: 100, FreeBytes: 900, TotalBytes: &total, Domains: []string{"containers"}, Source: "statfs"},
		{Volume: "remote:8f1c", At: 200, FreeBytes: 500, Domains: []string{"files"}, Source: "rclone-about"},
	}
	for _, s := range samples {
		if err := r.AddVolumeSample(s); err != nil {
			t.Fatalf("AddVolumeSample: %v", err)
		}
	}

	got, err := r.ListVolumeSamples(0)
	if err != nil {
		t.Fatalf("ListVolumeSamples: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("read %d samples", len(got))
	}
	if got[0].At != 100 || got[2].At != 300 {
		t.Fatalf("samples are not oldest first: %d … %d", got[0].At, got[2].At)
	}

	remote := got[1]
	if remote.Volume != "remote:8f1c" || remote.Source != "rclone-about" || remote.FreeBytes != 500 {
		t.Fatalf("the remote sample reads as %+v", remote)
	}
	if remote.TotalBytes != nil {
		t.Fatalf("a backend that reports no size got a total of %d", *remote.TotalBytes)
	}
	if len(remote.Domains) != 1 || remote.Domains[0] != "files" {
		t.Fatalf("domains are %v", remote.Domains)
	}
	if got[2].TotalBytes == nil || *got[2].TotalBytes != total {
		t.Fatalf("the local sample lost its total: %+v", got[2])
	}
	if len(got[2].Domains) != 2 {
		t.Fatalf("a volume that holds two domains reads as %v", got[2].Domains)
	}

	since, err := r.ListVolumeSamples(200)
	if err != nil {
		t.Fatalf("ListVolumeSamples since: %v", err)
	}
	if len(since) != 2 {
		t.Fatalf("the window from 200 holds %d samples", len(since))
	}

	n, err := r.PruneVolumeSamples(250)
	if err != nil {
		t.Fatalf("PruneVolumeSamples: %v", err)
	}
	if n != 2 {
		t.Fatalf("pruned %d samples, want the two before the cutoff", n)
	}
	left, err := r.ListVolumeSamples(0)
	if err != nil {
		t.Fatalf("ListVolumeSamples after pruning: %v", err)
	}
	if len(left) != 1 || left[0].At != 300 {
		t.Fatalf("what is left is %+v", left)
	}
}
