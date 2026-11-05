package api

import (
	"strings"
	"testing"
	"time"
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
	if !strings.Contains(msg, "Backups for containers are overdue: last success 30d ago, expected every 1d.") {
		t.Fatalf("the digest does not name the overdue section:\n%s", msg)
	}
	if !stats.needsAttention() {
		t.Fatal("a digest with stopped backups does not reach somebody who hears only about failures")
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
