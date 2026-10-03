package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/mdns"
)

// BombVault announces its web interface over mDNS so it can be found as
// bombvault.local. Inside Docker's default bridge network the announcement
// stays inside Docker; on br0, macvlan or the host network it reaches the LAN.
// It is on by default because in the bridge case it costs nothing.

// mdnsStartTimeout bounds the probing, which takes about a second per name.
const mdnsStartTimeout = 20 * time.Second

type mdnsState struct {
	mu        sync.Mutex
	responder *mdns.Responder
	err       string
}

func (h *Handler) mdnsService() mdns.Service {
	instance := "BombVault"
	if s, err := h.store.GetSettings(); err == nil && s.InstanceName != "" {
		instance = "BombVault (" + s.InstanceName + ")"
	}
	svc := mdns.Service{
		Instance: instance,
		Host:     "bombvault",
		Type:     "_https._tcp",
		Subtype:  "_bombvault",
		Port:     uint16(h.cfg.HTTPSPort), //nolint:gosec // G115: config.Load reads the port from the environment as a TCP port
		TXT:      []string{"version=" + Version, "path=/"},
	}
	if h.cfg.HTTPOnly {
		svc.Type = "_http._tcp"
		svc.Port = uint16(h.cfg.Port) //nolint:gosec // G115: as above
	}
	return svc
}

// applyMDNS stops the running announcement, saying goodbye, and starts a new
// one when on is set.
func (h *Handler) applyMDNS(on bool) {
	h.mdns.mu.Lock()
	defer h.mdns.mu.Unlock()
	if h.mdns.responder != nil {
		h.mdns.responder.Close()
		h.mdns.responder = nil
	}
	h.mdns.err = ""
	if !on {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), mdnsStartTimeout)
	defer cancel()
	r, err := mdns.Start(ctx, h.mdnsService())
	if err != nil {
		log.Printf("mdns: %v", err)
		h.mdns.err = err.Error()
		return
	}
	svc := r.Service()
	log.Printf("mdns: announcing %q as %s.local", svc.Instance, svc.Host)
	h.mdns.responder = r
}

func (h *Handler) stopMDNS() {
	h.mdns.mu.Lock()
	defer h.mdns.mu.Unlock()
	if h.mdns.responder != nil {
		h.mdns.responder.Close()
		h.mdns.responder = nil
	}
}

func (h *Handler) mdnsView() map[string]any {
	on, err := h.store.MDNSEnabled()
	if err != nil {
		log.Printf("mdns: read the switch: %v", err)
	}
	h.mdns.mu.Lock()
	defer h.mdns.mu.Unlock()
	view := map[string]any{"enabled": on, "running": h.mdns.responder != nil, "error": h.mdns.err, "url": "", "instance": ""}
	if h.mdns.responder != nil {
		svc := h.mdns.responder.Service()
		scheme := "https"
		if svc.Type == "_http._tcp" {
			scheme = "http"
		}
		view["url"] = scheme + "://" + svc.Host + ".local:" + strconv.Itoa(int(svc.Port)) + "/"
		view["instance"] = svc.Instance
	}
	return view
}

func (h *Handler) handleGetMDNS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(h.mdnsView()))
}

func (h *Handler) handleSetMDNS(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.store.SetMDNSEnabled(body.Enabled); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.applyMDNS(body.Enabled)
	writeJSON(w, http.StatusOK, okEnvelope(h.mdnsView()))
}
