package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// dumpBackupFixture is a container backup that can take a dump: the store, the
// docker fake, the engine and the service, wired as production wires them.
type dumpBackupFixture struct {
	st  *store.Repo
	doc *fakeServiceDocker
	eng *fakeResticEngine
	svc *api.Service
}

func newDumpBackupFixture(t *testing.T, image string) *dumpBackupFixture {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	if err := os.MkdirAll(root+"/user/appdata/pg", 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: root, HostSourceRoot: "/mnt",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	doc := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/pg", Image: image, Running: true,
		Config: model.Config{Image: image, Env: []string{"POSTGRES_PASSWORD=secret"}},
		Mounts: []model.Mount{{Type: "bind", Source: "/mnt/user/appdata/pg", Destination: "/var/lib/postgresql/data"}},
	}}
	eng := &fakeResticEngine{}
	return &dumpBackupFixture{st: st, doc: doc, eng: eng, svc: api.NewService(cfg, st, doc, fakeVirsh{}, eng)}
}

// runsOfKind returns the recorded runs of one kind for the only target.
func (f *dumpBackupFixture) runsOfKind(t *testing.T, kind string) []store.Run {
	t.Helper()
	tg, err := f.st.GetTargetByContainer("pg")
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	runs, err := f.st.RecentRunsOfKind(tg.ID, kind, 10)
	if err != nil {
		t.Fatalf("runs of kind %q: %v", kind, err)
	}
	return runs
}

