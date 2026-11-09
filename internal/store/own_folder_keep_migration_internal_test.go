package store

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"testing"
)

// ownFoldersBeforeTheUpgrade is a database one step before
// ownFolderKeepMigration: shared is the shared local rule, own the sections'
// own rules, and the own folders of domains lie in a location whose card is
// card.
func ownFoldersBeforeTheUpgrade(t *testing.T, shared, card RetentionKeep, own map[string]RetentionKeep, domains ...string) *Repo {
	t.Helper()
	db := OpenMem(t)
	bootForTodaysCode(t, db, ownFolderKeepMigration-1)
	r := New(db)
	if _, err := r.MutateSettings(func(s *Settings) error {
		s.RetentionKeepLast, s.RetentionKeepDaily, s.RetentionKeepWeekly = shared.KeepLast, shared.KeepDaily, shared.KeepWeekly
		s.RetentionKeepMonthly, s.RetentionKeepYearly = shared.KeepMonthly, shared.KeepYearly
		s.SetOwnRetention(own)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	d, err := r.SaveDestination(OffsiteTarget{
		Name: "Local folder", Repo: "zfs/bombvault/dxp480t", Provider: "path",
		RetentionKeepLast: card.KeepLast, RetentionKeepDaily: card.KeepDaily, RetentionKeepWeekly: card.KeepWeekly,
		RetentionKeepMonthly: card.KeepMonthly, RetentionKeepYearly: card.KeepYearly,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range domains {
		if _, err := r.UpsertPrimaryRemoteTarget(domain, OffsiteTarget{Repo: d.Repo + "/" + domain, DestinationID: d.ID, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func upgradeLogged(t *testing.T, r *Repo) []string {
	t.Helper()
	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(out); log.SetFlags(flags) })
	if err := Migrate(r.db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var lines []string
	for line := range strings.Lines(buf.String()) {
		if strings.HasPrefix(line, "retention: ") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func ownRules(t *testing.T, r *Repo) map[string]RetentionKeep {
	t.Helper()
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s.OwnRetention()
}

func TestTheUpgradeKeepsTheSharedRuleOfOwnFoldersThatPrunedByIt(t *testing.T) {
	week := RetentionKeep{KeepDaily: 7}
	r := ownFoldersBeforeTheUpgrade(t, week, RetentionKeep{KeepMonthly: 24},
		map[string]RetentionKeep{"vms": {KeepLast: 5}}, "containers", "vms")

	lines := upgradeLogged(t, r)

	want := map[string]RetentionKeep{"containers": week, "vms": {KeepLast: 5}}
	if got := ownRules(t, r); !reflect.DeepEqual(got, want) {
		t.Fatalf("own rules = %+v, want %+v", got, want)
	}
	if want := []string{"retention: containers keeps the shared keep rule as its own, since Local folder's How long to keep card says otherwise"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("logged %q, want %q", lines, want)
	}
}

func TestTheUpgradeLetsOwnFoldersThatKeptEverythingFollowTheirCard(t *testing.T) {
	r := ownFoldersBeforeTheUpgrade(t, RetentionKeep{}, RetentionKeep{KeepDaily: 14}, nil, "containers", "flash")

	lines := upgradeLogged(t, r)

	if got := ownRules(t, r); len(got) != 0 {
		t.Fatalf("own rules = %+v, want none, so both follow the card", got)
	}
	want := []string{
		"retention: the own folder of containers in Local folder kept every backup and from now on ages by that location's How long to keep card",
		"retention: the own folder of flash in Local folder kept every backup and from now on ages by that location's How long to keep card",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("logged %q, want %q", lines, want)
	}
}

func TestTheUpgradeLeavesOwnFoldersWhoseCardMatchesTheSharedRule(t *testing.T) {
	week := RetentionKeep{KeepDaily: 7}
	r := ownFoldersBeforeTheUpgrade(t, week, week, nil, "containers")

	if lines := upgradeLogged(t, r); len(lines) != 0 {
		t.Fatalf("logged %q, want nothing", lines)
	}
	if got := ownRules(t, r); len(got) != 0 {
		t.Fatalf("own rules = %+v, want none", got)
	}
}
