package homeassistant

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// fakeBroker stands in for a broker and the client in front of it: it keeps
// what is retained and hands subscribed messages back.
type fakeBroker struct {
	mu        sync.Mutex
	opts      *mqtt.ClientOptions
	connected bool
	retained  map[string][]byte
	handlers  map[string]mqtt.MessageHandler
	sent      []string
}

type doneToken struct{}

func (doneToken) Wait() bool                     { return true }
func (doneToken) WaitTimeout(time.Duration) bool { return true }
func (doneToken) Done() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}
func (doneToken) Error() error { return nil }

func (f *fakeBroker) client(opts *mqtt.ClientOptions) mqtt.Client {
	f.mu.Lock()
	f.opts = opts
	f.mu.Unlock()
	return f
}

func (f *fakeBroker) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}
func (f *fakeBroker) IsConnectionOpen() bool { return f.IsConnected() }
func (f *fakeBroker) Connect() mqtt.Token {
	f.mu.Lock()
	f.connected = true
	onConnect := f.opts.OnConnect
	f.mu.Unlock()
	onConnect(f)
	return doneToken{}
}
func (f *fakeBroker) Disconnect(uint) {
	f.mu.Lock()
	f.connected = false
	f.mu.Unlock()
}
func (f *fakeBroker) Publish(topic string, _ byte, retained bool, payload any) mqtt.Token {
	var b []byte
	switch p := payload.(type) {
	case string:
		b = []byte(p)
	case []byte:
		b = p
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, topic)
	if retained {
		if len(b) == 0 {
			delete(f.retained, topic)
		} else {
			f.retained[topic] = b
		}
	}
	return doneToken{}
}
func (f *fakeBroker) Subscribe(topic string, _ byte, cb mqtt.MessageHandler) mqtt.Token {
	f.mu.Lock()
	f.handlers[topic] = cb
	f.mu.Unlock()
	return doneToken{}
}
func (f *fakeBroker) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
	return doneToken{}
}
func (f *fakeBroker) Unsubscribe(...string) mqtt.Token        { return doneToken{} }
func (f *fakeBroker) AddRoute(string, mqtt.MessageHandler)    {}
func (f *fakeBroker) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

func (f *fakeBroker) get(topic string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.retained[topic]
	return string(b), ok
}

type message struct {
	topic    string
	payload  string
	retained bool
}

func (m message) Duplicate() bool   { return false }
func (m message) Qos() byte         { return 1 }
func (m message) Retained() bool    { return m.retained }
func (m message) Topic() string     { return m.topic }
func (m message) MessageID() uint16 { return 1 }
func (m message) Payload() []byte   { return []byte(m.payload) }
func (m message) Ack()              {}

func (f *fakeBroker) press(filter, topic, payload string) {
	f.mu.Lock()
	cb := f.handlers[filter]
	f.mu.Unlock()
	cb(f, message{topic: topic, payload: payload})
}

type rig struct {
	broker  *fakeBroker
	bridge  *Bridge
	mu      sync.Mutex
	state   State
	started chan string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		broker:  &fakeBroker{retained: map[string][]byte{}, handlers: map[string]mqtt.MessageHandler{}},
		started: make(chan string, 4),
		state: State{
			Status: "ok", Running: "idle",
			Domains: map[string]DomainState{
				"containers": {LastBackup: time.Unix(1_800_000_000, 0).UTC(), LastResult: "success"},
				"flash":      {},
			},
		},
	}
	r.bridge = NewBridge(Source{
		Device: func() Device { return Device{Name: "BombVault", Version: "v9.1.0"} },
		State: func(context.Context) (State, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.state, nil
		},
		Start: func(_ context.Context, domain string) string {
			r.started <- domain
			return "ok"
		},
	})
	r.bridge.newClient = r.broker.client
	t.Cleanup(r.bridge.Close)
	return r
}

var testConfig = Config{Enabled: true, Host: "broker", Port: 1883, Prefix: "bombvault", Buttons: true, Node: "a1b2c3d4"}

