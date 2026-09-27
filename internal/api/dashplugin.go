package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// The companion dashboard tile plugin is installed and removed over the host
// SSH connection with Unraid's own plugin command, so it stays manageable under
// Plugins. The endpoints are not on the authGate allowlist because they act on
// the host, and every remote command is a constant with no user input in it.

// dashPluginURL is the companion plugin's .plg install URL.
const dashPluginURL = "https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg"

// dashPluginMarker is where the plugin manager links every installed plugin's
// .plg. `[ -e ]` follows the link, so a dangling one counts as not installed.
const dashPluginMarker = "/var/log/plugins/bombvaultwidget.plg"

// dashPluginStatusCmd always exits 0, so "absent" is distinguishable from an
// SSH failure. When installed, the second line is the plugin's version.
const dashPluginStatusCmd = "if [ -e " + dashPluginMarker + " ]; then echo INSTALLED; " +
	"/usr/local/sbin/plugin version " + dashPluginMarker + " 2>/dev/null; else echo ABSENT; fi"

// The install and remove commands fold stderr into stdout, so the transcript
// reaches the UI even on failure; sshconn.Run keeps stdout on error.
const (
	dashPluginInstallCmd = "/usr/local/sbin/plugin install " + dashPluginURL + " 2>&1"
	dashPluginRemoveCmd  = "/usr/local/sbin/plugin remove bombvaultwidget.plg 2>&1"
)

// The status timeout covers the SSH layer's 10s connect timeout plus a quick
// probe. Install downloads the plugin payload from GitHub.
const (
	dashPluginStatusTimeout  = 15 * time.Second
	dashPluginInstallTimeout = 120 * time.Second
	dashPluginRemoveTimeout  = 60 * time.Second
)

// dashPluginOutputMax caps the transcript tail returned to the UI.
const dashPluginOutputMax = 1500

var errDashPluginNoSSH = errors.New("host SSH is not configured (set it up in Settings, System, Host SSH)")

// dashOutputTail returns the last dashPluginOutputMax bytes of a transcript,
// where a plugin install puts its failure reason.
func dashOutputTail(out string) string {
	out = strings.TrimSpace(out)
	if len(out) > dashPluginOutputMax {
		return out[len(out)-dashPluginOutputMax:]
	}
	return out
}

// DashboardPluginStatus reports whether the companion plugin is installed on
// the host, and its version when readable.
func (s *Service) DashboardPluginStatus(ctx context.Context) (installed bool, version string, err error) {
	if s.ssh == nil {
		return false, "", errDashPluginNoSSH
	}
	ctx, cancel := context.WithTimeout(ctx, dashPluginStatusTimeout)
	defer cancel()
	out, err := s.ssh.Run(ctx, "sh", "-c", dashPluginStatusCmd)
	if err != nil {
		return false, "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "INSTALLED" {
		return false, "", nil
	}
	if len(lines) > 1 {
		version = strings.TrimSpace(lines[1])
	}
	return true, version, nil
}

// InstallDashboardPlugin installs the companion plugin on the host and returns
// the transcript tail, also on failure. It ignores the request context, since a
// closed browser tab must not leave a half-installed plugin.
func (s *Service) InstallDashboardPlugin(_ context.Context) (output string, err error) {
	return s.runDashPluginCmd(dashPluginInstallCmd, dashPluginInstallTimeout)
}

// RemoveDashboardPlugin removes the companion plugin, with the same contract as
// InstallDashboardPlugin.
func (s *Service) RemoveDashboardPlugin(_ context.Context) (output string, err error) {
	return s.runDashPluginCmd(dashPluginRemoveCmd, dashPluginRemoveTimeout)
}

// runDashPluginCmd runs one of the plugin commands on the host under its own
// timeout and returns the transcript tail.
func (s *Service) runDashPluginCmd(cmd string, timeout time.Duration) (string, error) {
	// The plugin command only exists on Unraid. The mismatch error names the
	// detected platform and the /host/boot mount check, for an Unraid host whose
	// flash is not mounted into the container.
	if s.platformFn().Kind() != platform.KindUnraid {
		return "", s.unraidPlatformMismatchError("the companion dashboard plugin")
	}
	if s.ssh == nil {
		return "", errDashPluginNoSSH
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := s.ssh.Run(ctx, "sh", "-c", cmd)
	return dashOutputTail(out), err
}

// handleDashboardPluginStatus serves GET /api/dashboard-plugin. Without SSH the
// installed state is unknown, so it answers ok with only sshConfigured:false.
func (h *Handler) handleDashboardPluginStatus(w http.ResponseWriter, r *http.Request) {
	if h.svc.ssh == nil {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"sshConfigured": false}))
		return
	}
	installed, version, err := h.svc.DashboardPluginStatus(r.Context())
	if err != nil {
		env := failEnvelope(err)
		env["sshConfigured"] = true
		writeJSON(w, http.StatusOK, env)
		return
	}
	resp := map[string]any{"sshConfigured": true, "installed": installed}
	if version != "" {
		resp["version"] = version
	}
	writeJSON(w, http.StatusOK, okEnvelope(resp))
}

// handleDashboardPluginInstall serves POST /api/dashboard-plugin/install.
func (h *Handler) handleDashboardPluginInstall(w http.ResponseWriter, r *http.Request) {
	h.runDashPluginHandler(w, r, h.svc.InstallDashboardPlugin)
}

// handleDashboardPluginRemove serves POST /api/dashboard-plugin/remove.
func (h *Handler) handleDashboardPluginRemove(w http.ResponseWriter, r *http.Request) {
	h.runDashPluginHandler(w, r, h.svc.RemoveDashboardPlugin)
}

// runDashPluginHandler returns the transcript tail with the fail envelope too,
// so the UI can show why the plugin command refused.
func (h *Handler) runDashPluginHandler(w http.ResponseWriter, r *http.Request, op func(context.Context) (string, error)) {
	out, err := op(r.Context())
	if err != nil {
		env := failEnvelope(err)
		if out != "" {
			env["output"] = out
		}
		writeJSON(w, http.StatusOK, env)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"output": out}))
}
