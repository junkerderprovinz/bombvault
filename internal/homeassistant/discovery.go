// Package homeassistant publishes BombVault to Home Assistant over MQTT: one
// device with sensors for the backup state and a button per domain, announced
// through Home Assistant's MQTT discovery.
package homeassistant

import (
	"encoding/json"
	"strings"
)

// DiscoveryPrefix is the topic root Home Assistant listens on for discovery,
// its default and what almost every installation keeps.
const DiscoveryPrefix = "homeassistant"

// Domains are the backup domains in the order Home Assistant lists them.
var Domains = []string{"containers", "vms", "files", "zfs", "flash", "config"}

var domainTitles = map[string]string{
	"containers": "Containers",
	"vms":        "VMs",
	"files":      "Files",
	"zfs":        "ZFS",
	"flash":      "Flash",
	"config":     "Config",
}

// Device is what Home Assistant shows as the device the entities belong to.
type Device struct {
	Name    string
	Version string
}

// Topics are the MQTT topics of one BombVault instance.
type Topics struct {
	Prefix string
	Node   string
}

func (t Topics) base() string         { return t.Prefix + "/" + t.Node }
func (t Topics) Availability() string { return t.base() + "/availability" }
func (t Topics) State() string        { return t.base() + "/state" }

// Command is the topic a domain's button publishes to.
func (t Topics) Command(domain string) string { return t.base() + "/backup/" + domain }

// CommandFilter subscribes to every domain's button at once.
func (t Topics) CommandFilter() string { return t.base() + "/backup/+" }

// DomainOfCommand returns the domain a command topic names, or "" for any
// other topic.
func (t Topics) DomainOfCommand(topic string) string {
	domain, ok := strings.CutPrefix(topic, t.base()+"/backup/")
	if !ok || domainTitles[domain] == "" {
		return ""
	}
	return domain
}

// entity is one discovery config: the component it is and its payload.
type entity struct {
	component string
	object    string
	config    map[string]any
}

func (t Topics) discoveryTopic(e entity) string {
	return DiscoveryPrefix + "/" + e.component + "/bombvault_" + t.Node + "/" + e.object + "/config"
}

// entities lists every entity of an instance with these domains switched on,
// with buttons when buttons is set.
func (t Topics) entities(dev Device, domains []string, buttons bool) []entity {
	device := map[string]any{
		"identifiers":  []string{"bombvault_" + t.Node},
		"name":         dev.Name,
		"manufacturer": "BombVault",
		"model":        "BombVault",
		"sw_version":   dev.Version,
	}
	sensor := func(object, name, template string, extra map[string]any) entity {
		cfg := map[string]any{
			"name":               name,
			"unique_id":          "bombvault_" + t.Node + "_" + object,
			"state_topic":        t.State(),
			"value_template":     template,
			"availability_topic": t.Availability(),
			"device":             device,
		}
		for k, v := range extra {
			cfg[k] = v
		}
		return entity{component: "sensor", object: object, config: cfg}
	}

	out := []entity{
		sensor("status", "Status", "{{ value_json.status }}", map[string]any{
			"device_class": "enum",
			"options":      []string{"ok", "warning", "failed", "off"},
			"icon":         "mdi:shield-check",
		}),
		sensor("running", "Running job", "{{ value_json.running }}", map[string]any{"icon": "mdi:progress-clock"}),
		sensor("anomalies", "Open anomalies", "{{ value_json.anomalies }}", map[string]any{
			"state_class": "measurement",
			"icon":        "mdi:alert-circle-outline",
		}),
		sensor("next_run", "Next scheduled backup", "{{ value_json.next_run }}", map[string]any{"device_class": "timestamp"}),
	}
	for _, d := range domains {
		title := domainTitles[d]
		out = append(out,
			sensor(d+"_last_backup", title+" last backup", "{{ value_json."+d+".last_backup }}", map[string]any{"device_class": "timestamp"}),
			sensor(d+"_last_result", title+" last result", "{{ value_json."+d+".last_result }}", map[string]any{"icon": "mdi:check-circle-outline"}),
			// MQTT sensors ignore suggested_unit_of_measurement, so the
			// template converts to GB itself.
			sensor(d+"_free_space", title+" repository free space",
				"{{ none if value_json."+d+".free_bytes is none else (value_json."+d+".free_bytes / 1e9) | round(1) }}",
				map[string]any{
					"device_class":                "data_size",
					"unit_of_measurement":         "GB",
					"suggested_display_precision": 1,
					"state_class":                 "measurement",
				}),
		)
		if buttons {
			out = append(out, entity{component: "button", object: d + "_backup", config: map[string]any{
				"name":               "Back up " + strings.ToLower(title),
				"unique_id":          "bombvault_" + t.Node + "_" + d + "_backup",
				"command_topic":      t.Command(d),
				"payload_press":      "PRESS",
				"availability_topic": t.Availability(),
				"device":             device,
				"icon":               "mdi:backup-restore",
			}})
		}
	}
	return out
}

// Discovery returns every discovery topic of the instance with its retained
// payload.
func (t Topics) Discovery(dev Device, domains []string, buttons bool) map[string][]byte {
	out := map[string][]byte{}
	for _, e := range t.entities(dev, domains, buttons) {
		payload, _ := json.Marshal(e.config)
		out[t.discoveryTopic(e)] = payload
	}
	return out
}

// AllDiscoveryTopics is every discovery topic the instance could have
// published, which is what removing it from Home Assistant has to clear.
func (t Topics) AllDiscoveryTopics() []string {
	var out []string
	for _, e := range t.entities(Device{}, Domains, true) {
		out = append(out, t.discoveryTopic(e))
	}
	return out
}
