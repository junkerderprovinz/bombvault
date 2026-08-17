package platform

import (
	"context"
	"path"
	"time"
)

// Unraid implements Platform for Unraid: the array's /mnt/user/appdata and
// /mnt/user/domains share conventions, and the emhttp/dynamix.docker.manager
// PHP-over-SSH step that makes Unraid's own Docker tab re-check a container's
// update status after BombVault recreates it (#116). Every literal here is
// bombvault's original, pre-Platform-seam behavior, reproduced exactly —
// nothing changed except where the code lives.
type Unraid struct{}

var _ Platform = Unraid{}

func (Unraid) Kind() Kind { return KindUnraid }

// AppdataFallback returns Unraid's conventional appdata-share path for a
// container as a HOST path (not yet translated to a container-visible
// mount) — the caller translates it through the configured host source/mount
// roots exactly as it already does for every discovered bind mount.
// hostMountRoot is unused: Unraid's share convention is a fixed HOST-side
// path, independent of where the container happens to have it mounted.
func (Unraid) AppdataFallback(_, containerName string) string {
	return path.Join("/mnt/user/appdata", containerName)
}

// ForeignContainerDestBase returns Unraid's local appdata share, translated
// to the container-visible mount root.
func (Unraid) ForeignContainerDestBase(hostMountRoot string) string {
	return path.Join(path.Clean(hostMountRoot), "user/appdata")
}

// ForeignVMDestBase returns Unraid's local VM domains share, translated to
// the container-visible mount root.
func (Unraid) ForeignVMDestBase(hostMountRoot string) string {
	return path.Join(path.Clean(hostMountRoot), "user/domains")
}

// unraidReconcileUpdateStatusPHP is the one-liner run on the Unraid host to
// make Unraid refresh its OWN cached "update available" status for a single
// container image (#116). It require_once's DockerClient.php (which also
// defines the $dockerManPaths used for the status file), invalidates the
// image's cached entry via DockerUtil so the stale local digest is dropped,
// then reloadUpdateStatus re-inspects the now-current local image and
// rewrites the status file through Unraid's own locked DockerUtil::saveJSON
// writer. The image tag is passed as a separate argv token (never
// interpolated into this source) to avoid injection and quoting problems.
// The leading unset is required: on Unraid 7.0.1 a bare reloadUpdateStatus
// trusts the cached local digest and would keep the flag set.
const unraidReconcileUpdateStatusPHP = `require_once "/usr/local/emhttp/plugins/dynamix.docker.manager/include/DockerClient.php"; global $dockerManPaths; $img=DockerUtil::ensureImageTag($argv[1]); $s=DockerUtil::loadJSON($dockerManPaths["update-status"]); unset($s[$img]); DockerUtil::saveJSON($dockerManPaths["update-status"],$s); (new DockerUpdate())->reloadUpdateStatus($img);`

// ReconcileContainerUpdateStatus asks Unraid to refresh its own cached update
// status for a container BombVault just recreated (#116), so the Docker tab's
// stale "update available" banner clears. It runs Unraid's own update-status
// recheck over the existing host SSH link (the same mechanism as
// sendUnraidNotify), which means Unraid rewrites its status file itself
// rather than BombVault hand-writing the JSON (which would race Unraid's
// writer). A nil ssh or empty imageRef is a silent no-op — the caller (see
// internal/api's reconcileUnraidUpdateStatus) is the non-fatal/best-effort
// boundary: it logs any returned error and never lets it affect the update
// outcome. The recheck does a registry round-trip, so it gets a generous
// timeout.
func (Unraid) ReconcileContainerUpdateStatus(ctx context.Context, ssh SSHRunner, imageRef string) error {
	if ssh == nil || imageRef == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, err := ssh.Run(ctx, "php", "-r", unraidReconcileUpdateStatusPHP, "--", imageRef)
	return err
}
