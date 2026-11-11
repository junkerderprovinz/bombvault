package api

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

const knightLoaderContainer = "4f3a9c21b7e05d6a8c1f2e3d4b5a69788f7e6d5c4b3a29180f1e2d3c4b5a6978"

// fakeExporter is a fortress with one KnightLoader that runs in
// knightLoaderContainer and hands over a backup.
type fakeExporter struct {
	members  []fortress.Member
	reported map[string]string
	exported []string
	fail     error
}

func newFakeExporter() *fakeExporter {
	return &fakeExporter{
		members: []fortress.Member{
			{ID: "t1", App: fortress.App{ID: "towertorch"}, Direct: true},
			{ID: "k1", App: fortress.App{ID: "knightloader", Name: "KnightLoader"}, Direct: true, Actions: []string{"backup.export"}},
		},
		reported: map[string]string{"t1": "", "k1": knightLoaderContainer[:12]},
	}
}

func (f *fakeExporter) Members() []fortress.Member { return f.members }

func (f *fakeExporter) Status(_ context.Context, id, _ string) (status.Report, error) {
	return status.Report{Instance: status.Instance{Container: f.reported[id]}}, nil
}

func (f *fakeExporter) Export(_ context.Context, id string) (io.ReadCloser, error) {
	f.exported = append(f.exported, id)
	if f.fail != nil {
		return nil, f.fail
	}
	return io.NopCloser(strings.NewReader("the app's own backup")), nil
}

// stdinEngine records what reaches restic backup --stdin.
type stdinEngine struct {
	ResticEngine
	repo, path string
	tags       []string
	body       string
}

func (e *stdinEngine) BackupStdin(_ context.Context, repo string, rd io.Reader, path string, tags []string, _ restic.Mode) (restic.Summary, error) {
	b, err := io.ReadAll(rd)
	e.repo, e.path, e.tags, e.body = repo, path, tags, string(b)
	return restic.Summary{}, err
}

func TestTheAppInAContainerHandsItsOwnBackupOverBeforeTheBackup(t *testing.T) {
	svc, _ := newFortressSvc(t)
	eng := &stdinEngine{}
	svc.engine = eng
	ex := newFakeExporter()

	if err := svc.storeAppExportFrom(context.Background(), ex, "knightloader", knightLoaderContainer, "/backups/containers", restic.Mode{Encrypted: true}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ex.exported, []string{"k1"}) {
		t.Fatalf("fetched from %v, want k1", ex.exported)
	}
	if eng.repo != "/backups/containers" || eng.body != "the app's own backup" || eng.path != "/appexport/knightloader/knightloader" {
		t.Fatalf("stored %+v", eng)
	}
	if want := []string{"appexport:knightloader", "app:knightloader", "appname:KnightLoader"}; !slices.Equal(eng.tags, want) {
		t.Fatalf("tags = %v, want %v", eng.tags, want)
	}
}

func TestAnAppNameThatWouldSplitIntoTagsIsLeftOut(t *testing.T) {
	svc, _ := newFortressSvc(t)
	eng := &stdinEngine{}
	svc.engine = eng
	ex := newFakeExporter()
	ex.members[1].App.Name = "Knight,Loader"

	if err := svc.storeAppExportFrom(context.Background(), ex, "knightloader", knightLoaderContainer, "/backups/containers", restic.Mode{Encrypted: true}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"appexport:knightloader", "app:knightloader"}; !slices.Equal(eng.tags, want) {
		t.Fatalf("tags = %v, want %v", eng.tags, want)
	}
}

func TestAnAppExportStaysOutOfAnUnencryptedRepository(t *testing.T) {
	svc, _ := newFortressSvc(t)
	eng := &stdinEngine{}
	svc.engine = eng
	ex := newFakeExporter()

	err := svc.storeAppExportFrom(context.Background(), ex, "knightloader", knightLoaderContainer, "/backups/containers", restic.Mode{})
	if !errors.Is(err, errExportUnencrypted) {
		t.Fatalf("got %v, want the unencrypted refusal", err)
	}
	if len(ex.exported) != 0 || eng.repo != "" {
		t.Fatalf("fetched %v and stored %+v", ex.exported, eng)
	}
}

func TestAContainerNoMemberRunsInBacksUpAsBefore(t *testing.T) {
	svc, _ := newFortressSvc(t)
	eng := &stdinEngine{}
	svc.engine = eng
	ex := newFakeExporter()

	if err := svc.storeAppExportFrom(context.Background(), ex, "plex", strings.Repeat("9", 64), "/backups/containers", restic.Mode{Encrypted: true}); err != nil {
		t.Fatal(err)
	}
	ex.members[1].Direct = false
	if err := svc.storeAppExportFrom(context.Background(), ex, "knightloader", knightLoaderContainer, "/backups/containers", restic.Mode{Encrypted: true}); err != nil {
		t.Fatal(err)
	}
	if len(ex.exported) != 0 || eng.repo != "" {
		t.Fatalf("fetched %v and stored %+v", ex.exported, eng)
	}
}

func TestAFailedExportIsSaidAndStoresNothing(t *testing.T) {
	svc, _ := newFortressSvc(t)
	eng := &stdinEngine{}
	svc.engine = eng
	ex := newFakeExporter()
	ex.fail = &fortress.Refusal{Status: 503, Code: "busy"}

	if err := svc.storeAppExportFrom(context.Background(), ex, "knightloader", knightLoaderContainer, "/backups/containers", restic.Mode{Encrypted: true}); err == nil {
		t.Fatal("a refused export read as stored")
	}
	if eng.repo != "" {
		t.Fatalf("stored %+v", eng)
	}
}
