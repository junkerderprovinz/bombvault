package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func hostSSHTestResult(t *testing.T, ssh HostSSH) map[string]any {
	t.Helper()
	h := &Handler{svc: &Service{ssh: ssh}}
	rec := httptest.NewRecorder()
	h.handleVMSSHTest(rec, httptest.NewRequest(http.MethodPost, "/api/vm/ssh/test", nil))
	return dashDecode(t, rec)
}

// ZFS and Unraid notifications need only the SSH link, so a host without
// libvirt still passes and the missing libvirt is reported for VMs alone.
func TestHostSSHTestPassesWithoutLibvirt(t *testing.T) {
	body := hostSSHTestResult(t, &fakeHostSSH{testErr: errors.New("virsh: command not found")})
	if body["ok"] != true {
		t.Fatalf("a working SSH link must pass, got %v", body)
	}
	if body["libvirt"] != false {
		t.Fatalf("libvirt must read as unreachable, got %v", body)
	}
	if msg, _ := body["libvirtError"].(string); !strings.Contains(msg, "virsh: command not found") {
		t.Fatalf("libvirtError must carry the cause, got %v", body)
	}
}

func TestHostSSHTestReportsLibvirt(t *testing.T) {
	body := hostSSHTestResult(t, &fakeHostSSH{})
	if body["ok"] != true || body["libvirt"] != true {
		t.Fatalf("want ok with libvirt reachable, got %v", body)
	}
}

func TestHostSSHTestFailsWhenSSHFails(t *testing.T) {
	body := hostSSHTestResult(t, &fakeHostSSH{knownHostErr: errors.New("Permission denied (publickey)")})
	if body["ok"] != false {
		t.Fatalf("a refused SSH login must fail the test, got %v", body)
	}
}

// Switching VMs on still needs libvirt, not only the SSH link.
func TestVMSSHTestNeedsLibvirt(t *testing.T) {
	s := &Service{ssh: &fakeHostSSH{testErr: errors.New("virsh: command not found")}}
	if err := s.VMSSHTest(t.Context()); err == nil {
		t.Fatal("VMSSHTest must fail without libvirt")
	}
}
