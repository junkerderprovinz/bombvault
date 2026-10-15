package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// zfsSpaceSSH answers the host's ZFS listing with list and every other
// command with nothing.
type zfsSpaceSSH struct {
	zvolTPMSSH
	list string
}

// unreadable makes the listing fail, as a host without zfs on its path does.
const unreadable = "unreadable"

func (s *zfsSpaceSSH) Run(_ context.Context, args ...string) (string, error) {
	if len(args) > 1 && args[0] == "zfs" && args[1] == "list" {
		if s.list == unreadable {
			return "", errors.New("zfs: command not found")
		}
		return s.list, nil
	}
	return "", nil
}

const gib = int64(1) << 30

// checkZvolRestore checks the in-place restore of the zvol-only VM testvm,
// whose 1 GiB zvol tank/vms/testvm is backed up, on a host that lists list.
func checkZvolRestore(t *testing.T, list string) api.RestoreCheck {
	t.Helper()
	svc, eng, _, root := vmZvolTestService(t, zvolOnlyVMDomainXML, nil)
	svc.SetHostSSH(&zfsSpaceSSH{list: list})
	if _, err := svc.BackupVM(context.Background(), "testvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	zvolPath, zvolTags, _ := strings.Cut(eng.stdinBackups[0], ":")
	domainPath, domainTags, _ := strings.Cut(eng.stdinBackups[1], ":")
	eng.snaps = []restic.Snapshot{
		{ID: "1111111111111111", Time: "2026-10-15T12:00:00Z", Tags: strings.Split(zvolTags, ","), Paths: []string{zvolPath},
			Summary: &restic.SnapshotSummary{TotalBytesProcessed: uint64(gib)}},
		{ID: "deadbeef12345678", Time: "2026-10-15T12:00:05Z", Tags: strings.Split(domainTags, ","), Paths: []string{domainPath}},
	}
	seedVMsRepoConfig(t, root)

	res, err := svc.CheckRestore(context.Background(), api.RestoreCheckRequest{Kind: "vm", Name: "testvm", SnapshotID: "latest"})
	if err != nil {
		t.Fatalf("CheckRestore: %v", err)
	}
	return res
}

func spaceCheck(t *testing.T, res api.RestoreCheck) api.CheckLine {
	t.Helper()
	for _, c := range res.Checks {
		if c.ID == "space" {
			return c
		}
	}
	t.Fatalf("no space line in %+v", res.Checks)
	return api.CheckLine{}
}

func TestTheRestorePlanOfAZvolVMListsTheZvolAndItsSafetySnapshot(t *testing.T) {
	res := checkZvolRestore(t, "tank\tfilesystem\t53687091200\n"+
		"tank/vms\tfilesystem\t53687091200\n"+
		"tank/vms/testvm\tvolume\t54760833024\n")

	if res.Plan == nil || len(res.Plan.Zvols) != 1 {
		t.Fatalf("plan = %+v, want the one zvol", res.Plan)
	}
	z := res.Plan.Zvols[0]
	if z.Dataset != "tank/vms/testvm" || z.Size != gib || z.Change != "changed" || z.Safety != "tank/vms/testvm@bombvault-before-restore-" {
		t.Fatalf("zvol = %+v, want tank/vms/testvm replaced at 1 GiB after a safety snapshot", z)
	}
	if c := spaceCheck(t, res); c.Status != "ok" || c.Need != gib || c.Free != 54760833024 {
		t.Fatalf("space = %+v, want ok with the zvol's size and room", c)
	}
	if !res.Ready {
		t.Fatalf("check = %+v, want ready", res)
	}
}

func TestARestoreThatCreatesTheZvolNeedsItsSizeFreeOnThePool(t *testing.T) {
	res := checkZvolRestore(t, "tank\tfilesystem\t536870912\n"+
		"tank/vms\tfilesystem\t536870912\n")

	z := res.Plan.Zvols[0]
	if z.Change != "added" || z.Safety != "" || z.Free != 536870912 {
		t.Fatalf("zvol = %+v, want one created in tank/vms without a safety snapshot", z)
	}
	if c := spaceCheck(t, res); c.Status != "fail" || c.Reason != "short" || c.Need != gib || c.Free != 536870912 {
		t.Fatalf("space = %+v, want a shortfall of the zvol's size", c)
	}
	if res.Ready {
		t.Fatal("a zvol that cannot be created must keep the restore from starting")
	}
}

func TestARestoreOntoAServerWithoutThePoolIsRefused(t *testing.T) {
	res := checkZvolRestore(t, "cache\tfilesystem\t53687091200\n")

	if c := spaceCheck(t, res); c.Status != "fail" || c.Reason != "pool-missing" || c.Detail != "tank" {
		t.Fatalf("space = %+v, want the missing pool tank", c)
	}
	if res.Ready {
		t.Fatal("want the restore locked")
	}
}

func TestAZvolTheHostCannotListLeavesTheSpaceUnmeasured(t *testing.T) {
	res := checkZvolRestore(t, unreadable)

	if z := res.Plan.Zvols[0]; z.Change != "" || z.Free != -1 {
		t.Fatalf("zvol = %+v, want it unknown", z)
	}
	if c := spaceCheck(t, res); c.Status != "skip" || c.Reason != "unmeasured" {
		t.Fatalf("space = %+v, want it left unmeasured", c)
	}
}
