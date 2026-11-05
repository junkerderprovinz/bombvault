package api

import (
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheDigestNamesBackupsThatStopped(t *testing.T) {
	s, _, _, seeded := watchdogTestService(t)

	stats, err := s.collectDigestStats(time.Unix(seeded+30*86400, 0))
	if err != nil {
		t.Fatal(err)
	}
	msg := composeDigest(stats)
	if strings.Contains(msg, "All good") {
		t.Fatalf("a digest with stopped backups says all is good:\n%s", msg)
	}
	if !strings.Contains(msg, `Backups for containers are overdue: "plex": last success 30d ago, expected every 1d.`) {
		t.Fatalf("the digest does not name the overdue section:\n%s", msg)
	}
	if !stats.needsAttention() {
		t.Fatal("a digest with stopped backups does not reach somebody who hears only about failures")
	}
}

func TestTheDigestOffSiteLineReadsTheOldestCopyAsTheStatusDoes(t *testing.T) {
	s, st, _, seeded := watchdogTestService(t)
	if _, err := st.MutateSettings(func(set *store.Settings) error {
		set.ContainersOffsite = "backups/copy"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	second, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "second", Repo: "backups/second", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "first", Repo: "backups/first", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.RecordOffsiteRunForTarget("containers", first.ID, seeded)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(run, true, ""); err != nil {
		t.Fatal(err)
	}
	_ = second

	stats, err := s.collectDigestStats(time.Unix(seeded+60, 0))
	if err != nil {
		t.Fatal(err)
	}
	if msg := composeDigest(stats); !strings.Contains(msg, "containers: no successful copy yet") {
		t.Fatalf("a second copy that never ran reads:\n%s", msg)
	}
}

func TestTheDigestStaysGoodWhileBackupsAreCurrent(t *testing.T) {
	s, _, _, seeded := watchdogTestService(t)

	stats, err := s.collectDigestStats(time.Unix(seeded+3600, 0))
	if err != nil {
		t.Fatal(err)
	}
	if msg := composeDigest(stats); !strings.Contains(msg, "All good") || strings.Contains(msg, "overdue") {
		t.Fatalf("a current section reads:\n%s", msg)
	}
}
