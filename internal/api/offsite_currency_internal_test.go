package api

import (
	"testing"
	"time"
)

const oneDay = int64(86400)

func statusFor(t *testing.T, f *placementFixture, domain string) DomainStatusEntry {
	t.Helper()
	entries, err := f.svc.domainStatusFrom(settingsOf(t, f.svc))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Domain == domain {
			return e
		}
	}
	t.Fatalf("no status for %s", domain)
	return DomainStatusEntry{}
}

// dailyContainers turns the containers domain on with a daily backup, so a
// coupled replication has a grace of two days.
func dailyContainers(t *testing.T, f *placementFixture) {
	t.Helper()
	s := settingsOf(t, f.svc)
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:00"
	if err := f.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
}

// replicatedAt records a successful copy to the target; agingOnly marks it as a
// run that only aged the target.
func replicatedAt(t *testing.T, f *placementFixture, domain, targetID string, at int64, agingOnly bool) {
	t.Helper()
	id, err := f.st.RecordOffsiteRunForTarget(domain, targetID, at)
	if err != nil {
		t.Fatal(err)
	}
	if agingOnly {
		if err := f.st.MarkOffsiteRunAgingOnly(targetID, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.st.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
}

func TestAPausedDomainIsAmber(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainers(t, f)
	f.target("containers", "B2", "b2:bucket:containers")
	f.setDefault("containers", "")
	if _, err := f.st.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}
	if st := statusFor(t, f, "containers"); st.ReplicationState != "paused" || st.Protection != "amber" {
		t.Fatalf("state %q, protection %q; want paused and amber", st.ReplicationState, st.Protection)
	}
}

func TestATargetNoItemIsCopiedToDropsOut(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainers(t, f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	nginx := f.container("nginx", "")
	f.rule("containers", "container:nginx", hz.ID)
	now := time.Now().Unix()
	f.backupRun(nginx.ID, now-3*oneDay)
	replicatedAt(t, f, "containers", b2.ID, now-3*oneDay+600, false)

	if st := statusFor(t, f, "containers"); st.ReplicationState != "ok" {
		t.Fatalf("state %q, want ok: Hetzner gets nothing and does not count", st.ReplicationState)
	}
}

func TestCoupledCurrencyIsJudgedPerTarget(t *testing.T) {
	scene := func(t *testing.T, hzLast int64) string {
		f := newPlacementFixture(t)
		dailyContainers(t, f)
		b2 := f.target("containers", "B2", "b2:bucket:containers")
		hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
		nginx := f.container("nginx", "")
		plex := f.container("plex", "")
		f.rule("containers", "container:nginx", hz.ID)
		f.rule("containers", "container:plex", b2.ID)
		now := time.Now().Unix()
		f.backupRun(nginx.ID, now-4*oneDay)
		replicatedAt(t, f, "containers", b2.ID, now-4*oneDay+600, false)
		f.backupRun(plex.ID, now-3600)
		replicatedAt(t, f, "containers", hz.ID, now+hzLast, false)
		return statusFor(t, f, "containers").ReplicationState
	}
	t.Run("each target has what its items last wrote", func(t *testing.T) {
		if got := scene(t, -1800); got != "ok" {
			t.Fatalf("state %q, want ok: B2's copy is old, and so is the only backup it gets", got)
		}
	})
	t.Run("one target is behind its items", func(t *testing.T) {
		if got := scene(t, -5*oneDay); got != "overdue" {
			t.Fatalf("state %q, want overdue: Hetzner last copied long before plex's backup", got)
		}
	})
}

func TestAnAgingRunDoesNotMakeATargetCurrent(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainers(t, f)
	s := settingsOf(t, f.svc)
	s.ContainersOffsiteSchedule = "daily 03:00"
	if err := f.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	f.container("nginx", "")
	f.container("plex", "")
	f.rule("containers", "container:nginx", hz.ID)
	f.rule("containers", "container:plex", b2.ID)
	now := time.Now().Unix()
	replicatedAt(t, f, "containers", b2.ID, now-3600, false)
	replicatedAt(t, f, "containers", hz.ID, now-5*oneDay, false)
	replicatedAt(t, f, "containers", hz.ID, now-3600, true)

	if st := statusFor(t, f, "containers"); st.ReplicationState != "overdue" {
		t.Fatalf("state %q, want overdue: Hetzner's last copy is five days old", st.ReplicationState)
	}
}

func TestWithoutRulesTheDomainIsJudgedAsBefore(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainers(t, f)
	f.target("containers", "B2", "b2:bucket:containers")
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, time.Now().Unix()-3*oneDay)

	if _, byTarget, _ := f.svc.placementCurrency(settingsOf(t, f.svc), "containers"); byTarget {
		t.Fatal("a domain without rules is judged per target")
	}
	if st := statusFor(t, f, "containers"); st.ReplicationState != "overdue" {
		t.Fatalf("state %q, want overdue as before: a backup three days old never replicated", st.ReplicationState)
	}
}
