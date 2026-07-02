// Command bombvault is the single static binary: it loads config, opens the
// SQLite store, wires the real adapters into the backup service, starts the
// per-domain scheduler, and serves the JSON API + embedded React SPA.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
	web "github.com/junkerderprovinz/bombvault/web"
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
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
	// (written below) so off-site (rclone) repos authenticate.
	engine := &restic.Restic{Bin: "restic", RcloneConfig: filepath.Join(cfg.DataDir, "rclone.conf")}

	// Backup service bridges the adapters into the DI orchestrator.
	svc := api.NewService(cfg, st, dc, vc, engine)
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
	// VMs job calls BackupVM (wired via SetVMJob below).
	scheduler := schedule.New(
		func(name string) error {
			_, bErr := svc.Backup(context.Background(), name)
			return bErr
		},
		st.ListTargets,
	)
	scheduler.SetVMJob(
		func(name string) error {
			_, bErr := svc.BackupVM(context.Background(), name)
			if errors.Is(bErr, backup.ErrVMNotInstalled) {
				return nil // VM no longer on the host: a skip (already logged), not a job failure
			}
			return bErr
		},
		st.ListVMTargets,
	)
	scheduler.SetFlashJob(func() error {
		_, bErr := svc.BackupFlash(context.Background())
		return bErr
	})
	scheduler.SetOffsiteJob(func(domain string) error {
		return svc.ReplicateOffsite(context.Background(), domain)
	})
	scheduler.SetDrillJob(func(domain string) error {
		_, dErr := svc.RunRestoreDrill(context.Background(), domain, "local")
		return dErr
	})
	scheduler.SetTamperJob(func(domain string) error {
		_, tErr := svc.RunTamperTest(context.Background(), domain)
		return tErr
	})
	// Per-domain LastRunFuncs: the everyN due-gate queries the most recent
	// successful backup within each domain (containers / VMs / flash scoped separately).
	containersLastRun := schedule.LastRunFunc(st.LastSuccessfulContainerBackup)
	vmsLastRun := schedule.LastRunFunc(st.LastSuccessfulVMBackup)
	flashLastRun := schedule.LastRunFunc(st.LastSuccessfulFlashBackup)

	if settings, sErr := st.GetSettings(); sErr == nil {
		if rErr := scheduler.ReloadWithDueChecks(settings, containersLastRun, vmsLastRun, flashLastRun); rErr != nil {
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
