package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// dashHandler returns a Handler whose Service has only a HostSSH, the one thing
// the dashboard plugin endpoints use.
func dashHandler(ssh HostSSH) *Handler {
	return &Handler{svc: &Service{ssh: ssh}}
}

func dashDecode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return body
}

// TestDashboardPluginStatusNoSSH: without host SSH the status is unknown, so
// the endpoint answers ok with sshConfigured:false and the UI shows the manual
// install instructions.
func TestDashboardPluginStatusNoSSH(t *testing.T) {
	h := dashHandler(nil)
	rec := httptest.NewRecorder()
	h.handleDashboardPluginStatus(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard-plugin", nil))
	body := dashDecode(t, rec)
	if body["ok"] != true {
		t.Fatalf("no-SSH status must still be ok:true, got %v", body)
	}
	if body["sshConfigured"] != false {
		t.Fatalf("sshConfigured must be false without SSH, got %v", body)
	}
	if _, present := body["installed"]; present {
		t.Fatalf("installed is unknowable without SSH and must be absent, got %v", body)
	}
}

// TestDashboardPluginInstallRemoveNoSSH: without SSH, install and remove return
// the "not configured" error instead of panicking.
func TestDashboardPluginInstallRemoveNoSSH(t *testing.T) {
	h := dashHandler(nil)
	for _, ep := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"install", h.handleDashboardPluginInstall},
		{"remove", h.handleDashboardPluginRemove},
	} {
		rec := httptest.NewRecorder()
		ep.call(rec, jsonReq(http.MethodPost, "/api/dashboard-plugin/"+ep.name, nil))
		body := dashDecode(t, rec)
		if body["ok"] != false {
			t.Fatalf("%s without SSH must fail, got %v", ep.name, body)
		}
		msg, _ := body["error"].(string)
		if !strings.Contains(msg, "not configured") {
			t.Fatalf("%s without SSH must say the SSH connection is not configured, got %q", ep.name, msg)
		}
	}
}

// TestDashboardPluginStatusInstalled: the status probe runs the fixed marker
// check and reads the version from its second line.
func TestDashboardPluginStatusInstalled(t *testing.T) {
	ssh := &fakeHostSSH{runOut: "INSTALLED\n2026.07.26"}
	h := dashHandler(ssh)
	rec := httptest.NewRecorder()
	h.handleDashboardPluginStatus(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard-plugin", nil))

	if len(ssh.runs) != 1 {
		t.Fatalf("expected exactly one SSH round-trip, got %d", len(ssh.runs))
	}
	// Compare against the literal rather than the constant, so any edit to the
	// command shows up here.
	want := []string{"sh", "-c",
		"if [ -e /var/log/plugins/bombvaultwidget.plg ]; then echo INSTALLED; " +
			"/usr/local/sbin/plugin version /var/log/plugins/bombvaultwidget.plg 2>/dev/null; else echo ABSENT; fi"}
	if got := ssh.runs[0]; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("status command drifted:\n got %q\nwant %q", got, want)
	}

	body := dashDecode(t, rec)
	if body["ok"] != true || body["sshConfigured"] != true || body["installed"] != true {
		t.Fatalf("expected ok+sshConfigured+installed, got %v", body)
	}
	if body["version"] != "2026.07.26" {
		t.Fatalf("version not parsed from the second output line, got %v", body)
	}
}

