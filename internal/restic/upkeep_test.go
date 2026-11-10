package restic_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestAMemberNeverForgetsOrPrunesInASharedRepository(t *testing.T) {
	r := restic.Restic{Bin: filepath.Join(t.TempDir(), "no-restic")}
	ctx := context.Background()
	m := restic.Mode{Host: ownHost, Sharing: restic.SharingMember}
	p := restic.RetentionPolicy{KeepLast: 3}
	for name, err := range map[string]error{
		"forget":        r.Forget(ctx, "/repo", []string{"0123abcd"}, false, m),
		"forget policy": r.ForgetPolicy(ctx, "/repo", p, m, []string{"container:plex"}, false),
		"prune":         r.Prune(ctx, "/repo", m),
	} {
		if !errors.Is(err, restic.ErrSharedMemberPrunes) {
			t.Errorf("%s: err = %v, want ErrSharedMemberPrunes", name, err)
		}
	}
}

func TestStatsInASharedRepositoryCountTheModesHostAlone(t *testing.T) {
	for _, c := range []struct {
		sharing restic.Sharing
		hosts   []string
	}{
		{restic.SharingNone, nil},
		{restic.SharingMember, []string{ownHost}},
		{restic.SharingOwner, []string{ownHost}},
	} {
		args := restic.StatsArgs("/repo", "raw-data", restic.Mode{Host: ownHost, Sharing: c.sharing})
		if got := hostsIn(args); !slices.Equal(got, c.hosts) {
			t.Errorf("sharing %v: hosts %v in %v, want %v", c.sharing, got, args, c.hosts)
		}
	}
	every := restic.EveryHostStatsArgs("/repo", "raw-data", restic.Mode{Host: ownHost, Sharing: restic.SharingOwner})
	if got := hostsIn(every); len(got) != 0 {
		t.Errorf("every host: hosts %v, want no filter", got)
	}
}

func TestAPruneLeavesAsMuchUnusedSpaceAsTheModeAllows(t *testing.T) {
	args := restic.PruneArgs("/repo", restic.Mode{MaxUnused: 10})
	if i := slices.Index(args, "--max-unused"); i < 0 || i+1 >= len(args) || args[i+1] != "10%" {
		t.Fatalf("args %v, want --max-unused 10%%", args)
	}
	if slices.Contains(restic.PruneArgs("/repo", restic.Mode{}), "--max-unused") {
		t.Fatal("a prune without a limit of its own sets --max-unused")
	}
}

// Two servers that back up the same folder each count all of its data, while
// the repository holds it once.
func TestEachServersShareOfASharedRepositoryCountsTheDataItUses(t *testing.T) {
	repo, _, _ := sharedRepo(t)
	r, ctx := restic.Restic{Bin: "restic"}, context.Background()
	owner := restic.Mode{Host: ownHost, Sharing: restic.SharingOwner}
	whole, err := r.StatsOfEveryHost(ctx, repo, "raw-data", owner)
	if err != nil {
		t.Fatal(err)
	}
	var separate int64
	for _, host := range []string{ownHost, otherHost} {
		m := owner
		m.Host = host
		st, err := r.Stats(ctx, repo, "raw-data", m)
		if err != nil {
			t.Fatal(err)
		}
		if st.TotalSize <= 0 || st.TotalSize > whole.TotalSize {
			t.Fatalf("%s uses %d bytes of a repository of %d", host, st.TotalSize, whole.TotalSize)
		}
		separate += st.TotalSize
	}
	if separate <= whole.TotalSize {
		t.Fatalf("the two servers apart need %d bytes, the repository holds %d; want more apart", separate, whole.TotalSize)
	}
}
