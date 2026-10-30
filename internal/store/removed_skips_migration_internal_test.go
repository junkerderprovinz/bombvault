package store

import (
	"bytes"
	"database/sql"
	"log"
	"reflect"
	"strings"
	"testing"
)

// skipsBeforeTheUpgrade is a database one step before removedSkipsMigration
// whose skip lists name the vms target nas under containers and a target gone
// that exists nowhere.
func skipsBeforeTheUpgrade(t *testing.T) *sql.DB {
	t.Helper()
	db := OpenMem(t)
	bootAs(t, db, removedSkipsMigration-1)
	if _, err := db.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at) VALUES
		  ('box', 'containers', 'Box', 's3:box/containers', 'offsite', 1, 1),
		  ('nas', 'vms',        'NAS', 's3:nas/vms',        'offsite', 1, 1);
		DELETE FROM placement_defaults;
		INSERT INTO placement_defaults (domain, home, skip, confirmed_at) VALUES
		  ('containers', '', '["box","gone","nas"]', 1),
		  ('vms',        '', '["*"]', 1),
		  ('files',      '', '["gone"]', 1);
		INSERT INTO offsite_copy_rules (domain, identity, skip) VALUES
		  ('containers', 'container:plex', '["box","gone"]'),
		  ('vms',        'vm:win11',       '["nas"]');`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func TestTheUpgradeDropsRemovedTargetsFromSkipLists(t *testing.T) {
	db := skipsBeforeTheUpgrade(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	r := New(db)
	for domain, want := range map[string][]string{"containers": {"box"}, "vms": {SkipAll}, "files": {}} {
		d, found, err := r.PlacementDefaultFor(domain)
		if err != nil || !found {
			t.Fatalf("%s default: %v, %v", domain, found, err)
		}
		if !reflect.DeepEqual(d.Skip, want) {
			t.Errorf("%s skip = %v, want %v", domain, d.Skip, want)
		}
	}
	for _, c := range []struct {
		domain, identity string
		want             []string
	}{{"containers", "container:plex", []string{"box"}}, {"vms", "vm:win11", []string{"nas"}}} {
		rule, found, err := r.CopyRuleFor(c.domain, c.identity)
		if err != nil || !found || !reflect.DeepEqual(rule.Skip, c.want) {
			t.Errorf("%s rule = %+v, %v, %v, want skip %v", c.identity, rule, found, err, c.want)
		}
	}
}

func TestTheUpgradeLogsEachIdItTakesOutOfTheSkipLists(t *testing.T) {
	db := skipsBeforeTheUpgrade(t)
	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(out); log.SetFlags(flags) })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var got []string
	for line := range strings.Lines(buf.String()) {
		if strings.Contains(line, "skip lists") {
			got = append(got, strings.TrimSpace(line))
		}
	}
	want := []string{
		"placement: took gone out of the containers skip lists, since no copy of the section has that id",
		"placement: took nas out of the containers skip lists, since no copy of the section has that id",
		"placement: took gone out of the files skip lists, since no copy of the section has that id",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("logged %q, want %q", got, want)
	}
}
