package restic

import (
	"context"
	"errors"
	"strconv"
)

// ErrSharedMemberPrunes is what a member asked to forget or prune in a shared
// repository answers with. The owner ages every server's history there.
var ErrSharedMemberPrunes = errors.New("only the server that holds a shared repository forgets and prunes there")

// refuseMemberPrune refuses a forget or a prune on a member of a shared
// repository. Over the append-only rest-server it would fail anyway, but a
// member that reaches the folder some other way must not age the others'
// history either.
func (m Mode) refuseMemberPrune() error {
	if m.Sharing == SharingMember {
		return ErrSharedMemberPrunes
	}
	return nil
}

// EveryHostStatsArgs measures a shared repository as a whole, across the
// snapshots of every server in it.
func EveryHostStatsArgs(repo, mode string, m Mode) []string {
	return statsArgs(repo, mode, m, nil)
}

// StatsOfEveryHost is Stats without the filter a shared repository puts on
// it.
func (r Restic) StatsOfEveryHost(ctx context.Context, repo, mode string, m Mode) (StatsResult, error) {
	return r.stats(ctx, EveryHostStatsArgs(repo, mode, m), m)
}

func maxUnusedFlags(percent int) []string {
	if percent <= 0 {
		return nil
	}
	return []string{"--max-unused", strconv.Itoa(percent) + "%"}
}
