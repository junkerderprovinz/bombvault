package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestItemBackupsSeesASuccessfulRun(t *testing.T) {
	f := newPlacementFixture(t)
	tg := f.container("web", "")
	f.backupRun(tg.ID, 100)
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || got != backupsPresent {
		t.Fatalf("itemBackups = %v, %v, want present", got, err)
	}
}

func TestItemBackupsOfAnOpenItemReadsTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	f.openContainer("nginx")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:nginx"))
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "containers", Key: "nginx"})
	if err != nil || got != backupsPresent {
		t.Fatalf("itemBackups = %v, %v, want present", got, err)
	}
}

func TestItemBackupsKeepsAnUnreadableLocationApart(t *testing.T) {
	f := newPlacementFixture(t)
	f.openVM("win11")
	f.eng.listErr = map[string]error{f.domainPath("vms"): errors.New("wrong password or no key found")}
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "vms", Key: "win11"})
	if got != backupsUnreadable || err == nil {
		t.Fatalf("itemBackups = %v, %v, want unreadable with its cause", got, err)
	}
	if had, err := f.svc.vmHasBackups(context.Background(), "win11"); err != nil || !had {
		t.Fatalf("vmHasBackups = %v, %v, want an unreadable location to count as backed up", had, err)
	}
}

func TestItemBackupsOfAChosenItemReadsOnlyItsOwnRepository(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("web", nas.ID)
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:web"))
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || got != backupsNone {
		t.Fatalf("itemBackups = %v, %v, want none on NAS", got, err)
	}
	if n := f.eng.lists[f.domainPath("containers")]; n != 0 {
		t.Errorf("the domain path was listed %d times for an item on NAS", n)
	}
}

func TestItemBackupsOfAMissingFileSetIsAnError(t *testing.T) {
	f := newPlacementFixture(t)
	_, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "files", Key: "no-such-set"})
	if !errors.Is(err, errFileSetNotFound) {
		t.Fatalf("err = %v, want errFileSetNotFound", err)
	}
}
