// Command bombvault is the single static binary: it loads config, opens the
// SQLite store, wires the real adapters into the backup service, starts the
// per-domain scheduler, and serves the JSON API + embedded React SPA.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
	web "github.com/junkerderprovinz/bombvault/web"
)

func main() {
	// Docker HEALTHCHECK entry (#60): `bombvault healthcheck` probes the local API
	// and exits 0/1 so auto-heal tools can restart a wedged container. Handled
	// before run() so it never touches the store, scheduler or server.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

// healthcheck is the Docker HEALTHCHECK probe (#60, requested by @BaukeZwart). It
// asks the engine's own /api/health endpoint (open, LAN trust model) whether it is
// serving and returns 0 on HTTP 200, non-zero otherwise, so an auto-heal tool
// (e.g. Autoheal / willfarrell) can restart a container whose engine has wedged.
// It reuses this binary, so the image needs no shell or curl. PORT/HTTPS_PORT come
// from the environment (the Dockerfile sets both).
func healthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	httpsPort := os.Getenv("HTTPS_PORT")
	if httpsPort == "" {
		httpsPort = "3443"
	}
	return healthcheckAt(port, httpsPort)
}

// healthcheckAt is the testable core: it tries HTTP first, then HTTPS (the local
// cert is self-signed, so verification is skipped — this is a liveness probe on
// loopback, not a trust boundary), and returns 0 as soon as /api/health answers 200.
func healthcheckAt(port, httpsPort string) int {
	client := &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: loopback self-signed cert; liveness probe, not a trust boundary
		},
	}
	for _, url := range []string{
		"http://127.0.0.1:" + port + "/api/health",
		"https://127.0.0.1:" + httpsPort + "/api/health",
	} {
		resp, err := client.Get(url) //nolint:gosec // G107: 127.0.0.1 with an env-configured port, not user input
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return 0
		}
	}
	fmt.Fprintln(os.Stderr, "bombvault: healthcheck failed — /api/health did not answer 200")
	return 1
}

// ensureDataDirWritable verifies the data dir exists and is writable before the
// store is opened, so a missing/read-only /config mount fails loudly instead of
// silently persisting state to the container's ephemeral layer.
func ensureDataDirWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("data dir %q is not creatable — is the /config mount present? %w", dir, err)
	}
	probe := filepath.Join(dir, ".bombvault-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("data dir %q is not writable — is the /config mount present and read-write? %w", dir, err)
	}
	_ = os.Remove(probe)
	return nil
}

