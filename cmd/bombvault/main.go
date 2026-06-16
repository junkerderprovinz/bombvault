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

	// Real restic CLI adapter.
	engine := &restic.Restic{Bin: "restic"}

	// Backup service bridges the adapters into the DI orchestrator.
	svc := api.NewService(cfg, st, dc, vc, engine)
	svc.SetHostSSH(sc) // NVRAM transfer over SSH + the Settings key/test endpoints

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
			return bErr
		},
		st.ListVMTargets,
	)
	// Per-domain LastRunFuncs: the everyN due-gate queries the most recent
	// successful backup within each domain (containers vs. VMs scoped separately).
	containersLastRun := schedule.LastRunFunc(st.LastSuccessfulContainerBackup)
	vmsLastRun := schedule.LastRunFunc(st.LastSuccessfulVMBackup)

	if settings, sErr := st.GetSettings(); sErr == nil {
		if rErr := scheduler.ReloadWithDueChecks(settings, containersLastRun, vmsLastRun, nil); rErr != nil {
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
