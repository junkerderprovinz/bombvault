package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTheAnnouncementNamesTheSchemeAndPortTheWebInterfaceUses(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	h.cfg.HTTPSPort, h.cfg.Port = 3443, 3000
	svc := h.mdnsService()
	if svc.Type != "_https._tcp" || svc.Port != 3443 || svc.Instance != "BombVault" || svc.Host != "bombvault" {
		t.Fatalf("service %+v", svc)
	}
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.InstanceName = "tower"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	h.cfg.HTTPOnly = true
	svc = h.mdnsService()
	if svc.Type != "_http._tcp" || svc.Port != 3000 || svc.Instance != "BombVault (tower)" {
		t.Fatalf("service %+v", svc)
	}
	if len(svc.TXT) != 2 || svc.TXT[0] != "version="+Version || svc.TXT[1] != "path=/" {
		t.Fatalf("TXT %v", svc.TXT)
	}
}

func TestSwitchingTheAnnouncementOffIsStored(t *testing.T) {
	_, router, repo, _ := newMCPGateHandler(t)
	r := httptest.NewRequest(http.MethodPut, "/api/mdns", strings.NewReader(`{"enabled":false}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"running":false`) {
		t.Fatalf("PUT answered %d %s", w.Code, w.Body)
	}
	if on, err := repo.MDNSEnabled(); err != nil || on {
		t.Fatalf("the switch reads %v (%v) after switching off", on, err)
	}
}
