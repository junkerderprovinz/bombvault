package store

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"
)

// movedInstall is what the API reported of an install that set Days 14 on its
// Local card, added a location through the wizard, moved all six own folders
// there and set Days 14 on that location's card afterwards.
type movedInstall struct {
	Settings struct {
		RetentionKeepDaily int                      `json:"retentionKeepDaily"`
		OwnRetention       map[string]RetentionKeep `json:"ownRetention"`
	} `json:"settings"`
	Paths     map[string]string `json:"paths"`
	Locations []struct {
		Name      string        `json:"name"`
		Where     string        `json:"where"`
		Retention RetentionKeep `json:"retention"`
		Sections  []struct {
			Domain string `json:"domain"`
		} `json:"sections"`
	} `json:"locations"`
}

func readMovedInstall(t *testing.T) movedInstall {
	t.Helper()
	b, err := os.ReadFile("testdata/own_zero_rules_after_move.json")
	if err != nil {
		t.Fatal(err)
	}
	var m movedInstall
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// seedMovedInstall writes m into a database one step before the migration, each
// own folder linked to the location the way a move links it.
func seedMovedInstall(t *testing.T, m movedInstall) *Repo {
	t.Helper()
	db := OpenMem(t)
	bootForTodaysCode(t, db, hiddenZeroRuleMigration-1)
	r := New(db)
	if _, err := r.MutateSettings(func(s *Settings) error {
		s.RetentionKeepDaily = m.Settings.RetentionKeepDaily
		s.SetOwnRetention(m.Settings.OwnRetention)
		s.ContainersPath, s.VMsPath, s.FlashPath = m.Paths["containersPath"], m.Paths["vmsPath"], m.Paths["flashPath"]
		s.ConfigPath, s.FilesPath, s.ZFSPath = m.Paths["configPath"], m.Paths["filesPath"], m.Paths["zfsPath"]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	loc := m.Locations[0]
	d, err := r.SaveDestination(OffsiteTarget{
		Name: loc.Name, Repo: loc.Where, Provider: "path",
		RetentionKeepLast: loc.Retention.KeepLast, RetentionKeepDaily: loc.Retention.KeepDaily, RetentionKeepWeekly: loc.Retention.KeepWeekly,
		RetentionKeepMonthly: loc.Retention.KeepMonthly, RetentionKeepYearly: loc.Retention.KeepYearly,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range loc.Sections {
		if _, err := r.UpsertPrimaryRemoteTarget(sec.Domain, OffsiteTarget{Repo: m.Paths[sec.Domain+"Path"], DestinationID: d.ID, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func migrateLogged(t *testing.T, r *Repo) []string {
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

func zeroRuleLine(domain string) string {
	return "retention: " + domain + " lost its all-zero keep rule, and its own folder in Local folder ages by that location's How long to keep card"
}

func TestTheUpgradeClearsTheZeroRulesAMoveLeftOnSectionsInALocation(t *testing.T) {
	r := seedMovedInstall(t, readMovedInstall(t))

	lines := migrateLogged(t, r)

	if got := ownRules(t, r); len(got) != 0 {
		t.Errorf("own rules = %+v, want none, so every section follows the card", got)
	}
	want := []string{zeroRuleLine("config"), zeroRuleLine("containers"), zeroRuleLine("files"), zeroRuleLine("flash"), zeroRuleLine("vms"), zeroRuleLine("zfs")}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
}

func TestTheUpgradeKeepsARuleWithValuesOfItsOwn(t *testing.T) {
	m := readMovedInstall(t)
	m.Settings.OwnRetention["vms"] = RetentionKeep{KeepLast: 5}
	r := seedMovedInstall(t, m)

	migrateLogged(t, r)

	if got, want := ownRules(t, r), map[string]RetentionKeep{"vms": {KeepLast: 5}}; !reflect.DeepEqual(got, want) {
		t.Errorf("own rules = %+v, want %+v", got, want)
	}
}

func TestTheUpgradeKeepsTheZeroRuleOfASectionOutsideALocation(t *testing.T) {
	m := readMovedInstall(t)
	r := seedMovedInstall(t, m)
	if _, err := r.MutateSettings(func(s *Settings) error {
		s.FilesPath = "user/typed/files"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	lines := migrateLogged(t, r)

	if got, want := ownRules(t, r), map[string]RetentionKeep{"files": {}}; !reflect.DeepEqual(got, want) {
		t.Errorf("own rules = %+v, want the typed folder's rule kept", got)
	}
	if len(lines) != 5 {
		t.Errorf("logged %q, want a line for each of the other five sections", lines)
	}
}

func namedRepoFor(t *testing.T, r *Repo, container string) {
	t.Helper()
	archive, err := r.UpsertOffsiteTarget(OffsiteTarget{Role: RoleRepo, Name: "Archive", Repo: "user/archive", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertTarget(Target{ContainerName: container, Repo: archive.ID, RepoChosen: RepoChosen}); err != nil {
		t.Fatal(err)
	}
}

func TestTheUpgradeKeepsTheZeroRuleWhereANamedRepositoryWouldStartToPrune(t *testing.T) {
	r := seedMovedInstall(t, readMovedInstall(t))
	namedRepoFor(t, r, "plex")

	lines := migrateLogged(t, r)

	if got, want := ownRules(t, r), map[string]RetentionKeep{"containers": {}}; !reflect.DeepEqual(got, want) {
		t.Errorf("own rules = %+v, want only the section with the named repository kept", got)
	}
	held := "retention: containers keeps its all-zero keep rule, since a named repository of it would start to prune by the shared rule without it"
	want := []string{zeroRuleLine("config"), held, zeroRuleLine("files"), zeroRuleLine("flash"), zeroRuleLine("vms"), zeroRuleLine("zfs")}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
}

func TestTheUpgradeClearsTheZeroRuleWhereTheSharedRuleKeepsANamedRepositoryWhole(t *testing.T) {
	m := readMovedInstall(t)
	m.Settings.RetentionKeepDaily = 0
	r := seedMovedInstall(t, m)
	namedRepoFor(t, r, "plex")

	migrateLogged(t, r)

	if got := ownRules(t, r); len(got) != 0 {
		t.Errorf("own rules = %+v, want none, as the named repository ages the same either way", got)
	}
}

func TestTheUpgradeKeepsZeroRulesWhileTheCardIsZero(t *testing.T) {
	m := readMovedInstall(t)
	m.Locations[0].Retention = RetentionKeep{}
	r := seedMovedInstall(t, m)

	if lines := migrateLogged(t, r); len(lines) != 0 {
		t.Errorf("logged %q, want nothing", lines)
	}
	if got := ownRules(t, r); len(got) != 6 {
		t.Errorf("own rules = %+v, want all six kept", got)
	}
}