func run() error {
	// Send the standard logger to stdout so all runtime logs share ONE stream
	// with the ASCII banner (printed via fmt to stdout). Otherwise Docker/Unraid
	// interleaves stderr (log default) above the stdout banner.
	log.SetOutput(os.Stdout)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	// Fail fast if the data dir (the /config mount) isn't a present, writable
	// filesystem — otherwise the SQLite DB lands on the container's ephemeral
	// layer and settings/targets/password silently reset on every restart.
	if err := ensureDataDirWritable(cfg.DataDir); err != nil {
		return err
	}

	// Apply any staged config self-restore BEFORE the DB is opened — the only safe
	// moment to swap BombVault's own settings database, which the running process
	// otherwise holds open (WAL). Fail-safe: on any error the boot continues on the
	// existing live DB (ApplyPending has already cleared the pending state).
	if applied, err := selfrestore.ApplyPending(cfg.DataDir); err != nil {
		log.Printf("selfrestore: %v", err) // fail-safe: boot continues on the live DB
	} else if applied {
		log.Printf("selfrestore: applied a staged config restore; booting on the restored settings")
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := store.Migrate(db); err != nil {
		return err
	}
	st := store.New(db)

	// Reap runs left in 'running' by a previous lifetime (crash/update mid-backup)
	// so they don't linger as a perpetual "running" status on the dashboard.
	if n, rerr := st.ReapInterruptedRuns(); rerr != nil {
		log.Printf("reap interrupted runs: %v", rerr)
	} else if n > 0 {
		log.Printf("marked %d interrupted run(s) from a previous run as failed", n)
	}

	// Real Docker adapter over the mounted docker.sock.
	dc, err := dockercli.New()
	if err != nil {
		return err
	}
	defer func() { _ = dc.Close() }()

	// libvirt over SSH (qemu+ssh://) — NO filesystem mount, so the container can
	// never interfere with the host VM Manager. Generate the SSH key on first run;
	// the user authorizes the public key on the host (shown in Settings). virsh
	// then runs ON the host over SSH.
	sc := sshconn.New(cfg.LibvirtHost, cfg.LibvirtSSHUser, cfg.LibvirtSSHPort, cfg.DataDir)
	if err := sc.EnsureKey(); err != nil {
		log.Printf("sshconn: ensure key: %v", err) // non-fatal: VM backup stays unavailable until fixed
	}
	// Write ~/.ssh/config so libvirt's qemu+ssh (external ssh binary) uses our key
	// + known_hosts + accept-new — it ignores the URI's keyfile/known_hosts params.
	if err := sc.WriteSSHConfig(); err != nil {
		log.Printf("sshconn: write ssh config: %v", err)
	}
	vc := virshcli.New(sc.VirshURI())

	// Real restic CLI adapter. RcloneConfig points at the managed rclone config
	// (written below) so off-site (rclone) repos authenticate. CacheDir pins
	// restic's index/pack cache under the persistent /config volume so it survives
	// container restarts — without it every run after a restart re-downloads the
	// repo index, which is ruinous for off-site (high-latency) replication (#95).
	resticCacheDir := filepath.Join(cfg.DataDir, "cache", "restic")
	if mkErr := os.MkdirAll(resticCacheDir, 0o750); mkErr != nil { // internal cache; restic manages its own subdir perms
		log.Printf("main: could not create restic cache dir %s: %v (falling back to restic default)", resticCacheDir, mkErr)
		resticCacheDir = ""
	}
	engine := &restic.Restic{Bin: "restic", RcloneConfig: filepath.Join(cfg.DataDir, "rclone.conf"), CacheDir: resticCacheDir}

	// Backup service bridges the adapters into the DI orchestrator.
	svc := api.NewService(cfg, st, dc, vc, engine)
	// Tell the service where the persistent cache lives so the post-run cache
	// trim (TrimResticCache) can measure + evict per-repo cache subdirs. Empty
	// (the mkdir-failed fallback above) disables the size-based trim.
	svc.SetResticCacheDir(resticCacheDir)
	svc.SetHostSSH(sc) // NVRAM transfer over SSH + the Settings key/test endpoints
	// Live backup/restore progress: the service publishes percentages here and the
	// SSE endpoint (/api/progress) streams them to the SPA's per-card bars.
	prog := progress.NewStore()
	svc.SetProgress(prog)
	// Materialise the (decrypted) rclone config on disk for off-site repos.
	if err := svc.WriteRcloneConfFile(); err != nil {
		log.Printf("rclone: write config: %v", err) // non-fatal: off-site stays unavailable until fixed
	}

	// Per-domain scheduler; the containers job calls the service's Backup, the
	// VMs job calls BackupVM (wired via SetVMJob below). Each scheduled item runs
	// with its own Healthchecks ping suppressed (context flag) so the run's ONE
	// aggregate start/success/fail ping — wired via SetHealthchecksAggregator below —
	// represents the whole domain job, not each container/VM (#49). Every other
	// notification channel still fires per item.
	scheduler := schedule.New(
		func(name string) error {
			ctx := api.WithBulkReplicateSuppressed(notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background())))
			_, bErr := svc.Backup(ctx, name)
			if errors.Is(bErr, backup.ErrContainerNotInstalled) {
				return nil // container no longer on the host: a skip (already recorded), not a job failure (#57)
			}
			return bErr
		},
		st.ListTargetsScheduleOrder, // #95: never/least-recently-backed-up first so a slow run can't starve the same tail
	)
	scheduler.SetVMJob(
		func(name string) error {
			ctx := api.WithBulkReplicateSuppressed(notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background())))
			_, bErr := svc.BackupVM(ctx, name)
			if errors.Is(bErr, backup.ErrVMNotInstalled) {
				return nil // VM no longer on the host: a skip (already logged), not a job failure
			}
			return bErr
		},
		st.ListVMTargets,
	)
	// Aggregate the per-domain Healthchecks lifecycle for scheduled multi-item runs:
	// one /start before the first item, one success/fail after the last (#49).
	scheduler.SetHealthchecksAggregator(
		func(domain string) { svc.ScheduledHealthchecksStart(context.Background(), domain) },
		func(domain string, attempted, failed int, failures []schedule.ItemFailure) {
			svc.ScheduledHealthchecksResult(context.Background(), domain, attempted, failed)
			svc.ScheduledNotifyResult(context.Background(), domain, attempted, failed, failures)
		},
	)
	scheduler.SetFlashJob(func() error {
		_, bErr := svc.BackupFlash(context.Background())
		return bErr
	})
	scheduler.SetConfigJob(func() error {
		_, bErr := svc.BackupConfig(context.Background())
		return bErr
	})
	// Files is a multi-item domain like containers/VMs: each scheduled file set
	// runs with its per-item Healthchecks/summary pings suppressed so the ONE
	// aggregate ping (SetHealthchecksAggregator above) represents the whole run.
	// The scheduler hands over the set's stable ID, not its name.
	scheduler.SetFilesJob(func(id string) error {
		ctx := api.WithBulkReplicateSuppressed(notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background())))
		_, bErr := svc.BackupFileSet(ctx, id)
		return bErr
	}, st.ListFileSets)
	scheduler.SetOffsiteJob(func(domain string) error {
		return svc.ScheduledReplicateOffsite(context.Background(), domain)
	})
	// Batched post-loop local prune for scheduled multi-item domains: each item's
	// post-backup retention runs forget WITHOUT --prune under the bulk flag, so the
	// expensive space-reclaim happens ONCE per run instead of once per item (a
	// 44-container night used to pay 44 full local prunes). The scheduler invokes
	// it just BEFORE the batched off-site replication — retention first means fewer
	// snapshots to copy. No-op for domains without a retention policy.
	scheduler.SetPruneAfterBulkJob(func(domain string) {
		svc.PruneAfterBulk(context.Background(), domain)
	})
	// #95: batched off-site replication for scheduled multi-item domains. After the
	// whole backup loop the domain is replicated ONCE (the per-item inline copy is
	// suppressed via WithBulkReplicateSuppressed above), so a high-latency off-site
	// backend is opened + its index reloaded once per run instead of once per item.
	// No-op for domains with no off-site repo or a separate off-site schedule.
	scheduler.SetOffsiteAfterBulkJob(func(domain string) {
		svc.ReplicateOffsiteAfterBulk(context.Background(), domain)
		// End of the scheduled domain run: trim restic's persistent cache (its own
		// `cache --cleanup` janitor + the ResticCacheMaxMB LRU eviction). Riding the
		// existing after-bulk hook needs no new cron plumbing; best-effort and cheap.
		svc.TrimResticCache(context.Background())
	})
	scheduler.SetDrillJob(func(domain, source, kind string) error {
		// Scheduled: wait for the domain lock so a nightly backup/replication co-fire
		// can't make the drill silently vanish (records nothing → dashboard "never").
		_, dErr := svc.RunRestoreDrill(context.Background(), domain, source, kind, true)
		return dErr
	})
	scheduler.SetTamperJob(func(domain string) error {
		_, tErr := svc.RunTamperTest(context.Background(), domain)
		return tErr
	})
	// Weekly digest: one summary message per fire through the notify fan-out.
	scheduler.SetDigestJob(func() error {
		return svc.SendDigest(context.Background())
	})
	// Per-domain LastRunFuncs: the everyN due-gate queries the most recent
	// successful backup within each domain (containers / VMs / flash scoped separately).
	containersLastRun := schedule.LastRunFunc(st.LastSuccessfulContainerBackup)
	vmsLastRun := schedule.LastRunFunc(st.LastSuccessfulVMBackup)
	flashLastRun := schedule.LastRunFunc(st.LastSuccessfulFlashBackup)
	configLastRun := schedule.LastRunFunc(st.LastSuccessfulConfigBackup)
	filesLastRun := schedule.LastRunFunc(st.LastSuccessfulFilesBackup)

	if settings, sErr := st.GetSettings(); sErr == nil {
		if rErr := scheduler.ReloadWithDueChecks(settings, containersLastRun, vmsLastRun, flashLastRun, configLastRun, filesLastRun); rErr != nil {
			log.Printf("scheduler: initial reload failed: %v", rErr)
		}
	} else {
		log.Printf("scheduler: could not read settings: %v", sErr)
	}
	scheduler.Start()
	defer scheduler.Stop()

	// JSON API + embedded SPA.
	handler := api.NewHandler(cfg, st, dc, svc, scheduler, spike.DefaultProbes())
	handler.SetProgress(prog) // same store the service publishes to → SSE endpoint
	// Warm the host-integration check at startup so the dashboard shows the
	// green result list immediately on first load (no manual click needed).
	go handler.WarmSpike()
	// Sample existing repos' size shortly after boot so the dashboard Storage card
	// shows data for repos that already have backups (no wait for the next backup).
	go svc.CollectStatsOnStartup()
	server := api.NewServer(cfg, web.DistFS(), handler.Router())
	return server.Run()
}
