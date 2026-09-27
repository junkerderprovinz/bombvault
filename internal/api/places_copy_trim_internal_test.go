package api

import (
	"context"
	"slices"
	"testing"
)

// A copy to a place leaves out what the place's keep-policy forgets right after
// it, the way a copy to a target without a place does, and a raised retention
// at the place fetches what the place now keeps.
func TestACopyToAPlaceLeavesOutWhatThePlaceForgets(t *testing.T) {
	f, target, s3, _ := trimScene(t)
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	placed := f.storedTarget(target.ID)
	if placed.PlaceID == "" || placed.RetentionKeepLast != 2 {
		t.Fatalf("S3 target = %+v, want it at a place that keeps the last two", placed)
	}

	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a2", "a3"}, {"c2", "c3"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("first pass copied %v, want %v", got, want)
	}
	replicate(t, f, "files")
	if got := copiedIDs(f, s3); len(got) != 0 {
		t.Fatalf("second pass copied %v again, which the place's keep-last 2 forgets right after", got)
	}

	keep := 3
	if _, _, err := f.svc.patchPlace(context.Background(), placed.PlaceID, patchPlaceBody{RetentionKeepLast: &keep}); err != nil {
		t.Fatalf("raise the place's retention: %v", err)
	}
	replicate(t, f, "files")
	if got, want := copiedIDs(f, s3), [][]string{{"a1"}, {"c1"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("after keep-last 3 at the place copied %v, want %v", got, want)
	}
}
