package api

import (
	"context"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

// ComponentVersions are the versions the Info page lists: BombVault's own and
// the two tools it ships with. A tool whose probe failed is null.
type ComponentVersions struct {
	BombVault string  `json:"bombvault"`
	Restic    *string `json:"restic"`
	Rclone    *string `json:"rclone"`
}

// toolVersionTimeout bounds one version command, so a binary that hangs cannot
// hold the page.
const toolVersionTimeout = 5 * time.Second

var (
	resticVersionRe = regexp.MustCompile(`^restic (\d+(?:\.\d+)+\S*)`)
	rcloneVersionRe = regexp.MustCompile(`^rclone v(\d+(?:\.\d+)+\S*)`)
)

// versionCommand runs one tool's version command and returns what it printed.
type versionCommand func(ctx context.Context, tool string) ([]byte, error)

func runVersionCommand(ctx context.Context, tool string) ([]byte, error) {
	return exec.CommandContext(ctx, tool, "version").Output() //nolint:gosec // G204: tool is one of two fixed binary names
}

// toolVersions reads the versions of restic and rclone once and keeps them:
// both ship in the image and cannot change under a running process.
var toolVersions = sync.OnceValue(func() ComponentVersions {
	return probeToolVersions(runVersionCommand)
})

func probeToolVersions(run versionCommand) ComponentVersions {
	return ComponentVersions{
		Restic: probeToolVersion(run, "restic", resticVersionRe),
		Rclone: probeToolVersion(run, "rclone", rcloneVersionRe),
	}
}

func probeToolVersion(run versionCommand, tool string, pattern *regexp.Regexp) *string {
	ctx, cancel := context.WithTimeout(context.Background(), toolVersionTimeout)
	defer cancel()
	out, err := run(ctx, tool)
	if err != nil {
		log.Printf("versions: %s version: %v", tool, err)
		return nil
	}
	m := pattern.FindSubmatch(out)
	if m == nil {
		log.Printf("versions: %s version printed no version number", tool)
		return nil
	}
	version := string(m[1])
	return &version
}

func (h *Handler) handleVersions(w http.ResponseWriter, _ *http.Request) {
	versions := toolVersions()
	versions.BombVault = Version
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"versions": versions}))
}
