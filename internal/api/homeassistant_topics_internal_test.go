package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func startBroker(t *testing.T) (*mochi.Server, int) {
	t.Helper()
	srv := mochi.New(&mochi.Options{InlineClient: true, Logger: slog.New(slog.DiscardHandler)})
	if err := srv.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatal(err)
	}
	tcp := listeners.NewTCP(listeners.Config{ID: "test", Address: "127.0.0.1:0"})
	if err := srv.AddListener(tcp); err != nil {
		t.Fatal(err)
	}
	if err := srv.Serve(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	_, p, _ := net.SplitHostPort(tcp.Address())
	port, _ := strconv.Atoi(p)
	return srv, port
}

// Home Assistant keys its entities on the discovery topics and unique_ids an
// instance announced before, so they stay what they were, and so do the
// device and the state and availability topics.
func TestHomeAssistantKeepsTheTopicsAndIdsItHad(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t, "photos")
	srv, port := startBroker(t)
	s := store.MQTTSettings{Enabled: true, Host: "127.0.0.1", Port: port, Prefix: "bombvault", Buttons: true, NodeID: "a1b2c3d4"}
	if err := repo.SaveMQTTSettings(s); err != nil {
		t.Fatal(err)
	}
	st, err := h.haState(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var domains []string
	for d := range st.Domains {
		domains = append(domains, d)
	}
	if len(domains) == 0 {
		t.Fatal("no domain is switched on, so the test would see no domain entities")
	}
	want := []string{
		"homeassistant/sensor/bombvault_a1b2c3d4/anomalies/config",
		"homeassistant/sensor/bombvault_a1b2c3d4/next_run/config",
		"homeassistant/sensor/bombvault_a1b2c3d4/running/config",
		"homeassistant/sensor/bombvault_a1b2c3d4/status/config",
	}
	for _, d := range domains {
		want = append(want,
			"homeassistant/sensor/bombvault_a1b2c3d4/"+d+"_last_backup/config",
			"homeassistant/sensor/bombvault_a1b2c3d4/"+d+"_last_result/config",
			"homeassistant/sensor/bombvault_a1b2c3d4/"+d+"_free_space/config",
			"homeassistant/button/bombvault_a1b2c3d4/"+d+"_backup/config",
		)
	}
	slices.Sort(want)

	if err := h.ha.Apply(h.haConfig(s)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.ha.Close)
	var got []string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		got = got[:0]
		for _, pk := range srv.Topics.Messages("homeassistant/#") {
			got = append(got, pk.TopicName)
		}
		slices.Sort(got)
		if slices.Equal(got, want) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("discovery topics\n got %v\nwant %v", got, want)
	}
	for _, pk := range srv.Topics.Messages("homeassistant/#") {
		var c struct {
			UniqueID     string `json:"unique_id"`
			CommandTopic string `json:"command_topic"`
			StateTopic   string `json:"state_topic"`
			Availability string `json:"availability_topic"`
			Device       struct {
				Identifiers  []string `json:"identifiers"`
				Manufacturer string   `json:"manufacturer"`
			} `json:"device"`
		}
		if err := json.Unmarshal(pk.Payload, &c); err != nil {
			t.Fatal(err)
		}
		object := pk.TopicName[len("homeassistant/sensor/bombvault_a1b2c3d4/") : len(pk.TopicName)-len("/config")]
		if pk.TopicName[len("homeassistant/"):len("homeassistant/button")] == "button" {
			object = pk.TopicName[len("homeassistant/button/bombvault_a1b2c3d4/") : len(pk.TopicName)-len("/config")]
			if want := "bombvault/a1b2c3d4/backup/" + strings.TrimSuffix(object, "_backup"); c.CommandTopic != want {
				t.Errorf("%s is pressed on %q, want %q", pk.TopicName, c.CommandTopic, want)
			}
		} else if c.StateTopic != "bombvault/a1b2c3d4/state" {
			t.Errorf("%s reads its state from %q", pk.TopicName, c.StateTopic)
		}
		if c.UniqueID != "bombvault_a1b2c3d4_"+object || c.Availability != "bombvault/a1b2c3d4/availability" ||
			!slices.Equal(c.Device.Identifiers, []string{"bombvault_a1b2c3d4"}) || c.Device.Manufacturer != "BombVault" {
			t.Errorf("%s announces %s", pk.TopicName, pk.Payload)
		}
	}
	if pks := srv.Topics.Messages("bombvault/a1b2c3d4/availability"); len(pks) != 1 || string(pks[0].Payload) != "online" {
		t.Fatalf("availability = %v", pks)
	}
}
