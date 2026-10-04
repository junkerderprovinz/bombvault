package api

import (
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

// homeAssistantExport is the Home Assistant card in a settings file. The broker
// password travels in the credentials block like every other secret. The node
// id stays with the instance, because two instances built from one file would
// otherwise publish to the same topics.
type homeAssistantExport struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	TLS      bool   `json:"tls"`
	Prefix   string `json:"prefix"`
	Buttons  bool   `json:"buttons"`
}

// exportIntegrations adds the Home Assistant settings and the network switch
// to an export.
func (h *Handler) exportIntegrations(exp *settingsExport) error {
	s, err := h.store.GetMQTTSettings()
	if err != nil {
		return err
	}
	exp.HomeAssistant = &homeAssistantExport{
		Enabled: s.Enabled, Host: s.Host, Port: s.Port, Username: s.Username,
		TLS: s.TLS, Prefix: s.Prefix, Buttons: s.Buttons,
	}
	on, err := h.store.MDNSEnabled()
	if err != nil {
		return err
	}
	exp.MDNSEnabled = &on
	return nil
}

// brokerPassword opens the stored Home Assistant broker password for a
// credentialed export.
func (h *Handler) brokerPassword() (string, error) {
	s, err := h.store.GetMQTTSettings()
	if err != nil {
		return "", err
	}
	if len(s.PasswordEnc) == 0 {
		return "", nil
	}
	plain, err := secret.Decrypt(h.cfg.AppKey, s.PasswordEnc)
	if err != nil {
		return "", fmt.Errorf("read the MQTT broker password: %w", err)
	}
	return string(plain), nil
}

// integrationsRefusal checks the Home Assistant block of a file the way the
// card checks a save.
func integrationsRefusal(exp settingsExport) string {
	ha := exp.HomeAssistant
	if ha == nil {
		return ""
	}
	if _, err := mqttSettingsRefusal(ha.Enabled, strings.TrimSpace(ha.Host), ha.Port, strings.TrimSpace(ha.Prefix)); err != nil {
		return "invalid Home Assistant settings: " + err.Error()
	}
	return ""
}

// integrationGroups names the integration areas an apply would change. The
// network switch counts when it is off, since on is the default.
func integrationGroups(exp settingsExport) []string {
	var groups []string
	if ha := exp.HomeAssistant; ha != nil && (ha.Enabled || ha.Host != "") {
		groups = append(groups, "homeAssistant")
	}
	if exp.MDNSEnabled != nil && !*exp.MDNSEnabled {
		groups = append(groups, "network")
	}
	return groups
}

// applyImportedIntegrations writes the Home Assistant settings and the network
// switch a file carries, then reconnects both. A file from a build without them
// leaves this instance's own in place.
func (h *Handler) applyImportedIntegrations(exp settingsExport) error {
	if v := exp.HomeAssistant; v != nil {
		s, err := h.store.GetMQTTSettings()
		if err != nil {
			return err
		}
		prev := s
		s.Enabled, s.Host, s.Port, s.TLS, s.Buttons = v.Enabled, strings.TrimSpace(v.Host), v.Port, v.TLS, v.Buttons
		s.Prefix = strings.TrimSpace(v.Prefix)
		s.Username = strings.TrimSpace(v.Username)
		var password string
		if exp.Credentials != nil {
			password = exp.Credentials.MQTTPassword
		}
		if err := h.sealBrokerPassword(prev, &s, password); err != nil {
			return fmt.Errorf("store the MQTT broker password: %w", err)
		}
		if s.NodeID == "" {
			s.NodeID = newNodeID()
		}
		if err := h.store.SaveMQTTSettings(s); err != nil {
			return err
		}
		if err := h.ha.Apply(h.haConfig(s)); err != nil {
			log.Printf("homeassistant: %v", err)
		}
	}
	if on := exp.MDNSEnabled; on != nil {
		was, err := h.store.MDNSEnabled()
		if err != nil {
			return err
		}
		if err := h.store.SetMDNSEnabled(*on); err != nil {
			return err
		}
		if was != *on {
			h.applyMDNS(*on)
		}
	}
	return nil
}