func TestSwitchingOnAnnouncesTheDeviceToHomeAssistant(t *testing.T) {
	r := newRig(t)
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	topics := testConfig.topics()
	if v, _ := r.broker.get(topics.Availability()); v != "online" {
		t.Fatalf("availability = %q, want online", v)
	}
	if r.broker.opts.WillTopic != topics.Availability() || string(r.broker.opts.WillPayload) != "offline" || !r.broker.opts.WillRetained {
		t.Fatalf("last will = %q %q, want offline on the availability topic, retained", r.broker.opts.WillTopic, r.broker.opts.WillPayload)
	}
	for _, topic := range []string{
		"homeassistant/sensor/bombvault_a1b2c3d4/status/config",
		"homeassistant/sensor/bombvault_a1b2c3d4/containers_last_backup/config",
		"homeassistant/button/bombvault_a1b2c3d4/containers_backup/config",
		"homeassistant/button/bombvault_a1b2c3d4/flash_backup/config",
	} {
		if _, ok := r.broker.get(topic); !ok {
			t.Errorf("%s is not retained", topic)
		}
	}
	if _, ok := r.broker.get("homeassistant/button/bombvault_a1b2c3d4/vms_backup/config"); ok {
		t.Error("a switched-off domain got a button")
	}
	raw, _ := r.broker.get(topics.State())
	var state map[string]any
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatalf("state %q: %v", raw, err)
	}
	containers, _ := state["containers"].(map[string]any)
	if state["status"] != "ok" || containers["last_backup"] != "2027-01-15T08:00:00Z" || containers["free_bytes"] != nil {
		t.Fatalf("state = %s", raw)
	}
	if state["next_run"] != nil {
		t.Fatalf("an unknown next run reads %v, want null", state["next_run"])
	}
}

func TestADomainSwitchedOffLosesItsEntities(t *testing.T) {
	r := newRig(t)
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	delete(r.state.Domains, "flash")
	r.mu.Unlock()
	r.bridge.Refresh()
	for _, topic := range []string{
		"homeassistant/sensor/bombvault_a1b2c3d4/flash_last_result/config",
		"homeassistant/button/bombvault_a1b2c3d4/flash_backup/config",
	} {
		if _, ok := r.broker.get(topic); ok {
			t.Errorf("%s is still retained", topic)
		}
	}
	if _, ok := r.broker.get("homeassistant/button/bombvault_a1b2c3d4/containers_backup/config"); !ok {
		t.Error("the domain still on lost its button")
	}
}

