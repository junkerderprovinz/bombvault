package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nginxAtHomeAndB2 is nginx with a1 at home and its copy b1 at B2, a backup a7
// only B2 holds, and a snapshot of nginx2 beside them.
func nginxAtHomeAndB2(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("nginx2", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_000, "container:nginx"),
		snap("e1e1e1e1", 1_758_000_000, "container:nginx2"),
	)
	f.hold("b2:bucket:containers",
		copied("b1b1b1b1", "a1a1a1a1", 1_758_000_000, "container:nginx"),
		copied("b7b7b7b7", "a7a7a7a7", 1_757_000_000, "container:nginx"),
	)
	f.listing("containers", b2.ID, 1_758_100_000, copiesRow("container:nginx", 2, 1_758_000_000))
	return f, b2
}

func TestDeletePreviewAtOnePlaceNamesThePlacesThatStillHoldTheBackup(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	del, others, err := f.svc.timelineDeletePreview(context.Background(), "containers", "nginx", "a1a1a1a1", []string{"offsite:" + b2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if want := []placeDelete{{Place: "offsite:" + b2.ID, Label: "B2", SnapshotIDs: []string{"b1b1b1b1"}}}; !reflect.DeepEqual(del, want) {
		t.Fatalf("delete = %+v, want %+v", del, want)
	}
	if want := []otherPlace{{Place: "local", Label: "", State: "holds"}}; !slices.Equal(others, want) {
		t.Fatalf("others = %+v, want %+v", others, want)
	}
}

func TestDeletePreviewOfAWholeRowListsEveryPlaceOnce(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	del, others, err := f.svc.timelineDeletePreview(context.Background(), "containers", "nginx", "a1a1a1a1", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []placeDelete{
		{Place: "local", Label: "", SnapshotIDs: []string{"a1a1a1a1"}},
		{Place: "offsite:" + b2.ID, Label: "B2", SnapshotIDs: []string{"b1b1b1b1"}},
	}
	if !reflect.DeepEqual(del, want) || len(others) != 0 {
		t.Fatalf("delete = %+v, others %+v", del, others)
	}
	if n := f.eng.lists["b2:bucket:containers"]; n != 1 {
		t.Fatalf("the preview listed B2 %d times, want once", n)
	}
}

func TestDeletePreviewTellsTheLastCopyFromAPlaceItCouldNotRead(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	ctx := context.Background()
	at := []string{"offsite:" + b2.ID}

	_, others, err := f.svc.timelineDeletePreview(ctx, "containers", "nginx", "a7a7a7a7", at)
	if err != nil {
		t.Fatal(err)
	}
	if want := []otherPlace{{Place: "local", Label: "", State: "missing"}}; !slices.Equal(others, want) {
		t.Fatalf("others = %+v, want the home missing it", others)
	}

	f.eng.listErr[f.domainPath("containers")] = errors.New("permission denied")
	_, others, err = f.svc.timelineDeletePreview(ctx, "containers", "nginx", "a1a1a1a1", at)
	if err != nil {
		t.Fatal(err)
	}
	if want := []otherPlace{{Place: "local", Label: "", State: "unreadable"}}; !slices.Equal(others, want) {
		t.Fatalf("others = %+v, want the home unreadable", others)
	}
}

func TestTimelineDeleteForgetsThatPlacesIDsAndCorrectsWhatB2IsSeenToHold(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	deleted, skipped, err := f.svc.timelineDelete(context.Background(), "containers", "nginx", "a1a1a1a1",
		[]placeDelete{{Place: "offsite:" + b2.ID, SnapshotIDs: []string{"b1b1b1b1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []placeDelete{{Place: "offsite:" + b2.ID, Label: "B2", SnapshotIDs: []string{"b1b1b1b1"}}}; !reflect.DeepEqual(deleted, want) || len(skipped) != 0 {
		t.Fatalf("deleted = %+v, skipped %+v", deleted, skipped)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); !slices.Equal(got, []string{"b1b1b1b1"}) {
		t.Fatalf("forgotten at B2 = %v", got)
	}
	if got := forgottenAt(f, f.domainPath("containers")); len(got) != 0 {
		t.Fatalf("a delete at B2 reached the home: %v", got)
	}
	copies, err := f.st.ItemCopiesFor("containers", "container:nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 1 || copies[0].SnapshotCount != 1 || copies[0].LatestSnapshotAt != 1_757_000_000 {
		t.Fatalf("observed copies = %+v, want b7 left at B2", copies)
	}
}

func TestTimelineDeleteLeavesAppendOnlyPlacesOutAndNamesThem(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	f.appendOnly(b2.ID)
	ctx := context.Background()

	_, others, err := f.svc.timelineDeletePreview(ctx, "containers", "nginx", "a1a1a1a1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []otherPlace{{Place: "offsite:" + b2.ID, Label: "B2", State: "append-only"}}; !slices.Equal(others, want) {
		t.Fatalf("others = %+v, want B2 named as append-only", others)
	}
	deleted, skipped, err := f.svc.timelineDelete(ctx, "containers", "nginx", "a1a1a1a1", []placeDelete{
		{Place: "local", SnapshotIDs: []string{"a1a1a1a1"}},
		{Place: "offsite:" + b2.ID, SnapshotIDs: []string{"b1b1b1b1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0].Place != "local" {
		t.Fatalf("deleted = %+v, want the home only", deleted)
	}
	if want := []otherPlace{{Place: "offsite:" + b2.ID, Label: "B2", State: "append-only"}}; !slices.Equal(skipped, want) {
		t.Fatalf("skipped = %+v", skipped)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); len(got) != 0 {
		t.Fatalf("forgot at an append-only target: %v", got)
	}
}

func TestTimelineDeleteTakesOnlyWhatThePlaceHoldsOfTheRow(t *testing.T) {
	f, _ := nginxAtHomeAndB2(t)
	_, _, err := f.svc.timelineDelete(context.Background(), "containers", "nginx", "a1a1a1a1",
		[]placeDelete{{Place: "local", SnapshotIDs: []string{"a1a1a1a1", "e1e1e1e1", "0badc0de"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, f.domainPath("containers")); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("forgotten = %v, want a1 only", got)
	}
}

func TestTimelineDeleteOfAVMRunTakesItsDisksAtThatPlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.zvolVM("web")
	f.hold(f.domainPath("vms"),
		snap("a1a1a1a1", 1_758_000_000, "vm:web", "vmrun:r1"),
		snap("a2a2a2a2", 1_758_000_000, "vm:web:zvol:sda", "vmrun:r1"),
		snap("a3a3a3a3", 1_759_000_000, "vm:web", "vmrun:r2"),
		snap("a4a4a4a4", 1_759_000_000, "vm:web:zvol:sda", "vmrun:r2"),
	)
	ctx := context.Background()

	del, _, err := f.svc.timelineDeletePreview(ctx, "vms", "web", "a1a1a1a1", []string{"local"})
	if err != nil {
		t.Fatal(err)
	}
	if len(del) != 1 || !slices.Equal(del[0].SnapshotIDs, []string{"a1a1a1a1", "a2a2a2a2"}) {
		t.Fatalf("delete = %+v, want the run and its disk", del)
	}
	if _, _, err := f.svc.timelineDelete(ctx, "vms", "web", "a1a1a1a1", del); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, f.domainPath("vms")); !slices.Equal(got, []string{"a1a1a1a1", "a2a2a2a2"}) {
		t.Fatalf("forgotten = %v", got)
	}
}

func TestTimelineDeleteRoutes(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)

	m := f.do("GET", "/api/items/containers/nginx/timeline/a1a1a1a1/delete?place=offsite:"+b2.ID, nil)
	if m["ok"] != true || len(m["delete"].([]any)) != 1 || m["others"].([]any)[0].(map[string]any)["state"] != "holds" {
		t.Fatalf("preview = %v", m)
	}
	body := map[string]any{"places": []map[string]any{{"place": "offsite:" + b2.ID, "label": "B2", "snapshotIds": []string{"b1b1b1b1"}}}}
	m = f.do("DELETE", "/api/items/containers/nginx/timeline/a1a1a1a1", body)
	if m["ok"] != true || len(m["deleted"].([]any)) != 1 || len(m["skipped"].([]any)) != 0 {
		t.Fatalf("delete = %v", m)
	}

	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("could not take the domain lock")
	}
	m = f.do("DELETE", "/api/items/containers/nginx/timeline/a1a1a1a1", body)
	unlock()
	if m["ok"] != false || m["code"] != "domain-busy" {
		t.Fatalf("delete while a backup runs = %v, want domain-busy", m)
	}

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/items/containers/nginx/timeline/nothex/delete", ""},
		{http.MethodGet, "/api/items/containers/nginx/timeline/a1a1a1a1/delete?place=offsite:NOPE", ""},
		{http.MethodDelete, "/api/items/containers/nginx/timeline/a1a1a1a1", `{"places":[{"place":"local","snapshotIds":["nothex"]}]}`},
	} {
		var body io.Reader
		if c.body != "" {
			body = strings.NewReader(c.body)
		}
		rec := httptest.NewRecorder()
		f.h.Router().ServeHTTP(rec, jsonReq(c.method, c.path, body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s: status %d, want 400", c.method, c.path, rec.Code)
		}
	}
}
