package api_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestMetricsExposition(t *testing.T) {
	orig := api.Version
	defer func() { api.Version = orig }()
	api.Version = "v9.9.9"

	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersSchedule = "daily 02:30"
	s.VMsEnabled = false
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// One successful and one failed container backup.
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	okRun, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(okRun, "success", "deadbeef12345678", 2048, ""); err != nil {
		t.Fatal(err)
	}
	failRun, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(failRun, "failed", "", 0, "boom"); err != nil {
		t.Fatal(err)
	}

	if err := st.AddRepoStat(store.RepoStat{
		Domain: "containers", Source: "local", At: 1700000000,
		RawSize: 4096, RestoreSize: 8192, Snapshots: 7,
	}); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	out, err := svc.Metrics()
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}

	mustContain := []string{
		"# HELP bombvault_build_info",
		"# TYPE bombvault_build_info gauge",
		`bombvault_build_info{version="v9.9.9"} 1`,
		"# HELP bombvault_backup_last_success_timestamp_seconds",
		"# TYPE bombvault_backup_last_success_timestamp_seconds gauge",
		`bombvault_backup_last_success_timestamp_seconds{domain="vms"} 0`,
		"# HELP bombvault_domain_enabled",
		"# TYPE bombvault_domain_enabled gauge",
		`bombvault_domain_enabled{domain="containers"} 1`,
		`bombvault_domain_enabled{domain="vms"} 0`,
		`bombvault_domain_enabled{domain="files"} 0`,
		"# HELP bombvault_repo_size_bytes",
		"# TYPE bombvault_repo_size_bytes gauge",
		`bombvault_repo_size_bytes{domain="containers",source="local"} 4096`,
		`bombvault_repo_snapshots{domain="containers",source="local"} 7`,
		"# HELP bombvault_runs_total",
		"# TYPE bombvault_runs_total counter",
		`bombvault_runs_total{domain="containers",status="success"} 1`,
		`bombvault_runs_total{domain="containers",status="failed"} 1`,
		`bombvault_runs_total{domain="vms",status="success"} 0`,
		`bombvault_runs_total{domain="files",status="success"} 0`,
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q\n--- output ---\n%s", want, out)
		}
	}

	if !strings.Contains(out, `bombvault_backup_last_success_timestamp_seconds{domain="containers"} `) {
		t.Errorf("missing containers last-success line:\n%s", out)
	}
	if strings.Contains(out, `bombvault_backup_last_success_timestamp_seconds{domain="containers"} 0`) {
		t.Errorf("containers last-success should be non-zero after a successful backup:\n%s", out)
	}

	if strings.Contains(out, `bombvault_repo_size_bytes{domain="vms"`) {
		t.Errorf("vms has no repo sample; it must not emit a repo_size line:\n%s", out)
	}
}

// containers is enabled and immutable and gets all three protection gauges.
// vms is enabled but not immutable, so it has no tamper_test_ok. flash is
// disabled and gets none. The replication timestamp is the last success.
func TestMetricsRansomwareGauges(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	s.ContainersOffsite = "rest:http://192.168.1.2:8000/containers"
	s.ContainersOffsiteImmutable = true
	s.VMsEnabled = true
	s.VMsOffsite = "rest:http://192.168.1.2:8000/vms"
	s.FlashEnabled = false
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}
	// An older success and a newer failure.
	id, err := st.RecordOffsiteRun("containers", 1700000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
	idFail, err := st.RecordOffsiteRun("containers", 1800000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(idFail, false, "boom"); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	out, err := svc.Metrics()
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}

	mustContain := []string{
		"# HELP bombvault_offsite_immutable",
		"# TYPE bombvault_offsite_immutable gauge",
		`bombvault_offsite_immutable{domain="containers"} 1`,
		`bombvault_offsite_immutable{domain="vms"} 0`,
		"# HELP bombvault_tamper_test_ok",
		"# TYPE bombvault_tamper_test_ok gauge",
		`bombvault_tamper_test_ok{domain="containers"} 1`,
		"# HELP bombvault_offsite_last_replication_timestamp_seconds",
		"# TYPE bombvault_offsite_last_replication_timestamp_seconds gauge",
		`bombvault_offsite_last_replication_timestamp_seconds{domain="containers"} 1700000000`,
		`bombvault_offsite_last_replication_timestamp_seconds{domain="vms"} 0`,
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q\n--- output ---\n%s", want, out)
		}
	}
	mustNotContain := []string{
		`bombvault_tamper_test_ok{domain="vms"}`,
		`bombvault_offsite_immutable{domain="flash"}`,
		`bombvault_tamper_test_ok{domain="flash"}`,
		`bombvault_offsite_last_replication_timestamp_seconds{domain="flash"}`,
	}
	for _, bad := range mustNotContain {
		if strings.Contains(out, bad) {
			t.Errorf("metrics output must gate out %q\n--- output ---\n%s", bad, out)
		}
	}
}

// The version is the only label whose value is not a fixed set.
func TestMetricsLabelEscaping(t *testing.T) {
	orig := api.Version
	defer func() { api.Version = orig }()
	api.Version = "v1\"2\\3\n4"

	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

	out, err := svc.Metrics()
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	want := `bombvault_build_info{version="v1\"2\\3\n4"} 1`
	if !strings.Contains(out, want) {
		t.Errorf("label not escaped per Prometheus rules; want %q in:\n%s", want, out)
	}
}

func TestMetricsExportDBDumpCounts(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	tg, err := st.UpsertTarget(store.Target{ContainerName: "pg", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	okRun, err := st.StartRun(tg.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(okRun, "success", "dbdump00deadbeef", 4096, ""); err != nil {
		t.Fatal(err)
	}
	failRun, err := st.StartRun(tg.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(failRun, "failed", "", 0, store.ReasonDBDumpAuth); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	out, err := svc.Metrics()
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}

	for _, want := range []string{
		"# TYPE bombvault_dbdump_runs_total counter",
		`bombvault_dbdump_runs_total{status="success"} 1`,
		`bombvault_dbdump_runs_total{status="failed"} 1`,
		"# TYPE bombvault_dbdump_last_success_timestamp_seconds gauge",
		`bombvault_dbdump_last_success_timestamp_seconds{container="pg"} `,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q\n--- output ---\n%s", want, out)
		}
	}
	if strings.Contains(out, `bombvault_dbdump_last_success_timestamp_seconds{container="pg"} 0`) {
		t.Errorf("the last dump succeeded, so its timestamp must not be 0:\n%s", out)
	}
}
