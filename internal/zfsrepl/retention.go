package zfsrepl

import (
	"sort"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Expired returns the replica snapshots among names that keep does not cover,
// oldest first, counting the rules the way restic's forget does. Foreign names
// and protect, the base of the next run, are never returned, and all rules
// zero keeps everything.
func Expired(names []string, keep store.RetentionKeep, protect string, loc *time.Location) []string {
	if keep.KeepLast == 0 && keep.KeepDaily == 0 && keep.KeepWeekly == 0 && keep.KeepMonthly == 0 && keep.KeepYearly == 0 {
		return nil
	}
	type stamped struct {
		name string
		at   time.Time
	}
	var snaps []stamped
	for _, n := range names {
		if !zfs.IsReplicaSnapshot(n) {
			continue
		}
		at, ok := zfs.StampTime(n)
		if !ok {
			continue
		}
		snaps = append(snaps, stamped{n, at.In(loc)})
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].at.After(snaps[j].at) })

	type rule struct {
		left   int
		bucket func(time.Time) int
		last   int
	}
	rules := []*rule{
		{left: keep.KeepDaily, bucket: func(t time.Time) int { return t.Year()*10000 + int(t.Month())*100 + t.Day() }},
		{left: keep.KeepWeekly, bucket: func(t time.Time) int { y, w := t.ISOWeek(); return y*100 + w }},
		{left: keep.KeepMonthly, bucket: func(t time.Time) int { return t.Year()*100 + int(t.Month()) }},
		{left: keep.KeepYearly, bucket: func(t time.Time) int { return t.Year() }},
	}
	for _, r := range rules {
		r.last = -1
	}

	var expired []string
	for i, s := range snaps {
		kept := i < keep.KeepLast || s.name == protect
		for _, r := range rules {
			if r.left == 0 {
				continue
			}
			if b := r.bucket(s.at); b != r.last {
				r.last = b
				r.left--
				kept = true
			}
		}
		if !kept {
			expired = append(expired, s.name)
		}
	}
	for i, j := 0, len(expired)-1; i < j; i, j = i+1, j-1 {
		expired[i], expired[j] = expired[j], expired[i]
	}
	return expired
}
