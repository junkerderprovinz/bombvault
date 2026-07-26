package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Companion Unraid dashboard-tile plugin — GET /api/dashboard-plugin (status)
// + POST /api/dashboard-plugin/install + POST /api/dashboard-plugin/remove.
//
// BombVault can install/remove the bombvaultdash companion plugin (a BombVault
// status tile on the Unraid Dashboard) over the SAME host SSH connection the
// VM/NVRAM features and Unraid notifications already use. Everything runs
// Unraid's regular plugin mechanism (`plugin install <url>` / `plugin remove
// <name>.plg`), so the plugin shows up under Plugins like any other and stays
// removable there. All three endpoints are session-protected (they modify or
// probe the HOST, so they are deliberately NOT on the authGate public
// allowlist), and every remote command is a hard-coded constant — zero user
// input is ever interpolated into the SSH command line.
// ---------------------------------------------------------------------------

// dashPluginURL is the companion plugin's .plg install URL (the Unraid plugin
// manager downloads the txz payload it references from the matching GitHub
// release).
const dashPluginURL = "https://raw.githubusercontent.com/junkerderprovinz/bombvault-dashboard/main/plugin/bombvaultdash.plg"

// dashPluginMarker is Unraid's canonical installed-plugin marker: the plugin
// manager symlinks every installed plugin's .plg into /var/log/plugins/<name>.plg
// (the webGui's Plugins page enumerates exactly that directory), and `plugin
// remove` takes the same basename. `[ -e ]` follows the symlink, so a dangling
// link (plugin file gone from the flash) counts as not installed.
const dashPluginMarker = "/var/log/plugins/bombvaultdash.plg"

// dashPluginStatusCmd probes the marker in ONE round-trip and always exits 0 so
// an "absent" answer is distinguishable from an SSH failure (a bare `test -e`
// would make both look like a non-zero exit). When installed, the second line
// is the plugin's version entity via the plugin CLI's attribute lookup.
const dashPluginStatusCmd = "if [ -e " + dashPluginMarker + " ]; then echo INSTALLED; " +
	"/usr/local/sbin/plugin version " + dashPluginMarker + " 2>/dev/null; else echo ABSENT; fi"

// dashPluginInstallCmd / dashPluginRemoveCmd are the exact host commands.
// 2>&1 folds the plugin CLI's stderr into stdout so the transcript tail can be
// returned to the UI even on failure (sshconn.Run keeps stdout on error).
const (
	dashPluginInstallCmd = "/usr/local/sbin/plugin install " + dashPluginURL + " 2>&1"
	dashPluginRemoveCmd  = "/usr/local/sbin/plugin remove bombvaultdash.plg 2>&1"
)

// Timeouts: the status probe is a quick marker test (the SSH layer's own
// ConnectTimeout is 10s, so 15s covers connect + probe, mirroring
// sendUnraidNotify). Install downloads the release txz from GitHub — generous
// 120s. Remove only deletes local files.
const (
	dashPluginStatusTimeout  = 15 * time.Second
	dashPluginInstallTimeout = 120 * time.Second
	dashPluginRemoveTimeout  = 60 * time.Second
)

// dashPluginOutputMax caps the transcript tail returned to the UI — enough to
// show why an install failed without shipping the whole download log.
const dashPluginOutputMax = 1500

// errDashPluginNoSSH is the clean not-configured answer, mirroring the house
// nil-guard wording (VMSSHInfo / sendUnraidNotify).
var errDashPluginNoSSH = errors.New("host SSH is not configured (set it up in Settings → VM Backup over SSH)")

// dashOutputTail returns the LAST dashPluginOutputMax bytes of a command
// transcript (the failure reason is at the end of a plugin install log).
func dashOutputTail(out string) string {
	out = strings.TrimSpace(out)
	if len(out) > dashPluginOutputMax {
		return out[len(out)-dashPluginOutputMax:]
	}
	return out
}

// DashboardPluginStatus reports whether the companion dashboard plugin is
// installed on the host, and its version when readable. sshConfigured=false
// (nil HostSSH) means the state is unknowable — the handler reports that
// instead of erroring so the UI can show manual instructions.
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

// InstallDashboardPlugin runs the hard-coded `plugin install <url>` on the host
// and returns the transcript tail (also on failure). The exec context is
// detached from the HTTP request on purpose (same lesson as #93, smaller
// scale): a closed browser tab must not cancel a plugin install midway and
// leave a half-installed plugin — the operation finishes or times out on its
// own 120s ceiling either way.
func (s *Service) InstallDashboardPlugin(_ context.Context) (output string, err error) {
	return s.runDashPluginCmd(dashPluginInstallCmd, dashPluginInstallTimeout)
}

// RemoveDashboardPlugin runs the hard-coded `plugin remove bombvaultdash.plg`
// on the host, same contract as InstallDashboardPlugin.
func (s *Service) RemoveDashboardPlugin(_ context.Context) (output string, err error) {
	return s.runDashPluginCmd(dashPluginRemoveCmd, dashPluginRemoveTimeout)
}

// runDashPluginCmd executes one of the constant plugin commands detached from
// any request context and returns the truncated transcript tail.
func (s *Service) runDashPluginCmd(cmd string, timeout time.Duration) (string, error) {
	if s.ssh == nil {
		return "", errDashPluginNoSSH
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := s.ssh.Run(ctx, "sh", "-c", cmd)
	return dashOutputTail(out), err
}

// ---------------------------------------------------------------------------
// handlers
// ---------------------------------------------------------------------------

// handleDashboardPluginStatus — GET /api/dashboard-plugin.
// {ok, sshConfigured, installed, version?}: without SSH the installed state is
// unknown, so only sshConfigured:false is reported (ok:true — that is a valid
// answer, not a failure); an SSH probe error is a normal fail envelope.
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

// handleDashboardPluginInstall — POST /api/dashboard-plugin/install.
func (h *Handler) handleDashboardPluginInstall(w http.ResponseWriter, r *http.Request) {
	h.runDashPluginHandler(w, r, h.svc.InstallDashboardPlugin)
}

// handleDashboardPluginRemove — POST /api/dashboard-plugin/remove.
func (h *Handler) handleDashboardPluginRemove(w http.ResponseWriter, r *http.Request) {
	h.runDashPluginHandler(w, r, h.svc.RemoveDashboardPlugin)
}

// runDashPluginHandler shares the install/remove response contract: ok + the
// transcript tail, or a fail envelope that still carries the tail so the UI
// can show WHY the plugin CLI refused.
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
