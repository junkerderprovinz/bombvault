package api_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestMetricsExposition checks the Prometheus text the service builds: the
// build_info line (with the version label), per-domain last-success timestamps,
// enabled flags, repo size/snapshots from the latest sample, run counts, and
// that every metric carries its # HELP / # TYPE lines.
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

	// One successful + one failed container backup, so run counts and the
	// last-success timestamp are non-zero.
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

	// A repo-size sample for containers/local so the size + snapshot series appear.
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
		"# HELP bombvault_repo_size_bytes",
		"# TYPE bombvault_repo_size_bytes gauge",
		`bombvault_repo_size_bytes{domain="containers",source="local"} 4096`,
		`bombvault_repo_snapshots{domain="containers",source="local"} 7`,
		"# HELP bombvault_runs_total",
		"# TYPE bombvault_runs_total counter",
		`bombvault_runs_total{domain="containers",status="success"} 1`,
		`bombvault_runs_total{domain="containers",status="failed"} 1`,
		`bombvault_runs_total{domain="vms",status="success"} 0`,
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q\n--- output ---\n%s", want, out)
		}
	}

	// The containers last-success line must carry a real (non-zero) timestamp.
	if !strings.Contains(out, `bombvault_backup_last_success_timestamp_seconds{domain="containers"} `) {
		t.Errorf("missing containers last-success line:\n%s", out)
	}
	if strings.Contains(out, `bombvault_backup_last_success_timestamp_seconds{domain="containers"} 0`) {
		t.Errorf("containers last-success should be non-zero after a successful backup:\n%s", out)
	}

	// No domain that has no sample should emit a repo_size line (vms had none).
	if strings.Contains(out, `bombvault_repo_size_bytes{domain="vms"`) {
		t.Errorf("vms has no repo sample; it must not emit a repo_size line:\n%s", out)
	}
}

// TestMetricsLabelEscaping verifies label values are escaped per Prometheus
// rules (backslash, quote, newline) — exercised via the version label, the only
// label whose value isn't a fixed enum.
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
