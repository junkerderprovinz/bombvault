package platform

import (
	"context"
	"path"
	"time"
)

// Unraid implements Platform for Unraid: the /mnt/user/appdata and
// /mnt/user/domains shares, and the dynamix.docker.manager step over SSH that
// makes the Docker tab re-check a recreated container's update status.
type Unraid struct{}

var _ Platform = Unraid{}

func (Unraid) Kind() Kind { return KindUnraid }

// AppdataFallback returns the container's folder on the appdata share as a
// host path. hostMountRoot is unused because the share path is fixed on the
// host.
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

// unraidReconcileUpdateStatusPHP makes Unraid refresh its cached "update
// available" status for one image. It drops the image's cached entry and lets
// reloadUpdateStatus re-inspect the local image, so Unraid writes the status
// file through its own locked writer. The image is passed as a separate argv
// token and never interpolated into the source. The unset is needed because
// on Unraid 7.0.1 a bare reloadUpdateStatus trusts the cached digest and keeps
// the flag set.
const unraidReconcileUpdateStatusPHP = `require_once "/usr/local/emhttp/plugins/dynamix.docker.manager/include/DockerClient.php"; global $dockerManPaths; $img=DockerUtil::ensureImageTag($argv[1]); $s=DockerUtil::loadJSON($dockerManPaths["update-status"]); unset($s[$img]); DockerUtil::saveJSON($dockerManPaths["update-status"],$s); (new DockerUpdate())->reloadUpdateStatus($img);`

// ReconcileContainerUpdateStatus clears the Docker tab's stale "update
// available" banner for a container BombVault just recreated. Unraid rewrites
// its status file itself; writing the JSON from here would race its writer.
// A nil ssh or empty imageRef does nothing. The recheck contacts the
// registry, hence the long timeout.
func (Unraid) ReconcileContainerUpdateStatus(ctx context.Context, ssh SSHRunner, imageRef string) error {
	if ssh == nil || imageRef == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, err := ssh.Run(ctx, "php", "-r", unraidReconcileUpdateStatusPHP, "--", imageRef)
	return err
}