func TestAButtonPressStartsItsDomain(t *testing.T) {
	r := newRig(t)
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	topics := testConfig.topics()
	r.broker.press(topics.CommandFilter(), topics.Command("printers"), "PRESS")
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "HOLD")
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	select {
	case d := <-r.started:
		if d != "containers" {
			t.Fatalf("started %q, want containers", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the press started nothing")
	}
	select {
	case d := <-r.started:
		t.Fatalf("a second start of %q from an unknown domain or payload", d)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestButtonsSwitchedOffDoNothing(t *testing.T) {
	r := newRig(t)
	cfg := testConfig
	cfg.Buttons = false
	if err := r.bridge.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.broker.get("homeassistant/button/bombvault_a1b2c3d4/containers_backup/config"); ok {
		t.Fatal("a button was announced with buttons switched off")
	}
	topics := cfg.topics()
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	select {
	case d := <-r.started:
		t.Fatalf("a press started %s with buttons switched off", d)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSwitchingOffRemovesTheDevice(t *testing.T) {
	r := newRig(t)
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	off := testConfig
	off.Enabled = false
	if err := r.bridge.Apply(off); err != nil {
		t.Fatal(err)
	}
	r.broker.mu.Lock()
	left := len(r.broker.retained)
	r.broker.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d retained messages left, want none: %v", left, r.broker.retained)
	}
	if r.broker.IsConnected() {
		t.Fatal("still connected after switching off")
	}
}

func TestShuttingDownLeavesTheDeviceUnavailable(t *testing.T) {
	r := newRig(t)
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	r.bridge.Close()
	if v, _ := r.broker.get(testConfig.topics().Availability()); v != "offline" {
		t.Fatalf("availability after shutdown = %q, want offline", v)
	}
	if _, ok := r.broker.get("homeassistant/sensor/bombvault_a1b2c3d4/status/config"); !ok {
		t.Fatal("shutting down removed the entities")
	}
}

func TestDiscoveryTiesEveryEntityToOneDevice(t *testing.T) {
	topics := testConfig.topics()
	for topic, payload := range topics.Discovery(Device{Name: "BombVault (tower)", Version: "v9.1.0"}, []string{"vms"}, true) {
		var cfg map[string]any
		if err := json.Unmarshal(payload, &cfg); err != nil {
			t.Fatalf("%s: %v", topic, err)
		}
		device, _ := cfg["device"].(map[string]any)
		ids, _ := device["identifiers"].([]any)
		if len(ids) != 1 || ids[0] != "bombvault_a1b2c3d4" || device["name"] != "BombVault (tower)" {
			t.Errorf("%s: device %v", topic, device)
		}
		if id, _ := cfg["unique_id"].(string); !strings.HasPrefix(id, "bombvault_a1b2c3d4_") {
			t.Errorf("%s: unique_id %v", topic, cfg["unique_id"])
		}
		if cfg["availability_topic"] != "bombvault/a1b2c3d4/availability" {
			t.Errorf("%s: availability_topic %v", topic, cfg["availability_topic"])
		}
	}
	if n := len(topics.AllDiscoveryTopics()); n != 4+len(Domains)*4 {
		t.Fatalf("%d possible topics, want %d", n, 4+len(Domains)*4)
	}
}

func (f *fakeBroker) deliver(filter string, m message) {
	f.mu.Lock()
	cb := f.handlers[filter]
	f.mu.Unlock()
	cb(f, m)
}

func expectNoStart(t *testing.T, r *rig, why string) {
	t.Helper()
	select {
	case d := <-r.started:
		t.Fatalf("%s started %s", why, d)
	case <-time.After(100 * time.Millisecond):
	}
}

func expectStart(t *testing.T, r *rig, domain string) {
	t.Helper()
	select {
	case d := <-r.started:
		if d != domain {
			t.Fatalf("started %q, want %q", d, domain)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("the press of %s started nothing", domain)
	}
}

// A retained press is replayed by the broker to every new subscriber, so it
// would start a backup on each reconnect.
func TestARetainedPressStartsNothing(t *testing.T) {
	r := newRig(t)
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	topics := testConfig.topics()
	r.broker.deliver(topics.CommandFilter(), message{topic: topics.Command("containers"), payload: "PRESS", retained: true})
	expectNoStart(t, r, "a retained press")
}

func TestAPressWhileOneIsPendingForItsDomainIsIgnored(t *testing.T) {
	r := newRig(t)
	release := make(chan struct{})
	start := r.bridge.src.Start
	r.bridge.src.Start = func(ctx context.Context, domain string) string {
		out := start(ctx, domain)
		<-release
		return out
	}
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	topics := testConfig.topics()
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	expectStart(t, r, "containers")
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	expectNoStart(t, r, "a second press while the first was pending")
	r.broker.press(topics.CommandFilter(), topics.Command("flash"), "PRESS")
	expectStart(t, r, "flash")
	close(release)
	time.Sleep(50 * time.Millisecond)
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	expectStart(t, r, "containers")
}

func TestPressesAreLimitedPerMinute(t *testing.T) {
	r := newRig(t)
	clock := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	r.bridge.now = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return clock
	}
	if err := r.bridge.Apply(testConfig); err != nil {
		t.Fatal(err)
	}
	topics := testConfig.topics()
	for range pressesPerMinute {
		r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
		expectStart(t, r, "containers")
		time.Sleep(20 * time.Millisecond)
	}
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	expectNoStart(t, r, "a press over the limit")
	clockMu.Lock()
	clock = clock.Add(time.Minute + time.Second)
	clockMu.Unlock()
	r.broker.press(topics.CommandFilter(), topics.Command("containers"), "PRESS")
	expectStart(t, r, "containers")
}