func TestDashboardPluginStatusAbsent(t *testing.T) {
	h := dashHandler(&fakeHostSSH{runOut: "ABSENT"})
	rec := httptest.NewRecorder()
	h.handleDashboardPluginStatus(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard-plugin", nil))
	body := dashDecode(t, rec)
	if body["ok"] != true || body["sshConfigured"] != true || body["installed"] != false {
		t.Fatalf("expected ok+sshConfigured+not-installed, got %v", body)
	}
	if _, present := body["version"]; present {
		t.Fatalf("no version expected when absent, got %v", body)
	}
}

// TestDashboardPluginStatusSSHError: an SSH failure still reports
// sshConfigured:true, since configured but unreachable is not the same as not
// set up.
func TestDashboardPluginStatusSSHError(t *testing.T) {
	h := dashHandler(&fakeHostSSH{runErr: errors.New("connection refused")})
	rec := httptest.NewRecorder()
	h.handleDashboardPluginStatus(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard-plugin", nil))
	body := dashDecode(t, rec)
	if body["ok"] != false || body["sshConfigured"] != true {
		t.Fatalf("expected fail envelope with sshConfigured:true, got %v", body)
	}
}

// TestDashboardPluginInstallRunsPinnedCommand: install runs the fixed plugin
// command, with no user input in it, and returns the transcript.
func TestDashboardPluginInstallRunsPinnedCommand(t *testing.T) {
	ssh := &fakeHostSSH{runOut: "plugin: installing: bombvaultwidget.plg\nplugin: bombvaultwidget.plg installed"}
	h := dashHandler(ssh)
	rec := httptest.NewRecorder()
	h.handleDashboardPluginInstall(rec, jsonReq(http.MethodPost, "/api/dashboard-plugin/install", nil))

	if len(ssh.runs) != 1 {
		t.Fatalf("expected exactly one SSH round-trip, got %d", len(ssh.runs))
	}
	want := []string{"sh", "-c",
		"/usr/local/sbin/plugin install https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg 2>&1"}
	if got := ssh.runs[0]; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("install command drifted:\n got %q\nwant %q", got, want)
	}

	body := dashDecode(t, rec)
	if body["ok"] != true {
		t.Fatalf("expected ok install, got %v", body)
	}
	out, _ := body["output"].(string)
	if !strings.Contains(out, "installed") {
		t.Fatalf("install response should carry the transcript, got %q", out)
	}
}

func TestDashboardPluginRemoveRunsPinnedCommand(t *testing.T) {
	ssh := &fakeHostSSH{}
	h := dashHandler(ssh)
	rec := httptest.NewRecorder()
	h.handleDashboardPluginRemove(rec, jsonReq(http.MethodPost, "/api/dashboard-plugin/remove", nil))

	if len(ssh.runs) != 1 {
		t.Fatalf("expected exactly one SSH round-trip, got %d", len(ssh.runs))
	}
	want := []string{"sh", "-c", "/usr/local/sbin/plugin remove bombvaultwidget.plg 2>&1"}
	if got := ssh.runs[0]; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("remove command drifted:\n got %q\nwant %q", got, want)
	}
	if body := dashDecode(t, rec); body["ok"] != true {
		t.Fatalf("expected ok remove, got %v", body)
	}
}

// TestDashboardPluginInstallFailureCarriesOutputTail: a failed install still
// returns the transcript tail so the UI can show why.
func TestDashboardPluginInstallFailureCarriesOutputTail(t *testing.T) {
	ssh := &fakeHostSSH{
		runOut: "plugin: downloading: bombvaultwidget.txz\nplugin: bad file MD5",
		runErr: errors.New("sshconn: run \"sh\": exit status 1"),
	}
	h := dashHandler(ssh)
	rec := httptest.NewRecorder()
	h.handleDashboardPluginInstall(rec, jsonReq(http.MethodPost, "/api/dashboard-plugin/install", nil))
	body := dashDecode(t, rec)
	if body["ok"] != false {
		t.Fatalf("expected failure envelope, got %v", body)
	}
	out, _ := body["output"].(string)
	if !strings.Contains(out, "bad file MD5") {
		t.Fatalf("failure must carry the transcript tail, got %q", out)
	}
}

// TestDashOutputTailTruncates: the failure reason is at the end of an install
// log, so the tail is kept.
func TestDashOutputTailTruncates(t *testing.T) {
	long := strings.Repeat("x", dashPluginOutputMax) + "TAIL-MARKER"
	got := dashOutputTail(long)
	if len(got) != dashPluginOutputMax {
		t.Fatalf("tail length = %d, want %d", len(got), dashPluginOutputMax)
	}
	if !strings.HasSuffix(got, "TAIL-MARKER") {
		t.Fatal("truncation must keep the END of the transcript")
	}
}