func TestBackupDatabaseContainerDumpsBeforeStop(t *testing.T) {
	f := newDumpBackupFixture(t, "postgres:16")
	var callsAtDump []string
	f.eng.onCommandBackup = func() { callsAtDump = append([]string{}, f.doc.calls...) }
	f.eng.commandBackupSum = restic.Summary{SnapshotID: "dbdump00deadbeef", TotalBytesProcessed: 4096}
	f.eng.commandBackupLines = []string{
		"subprocess /usr/local/bin/bombvault: " + dbdump.Result{V: 1, OK: true, Bytes: 4096, Scope: "all"}.Line(),
	}

	if _, err := f.svc.Backup(context.Background(), "pg"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if len(f.eng.commandBackups) != 1 {
		t.Fatalf("%d dumps, want one", len(f.eng.commandBackups))
	}
	dump := f.eng.commandBackups[0]
	if dump.StdinPath != "/dbdump/pg.sql" {
		t.Errorf("stdin path = %q", dump.StdinPath)
	}
	for _, want := range []string{"dbdump:pg", "p1", "dbengine:postgres", "dbimage:postgres:16"} {
		if !contains(dump.Tags, want) {
			t.Errorf("tags %v are missing %q", dump.Tags, want)
		}
	}
	if !hasPrefixIn(dump.Tags, "bvrun:") {
		t.Errorf("tags %v carry no backup run id", dump.Tags)
	}
	wantCommand := "dbdump-stream --container pg --engine postgres --max-seconds"
	if !strings.Contains(strings.Join(dump.Command, " "), wantCommand) {
		t.Errorf("command = %v, want it to carry %q", dump.Command, wantCommand)
	}

	for _, call := range callsAtDump {
		if call == "stop:pg" {
			t.Fatalf("the container was stopped before the dump: %v", callsAtDump)
		}
	}

	dumps := f.runsOfKind(t, "dbdump")
	if len(dumps) != 1 || dumps[0].Status != "success" {
		t.Fatalf("dump runs = %+v, want one success", dumps)
	}
	if dumps[0].SnapshotID != "dbdump00deadbeef" || dumps[0].Bytes != 4096 {
		t.Errorf("dump run = %+v", dumps[0])
	}
	backups := f.runsOfKind(t, "backup")
	if len(backups) != 1 || backups[0].Status != "success" {
		t.Fatalf("backup runs = %+v, want one success", backups)
	}
}

func TestBackupNonDatabaseContainerNeverDumps(t *testing.T) {
	f := newDumpBackupFixture(t, "plexinc/pms-docker:latest")

	if _, err := f.svc.Backup(context.Background(), "pg"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if len(f.eng.commandBackups) != 0 {
		t.Fatalf("a container that is no database was dumped: %+v", f.eng.commandBackups)
	}
	if contains(f.eng.lastTags, "dbdump:pg") {
		t.Fatalf("volume tags = %v, want no dump identity", f.eng.lastTags)
	}
}

func TestBackupDumpFailureStillBacksUp(t *testing.T) {
	f := newDumpBackupFixture(t, "postgres:16")
	f.eng.commandBackupErr = errorString("restic backup: exit status 1")
	f.eng.commandBackupLines = []string{
		"subprocess /usr/local/bin/bombvault: " + dbdump.Result{V: 1, Reason: dbdump.ReasonAuth, Exit: 1}.Line(),
	}

	if _, err := f.svc.Backup(context.Background(), "pg"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	dumps := f.runsOfKind(t, "dbdump")
	if len(dumps) != 1 || dumps[0].Status != "failed" {
		t.Fatalf("dump runs = %+v, want one failure", dumps)
	}
	if dumps[0].Error != store.ReasonDBDumpAuth {
		t.Errorf("dump reason = %q, want %q", dumps[0].Error, store.ReasonDBDumpAuth)
	}
	backups := f.runsOfKind(t, "backup")
	if len(backups) != 1 || backups[0].Status != "success" {
		t.Fatalf("backup runs = %+v, want the files backup to have succeeded", backups)
	}
}

func TestBackupPrunesOnceForBothIdentities(t *testing.T) {
	f := newDumpBackupFixture(t, "postgres:16")
	s := mustSettings(t, f.st)
	s.RetentionKeepLast = 3
	if err := f.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.Backup(context.Background(), "pg"); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if want := []string{"dbdump:pg", "container:pg"}; strings.Join(f.eng.forgetTags, ",") != strings.Join(want, ",") {
		t.Fatalf("retention tags = %v, want %v", f.eng.forgetTags, want)
	}
	if got := f.eng.forgetPolicyPruned; len(got) != 2 || got[0] || !got[1] {
		t.Fatalf("prune flags = %v, want only the container pass to prune", got)
	}
}

func TestBackupDumpSwitches(t *testing.T) {
	t.Run("the container is opted out", func(t *testing.T) {
		f := newDumpBackupFixture(t, "postgres:16")
		if _, err := f.st.UpsertTarget(store.Target{ContainerName: "pg"}); err != nil {
			t.Fatal(err)
		}
		if err := f.st.SetDBDumpOff("pg", true); err != nil {
			t.Fatal(err)
		}
		f.mustBackup(t)
		f.wantNoDump(t)
	})

	t.Run("the label switches the dump off", func(t *testing.T) {
		f := newDumpBackupFixture(t, "postgres:16")
		f.doc.inspect.Config.Labels = map[string]string{"bombvault.dbdump": "false"}
		f.mustBackup(t)
		f.wantNoDump(t)
	})

	t.Run("the global switch is off", func(t *testing.T) {
		f := newDumpBackupFixture(t, "postgres:16")
		s := mustSettings(t, f.st)
		s.DBDumpsEnabled = false
		if err := f.st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		f.mustBackup(t)
		f.wantNoDump(t)
	})

	t.Run("a lookalike nobody decided about", func(t *testing.T) {
		f := newDumpBackupFixture(t, "acme/my-postgres:1")
		f.mustBackup(t)
		f.wantNoDump(t)
	})

	t.Run("a lookalike with a chosen engine", func(t *testing.T) {
		f := newDumpBackupFixture(t, "acme/my-postgres:1")
		if _, err := f.st.UpsertTarget(store.Target{ContainerName: "pg"}); err != nil {
			t.Fatal(err)
		}
		if err := f.st.SetDBDumpEngine("pg", "postgres"); err != nil {
			t.Fatal(err)
		}
		f.mustBackup(t)
		if len(f.eng.commandBackups) != 1 {
			t.Fatalf("%d dumps, want one", len(f.eng.commandBackups))
		}
	})
}

func (f *dumpBackupFixture) mustBackup(t *testing.T) {
	t.Helper()
	if _, err := f.svc.Backup(context.Background(), "pg"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
}

func (f *dumpBackupFixture) wantNoDump(t *testing.T) {
	t.Helper()
	if len(f.eng.commandBackups) != 0 {
		t.Fatalf("a dump ran although it is switched off: %+v", f.eng.commandBackups)
	}
}

// hasPrefixIn reports whether any of ss starts with prefix.
func hasPrefixIn(ss []string, prefix string) bool {
	for _, s := range ss {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

type errorString string

func (e errorString) Error() string { return string(e) }
