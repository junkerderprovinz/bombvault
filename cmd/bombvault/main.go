// Command bombvault is the single static binary: it loads config, opens the
// SQLite store, wires the real adapters into the backup service, starts the
// per-domain scheduler, and serves the JSON API + embedded React SPA.
package main

import (
	"context"
	"log"
	"os"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
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

func run() error {
	// Send the standard logger to stdout so all runtime logs share ONE stream
	// with the ASCII banner (printed via fmt to stdout). Otherwise Docker/Unraid
	// interleaves stderr (log default) above the stdout banner.
	log.SetOutput(os.Stdout)

	cfg, err := config.LoadFromEnv()
	if err != nil {
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

	// Real Docker adapter over the mounted docker.sock.
	dc, err := dockercli.New()
	if err != nil {
		return err
	}
	defer func() { _ = dc.Close() }()

	// Real virsh adapter over the mounted libvirt socket. The container mounts the
	// host run PARENT (HostRunRoot) — not /var/run/libvirt directly, which would
	// pin that dir and break the host VM Manager on toggle — so symlink the socket
	// to where virsh expects it. Best-effort: a missing mount just means no VM
	// backup, surfaced by the host-integration probe, not a fatal error.
	if err := virshcli.LinkSocket(cfg.HostRunRoot, "/var/run/libvirt"); err != nil {
		log.Printf("libvirt: link socket: %v", err)
	}
	vc := virshcli.New()

	// Real restic CLI adapter.
	engine := &restic.Restic{Bin: "restic"}

	// Backup service bridges the adapters into the DI orchestrator.
	svc := api.NewService(cfg, st, dc, vc, engine)

	// Per-domain scheduler; the containers job calls the service's Backup.
	scheduler := schedule.New(
		func(name string) error {
			_, bErr := svc.Backup(context.Background(), name)
			return bErr
		},
		st.ListTargets,
	)
	// Build the containers LastRunFunc: the everyN due-gate queries the most
	// recent successful backup across all container targets.
	containersLastRun := schedule.LastRunFunc(st.LastSuccessfulContainerBackup)

	if settings, sErr := st.GetSettings(); sErr == nil {
		if rErr := scheduler.ReloadWithDueChecks(settings, containersLastRun, nil, nil); rErr != nil {
			log.Printf("scheduler: initial reload failed: %v", rErr)
		}
	} else {
		log.Printf("scheduler: could not read settings: %v", sErr)
	}
	scheduler.Start()
	defer scheduler.Stop()

	// JSON API + embedded SPA.
	handler := api.NewHandler(cfg, st, dc, svc, scheduler, spike.DefaultProbes())
	// Warm the host-integration check at startup so the dashboard shows the
	// green result list immediately on first load (no manual click needed).
	go handler.WarmSpike()
	server := api.NewServer(cfg, web.DistFS(), handler.Router())
	return server.Run()
}
