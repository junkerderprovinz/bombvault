package store

import (
	"slices"
	"testing"
)

// An instance upgraded with logins on its receiver, repositories it watches
// and sources it pulls from goes on serving every one of those members: each
// is a request it allowed, with nobody having to answer anything.
func TestTheUpgradeTurnsExistingLoginsAndPairingsIntoAllowedRequests(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, roleRequestMigration-1)

	seed := []string{
		`INSERT INTO fleet_peers (id, member_id, name, url, enabled, created_at) VALUES ('fp1', 'm-barn', 'barn', '', 1, 1700000000)`,
		`INSERT INTO receiver_logins (member_id, member_name, rest_user, created_at) VALUES ('m-attic', 'attic', 'attic', 1700000100)`,
		`INSERT INTO received_repos (id, member_id, name, repo, created_at) VALUES ('rr1', 'm-attic', 'from attic', '/host/user/restic/attic', 1700000200)`,
		`INSERT INTO received_repos (id, member_id, name, repo, created_at) VALUES ('rr2', 'm-barn', 'barn a', '/host/user/restic/barn-a', 1700000400)`,
		`INSERT INTO received_repos (id, member_id, name, repo, created_at) VALUES ('rr3', 'm-barn', 'barn b', '/host/user/restic/barn-b', 1700000300)`,
		`INSERT INTO received_repos (id, name, repo, created_at) VALUES ('rr4', 'unpaired', '/host/user/restic/old', 1700000000)`,
		`INSERT INTO pull_sources (id, member_id, name, repo, domain, created_at) VALUES ('ps1', 'm-barn', 'barn vms', 'rest:http://192.168.1.9:8000/vms', 'vms', 1700000500)`,
		`INSERT INTO pull_sources (id, member_id, name, repo, domain, created_at) VALUES ('ps2', 'm-barn', 'barn containers', 'rest:http://192.168.1.9:8000/containers', 'containers', 1700000600)`,
		`INSERT INTO pull_sources (id, member_id, name, repo, domain, created_at) VALUES ('ps3', 'm-shed', 'shed', 'rest:http://192.168.1.7:8000/all', '', 1700000700)`,
		`INSERT INTO pull_sources (id, name, repo, domain, created_at) VALUES ('ps4', 'unpaired', 'rest:http://192.168.1.6:8000/old', 'flash', 1700000000)`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := New(db)
	all, err := r.ListRoleRequests()
	if err != nil || len(all) != 4 {
		t.Fatalf("requests after the upgrade = %+v, %v; want four", all, err)
	}
	every := []string{"config", "containers", "files", "flash", "vms", "zfs"}
	for _, want := range []struct {
		member, role, name, store string
		sections                  []string
		at                        int64
	}{
		{"m-attic", RoleReceiver, "attic", RoleStoreRest, every, 1700000100},
		{"m-barn", RoleReceiver, "barn", "", every, 1700000300},
		{"m-barn", RoleFetcher, "barn", "", []string{"containers", "vms"}, 1700000500},
		{"m-shed", RoleFetcher, "", "", every, 1700000700},
	} {
		got, ok, err := r.FindRoleRequest(RoleRequestIn, want.member, want.role)
		if err != nil || !ok {
			t.Fatalf("%s as %s: no request, %v", want.member, want.role, err)
		}
		if len(got.ID) != 32 || got.State != RoleAllowed || got.DecidedBy != RoleDecidedUpgrade ||
			got.MemberName != want.name || got.Store != want.store || !slices.Equal(got.Sections, want.sections) ||
			got.AskedAt != want.at || got.DecidedAt != want.at {
			t.Errorf("%s as %s = %+v", want.member, want.role, got)
		}
	}
}

func TestAFreshDatabaseStartsWithoutRoleRequests(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	all, err := New(db).ListRoleRequests()
	if err != nil || len(all) != 0 {
		t.Fatalf("requests on a fresh database = %+v, %v", all, err)
	}
}
