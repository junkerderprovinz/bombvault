package store

import "testing"

func TestUpdatingAfterBackupIsOffForEveryContainerByDefault(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	s, err := New(db).GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.UpdateContainersAfterBackup {
		t.Fatal("a fresh install must not update containers after a backup")
	}
}

func TestTheSwitchForEveryContainerIsSaved(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	s, _ := repo.GetSettings()
	s.UpdateContainersAfterBackup = true
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetSettings()
	if !got.UpdateContainersAfterBackup {
		t.Fatal("the switch did not survive a save")
	}
}

func TestAContainerFollowsTheSwitchUntilItHasAChoiceOfItsOwn(t *testing.T) {
	follows := Target{}
	if follows.UpdatesAfterBackup(false) || !follows.UpdatesAfterBackup(true) {
		t.Fatal("a container without a choice of its own must follow the switch")
	}
	on := Target{UpdateAfterBackup: true, UpdateAfterBackupOwn: true}
	off := Target{UpdateAfterBackupOwn: true}
	if !on.UpdatesAfterBackup(false) {
		t.Fatal("a container switched on must update while the switch is off")
	}
	if off.UpdatesAfterBackup(true) {
		t.Fatal("a container switched off must not update while the switch is on")
	}
}

func TestSettingAndClearingAContainersOwnChoice(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	if err := repo.SetUpdateAfterBackup("plex", false); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetTargetByContainer("plex")
	if !got.UpdateAfterBackupOwn || got.UpdateAfterBackup {
		t.Fatalf("want an own choice of off, got own=%v on=%v", got.UpdateAfterBackupOwn, got.UpdateAfterBackup)
	}
	if err := repo.FollowUpdateAfterBackup("plex"); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetTargetByContainer("plex")
	if got.UpdateAfterBackupOwn {
		t.Fatal("the container must follow the switch again")
	}
}

// A container switched on before the switch for every container existed keeps
// updating, and the others follow the switch, which starts off, so nothing
// changes for an existing install.
func TestMigrationKeepsWhatEachContainerDid(t *testing.T) {
	db := OpenMem(t)
	all := migrations
	var before []migration
	for _, m := range all {
		if m.version < updateEveryContainerMigration {
			before = append(before, m)
		}
	}
	migrations = before
	if err := Migrate(db); err != nil {
		migrations = all
		t.Fatal(err)
	}
	migrations = all
	if _, err := db.Exec(`INSERT INTO targets (id, container_name, appdata_paths, created_at, update_after_backup) VALUES
  ('t1', 'plex', '[]', 1000, 1),
  ('t2', 'nginx', '[]', 1000, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	s, _ := repo.GetSettings()
	plex, _ := repo.GetTargetByContainer("plex")
	nginx, _ := repo.GetTargetByContainer("nginx")
	if !plex.UpdatesAfterBackup(s.UpdateContainersAfterBackup) {
		t.Fatal("plex was switched on and must keep updating")
	}
	if nginx.UpdatesAfterBackup(s.UpdateContainersAfterBackup) {
		t.Fatal("nginx was off and must stay off")
	}
	if !plex.UpdateAfterBackupOwn || nginx.UpdateAfterBackupOwn {
		t.Fatalf("only plex has a choice of its own, got plex=%v nginx=%v", plex.UpdateAfterBackupOwn, nginx.UpdateAfterBackupOwn)
	}
}
