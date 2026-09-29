package api

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/homeassistant"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Home Assistant finds BombVault through MQTT discovery, so the whole link is
// a broker connection: sensors read the same status the dashboard shows, and
// a domain's button starts that domain the way POST /api/v1/backups does,
// under the start limits every caller outside the web interface shares.

var (
	mqttHostRe   = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,253}$`)
	mqttPrefixRe = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)

	errMQTTHost   = errors.New("enter the broker's host name or IP address")
	errMQTTPort   = errors.New("the port is a number from 1 to 65535")
	errMQTTPrefix = errors.New("the topic prefix takes letters, digits, dashes, underscores and slashes between them")
)

func (h *Handler) newHomeAssistantBridge() *homeassistant.Bridge {
	return homeassistant.NewBridge(homeassistant.Source{
		Device: h.haDevice,
		State:  h.haState,
		Start:  h.haStart,
	})
}

// haConfig turns the stored settings into a bridge configuration. A password
// the APP_KEY no longer opens is left out, and the broker's refusal says the
// rest.
func (h *Handler) haConfig(s store.MQTTSettings) homeassistant.Config {
	var password string
	if len(s.PasswordEnc) > 0 {
		plain, err := secret.Decrypt(h.cfg.AppKey, s.PasswordEnc)
		if err != nil {
			log.Printf("homeassistant: the stored broker password cannot be read: %v", err)
		} else {
			password = string(plain)
		}
	}
	return homeassistant.Config{
		Enabled:  s.Enabled,
		Host:     strings.Trim(s.Host, "[]"),
		Port:     s.Port,
		Username: s.Username,
		Password: password,
		TLS:      s.TLS,
		Prefix:   s.Prefix,
		Buttons:  s.Buttons,
		Node:     s.NodeID,
	}
}

func (h *Handler) haDevice() homeassistant.Device {
	name := "BombVault"
	if s, err := h.store.GetSettings(); err == nil && s.InstanceName != "" {
		name = "BombVault (" + s.InstanceName + ")"
	}
	return homeassistant.Device{Name: name, Version: Version}
}

// haState reads what the sensors show. The overall status is the worst of the
// switched-on domains: failed for an overdue domain or a failed last backup,
// warning for one that is late or has never been backed up.
func (h *Handler) haState(ctx context.Context) (homeassistant.State, error) {
	settings, err := h.store.GetSettings()
	if err != nil {
		return homeassistant.State{}, err
	}
	statuses, err := h.svc.domainStatusFrom(settings)
	if err != nil {
		return homeassistant.State{}, err
	}
	st := homeassistant.State{Status: "off", Domains: map[string]homeassistant.DomainState{}}
	for _, d := range statuses {
		if !d.Enabled {
			continue
		}
		ds := homeassistant.DomainState{LastResult: h.lastBackupResult(d.Domain)}
		if d.LastSuccess > 0 {
			ds.LastBackup = time.Unix(d.LastSuccess, 0)
		}
		for _, c := range h.svc.repoCapacities(d.Domain) {
			if c.Primary {
				ds.FreeBytes = c.Free
			}
		}
		st.Domains[d.Domain] = ds
		switch {
		case d.Status == "overdue" || ds.LastResult == "failed":
			st.Status = "failed"
		case (d.Status == "warn" || d.Status == "never") && st.Status != "failed":
			st.Status = "warning"
		case st.Status == "off":
			st.Status = "ok"
		}
	}
	st.Running = h.haRunning()
	open := h.svc.AnomalySummary(ctx).Open
	st.Anomalies = open.Critical + open.Warning + open.Info
	if h.scheduler != nil {
		for _, next := range h.scheduler.NextRuns() {
			if next.Job == "backup" && (st.NextRun.IsZero() || next.Next.Before(st.NextRun)) {
				st.NextRun = next.Next
			}
		}
	}
	return st, nil
}

// lastBackupResult is how the newest finished backup of a domain ended, or ""
// before the first one.
func (h *Handler) lastBackupResult(domain string) string {
	ids, err := h.mcpDomainTargetIDs(domain)
	if err != nil {
		return ""
	}
	runs, err := h.store.ListRunsFiltered(store.RunFilter{
		TargetIDs: ids,
		Kinds:     []string{"backup"},
		Statuses:  []string{"success", "failed", "cancelled", "skipped"},
		Limit:     1,
	})
	if err != nil || len(runs) == 0 {
		return ""
	}
	return runs[0].Status
}

// haRunning names what runs now in one short line, "idle" when nothing does.
func (h *Handler) haRunning() string {
	items := h.svc.ActivitySnapshot().Items
	if len(items) == 0 {
		return "idle"
	}
	slices.SortFunc(items, func(a, b ActivityItem) int {
		return cmp.Or(cmp.Compare(a.Domain, b.Domain), cmp.Compare(a.Item, b.Item))
	})
	// A whole-domain run reports its domain and no item.
	first := items[0]
	line := fmt.Sprintf("%s: %s %d%%", cmp.Or(first.Item, first.Domain), first.Phase, int(first.Percent))
	if len(items) > 1 {
		line += fmt.Sprintf(" (+%d)", len(items)-1)
	}
	return line
}

// haStart backs up a domain for a button press, through the same tool and
// limits as an API start. The permission is the buttons switch as it is
// stored now.
func (h *Handler) haStart(ctx context.Context, domain string) string {
	s, err := h.store.GetMQTTSettings()
	if err != nil {
		return "unavailable"
	}
	ctx = withMCPCaller(ctx, mcpCaller{
		Via:             viaMQTT,
		Label:           "Home Assistant",
		CanStartBackups: s.Buttons,
		ClientAddr:      "mqtt",
	})
	res, err := callMCPTool(h, ctx, (*Handler).toolStartDomainBackup, map[string]any{"domain": domain})
	if err != nil {
		return "failed: " + err.Error()
	}
	if res.IsError {
		return mcpErrorCodeOf(res)
	}
	return "started"
}

// StartIntegrations connects to the Home Assistant broker and starts the
// network announcement, each when it is switched on. The announcement probes
// its names for a second or so, which the boot does not wait for.
func (h *Handler) StartIntegrations() {
	if on, err := h.store.MDNSEnabled(); err != nil {
		log.Printf("mdns: read the switch: %v", err)
	} else if on {
		go h.applyMDNS(true)
	}
	s, err := h.store.GetMQTTSettings()
	if err != nil {
		log.Printf("homeassistant: could not read the settings: %v", err)
		return
	}
	if s.Enabled {
		if err := h.ha.Apply(h.haConfig(s)); err != nil {
			log.Printf("homeassistant: %v", err)
		}
	}
}

// StopIntegrations marks BombVault unavailable in Home Assistant, sends the
// mDNS goodbye and disconnects both.
func (h *Handler) StopIntegrations() {
	h.ha.Close()
	h.stopMDNS()
}

type homeAssistantView struct {
	Enabled     bool                 `json:"enabled"`
	Host        string               `json:"host"`
	Port        int                  `json:"port"`
	Username    string               `json:"username"`
	PasswordSet bool                 `json:"passwordSet"`
	TLS         bool                 `json:"tls"`
	Prefix      string               `json:"prefix"`
	Buttons     bool                 `json:"buttons"`
	NodeID      string               `json:"nodeId"`
	Status      homeassistant.Status `json:"status"`
}

func (h *Handler) homeAssistantView(s store.MQTTSettings) homeAssistantView {
	return homeAssistantView{
		Enabled:     s.Enabled,
		Host:        s.Host,
		Port:        s.Port,
		Username:    s.Username,
		PasswordSet: len(s.PasswordEnc) > 0,
		TLS:         s.TLS,
		Prefix:      s.Prefix,
		Buttons:     s.Buttons,
		NodeID:      s.NodeID,
		Status:      h.ha.Status(),
	}
}

func (h *Handler) handleGetHomeAssistant(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetMQTTSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"settings":         h.homeAssistantView(s),
		"cooldownMinutes":  int(mcpStartCooldown / time.Minute),
		"itemStartsPerDay": mcpItemStartsPerDay,
	}))
}

// handleSetHomeAssistant saves the broker settings and reconnects. A password
// left empty keeps the stored one; clearing the user name clears both.
func (h *Handler) handleSetHomeAssistant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled  bool   `json:"enabled"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		TLS      bool   `json:"tls"`
		Prefix   string `json:"prefix"`
		Buttons  bool   `json:"buttons"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	s, err := h.store.GetMQTTSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	body.Host = strings.TrimSpace(body.Host)
	body.Prefix = strings.TrimSpace(body.Prefix)
	if code, err := mqttSettingsRefusal(body.Enabled, body.Host, body.Port, body.Prefix); err != nil {
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, code))
		return
	}

	s.Enabled, s.Host, s.Port, s.TLS, s.Prefix, s.Buttons = body.Enabled, body.Host, body.Port, body.TLS, body.Prefix, body.Buttons
	s.Username = strings.TrimSpace(body.Username)
	if err := h.sealBrokerPassword(&s, body.Password); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if s.NodeID == "" {
		s.NodeID = newNodeID()
	}
	if err := h.store.SaveMQTTSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := map[string]any{}
	if err := h.ha.Apply(h.haConfig(s)); err != nil {
		log.Printf("homeassistant: %v", err)
		out["warning"] = "mqtt-remove-failed"
	}
	out["settings"] = h.homeAssistantView(s)
	writeJSON(w, http.StatusOK, okEnvelope(out))
}

// mqttSettingsRefusal says why broker settings cannot be stored, with the code
// the card translates. The settings import applies the same checks.
func mqttSettingsRefusal(enabled bool, host string, port int, prefix string) (string, error) {
	switch {
	case host != "" && !mqttHostRe.MatchString(host), enabled && host == "":
		return "mqtt-host-invalid", errMQTTHost
	case port < 1 || port > 65535:
		return "mqtt-port-invalid", errMQTTPort
	case len(prefix) > 64 || !mqttPrefixRe.MatchString(prefix):
		return "mqtt-prefix-invalid", errMQTTPrefix
	}
	return "", nil
}

// sealBrokerPassword stores password in s. An empty one keeps the stored
// password, and an empty user name clears it.
func (h *Handler) sealBrokerPassword(s *store.MQTTSettings, password string) error {
	switch {
	case s.Username == "":
		s.PasswordEnc = nil
	case password != "":
		sealed, err := secret.Encrypt(h.cfg.AppKey, []byte(password))
		if err != nil {
			return err
		}
		s.PasswordEnc = sealed
	}
	return nil
}

// newNodeID names this instance in the topics: short, and random so two
// instances on one broker never meet.
func newNodeID() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("newNodeID: %v", err))
	}
	return hex.EncodeToString(buf)
}
