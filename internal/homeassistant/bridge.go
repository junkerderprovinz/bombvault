package homeassistant

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config is the MQTT connection and what to publish over it.
type Config struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool
	Prefix   string
	Buttons  bool
	Node     string
}

func (c Config) topics() Topics { return Topics{Prefix: c.Prefix, Node: c.Node} }

// DomainState is what the sensors of one domain show.
type DomainState struct {
	LastBackup time.Time
	LastResult string
	FreeBytes  *int64
}

// State is one reading of everything the sensors show. Domains holds the
// switched-on domains only, and they are the ones that get entities.
type State struct {
	Status    string
	Running   string
	Anomalies int
	NextRun   time.Time
	Domains   map[string]DomainState
}

func (s State) domains() []string {
	var out []string
	for _, d := range Domains {
		if _, ok := s.Domains[d]; ok {
			out = append(out, d)
		}
	}
	return out
}

// payload is the retained JSON every sensor's template reads from. A time or a
// size nobody knows is null, which Home Assistant shows as unknown.
func (s State) payload() []byte {
	stamp := func(t time.Time) any {
		if t.IsZero() {
			return nil
		}
		return t.Format(time.RFC3339)
	}
	out := map[string]any{
		"status":    s.Status,
		"running":   s.Running,
		"anomalies": s.Anomalies,
		"next_run":  stamp(s.NextRun),
	}
	for d, ds := range s.Domains {
		var result any
		if ds.LastResult != "" {
			result = ds.LastResult
		}
		out[d] = map[string]any{"last_backup": stamp(ds.LastBackup), "last_result": result, "free_bytes": ds.FreeBytes}
	}
	b, _ := json.Marshal(out)
	return b
}

// Status is how the connection stands, for the settings card.
type Status struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error"`
}

// Source is what the bridge reads and does on BombVault's side.
type Source struct {
	Device func() Device
	State  func(ctx context.Context) (State, error)
	// Start backs up a domain for a button press and returns what came of it,
	// for the log.
	Start func(ctx context.Context, domain string) string
}

// pollEvery is how often the state is read again. A reading that has not
// changed is not sent.
const pollEvery = 15 * time.Second

// waitFor bounds every wait on the broker, so a dead one cannot hold up a
// settings save or a shutdown.
const waitFor = 5 * time.Second

// pressesPerMinute caps the button presses the bridge acts on, whoever sends
// them. The per-item limits of a start still apply on top.
const pressesPerMinute = 6

// Bridge keeps one MQTT connection per configuration and publishes to it.
type Bridge struct {
	src Source
	// newClient is mqtt.NewClient; tests hand in a fake broker.
	newClient func(*mqtt.ClientOptions) mqtt.Client

	// refreshMu keeps the poll and a reconnect from publishing at once.
	refreshMu sync.Mutex

	mu        sync.Mutex
	cfg       Config
	client    mqtt.Client
	status    Status
	published map[string][]byte
	lastState []byte
	stopLoop  context.CancelFunc
	loopDone  chan struct{}

	// now is time.Now; tests move it.
	now func() time.Time
	// pending holds the domains whose press is still being started, and
	// pressed when the presses of the last minute came in.
	pressMu sync.Mutex
	pending map[string]bool
	pressed []time.Time
}

// NewBridge returns a bridge that is not connected to anything yet.
func NewBridge(src Source) *Bridge {
	return &Bridge{src: src, newClient: mqtt.NewClient, now: time.Now, pending: map[string]bool{}}
}

// Status reports the connection for the settings card.
func (b *Bridge) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}

// Apply switches the bridge to cfg. When the broker, the topics or the
// switch changed, the old entities are removed from Home Assistant first, so
// no device is left behind under an old name.
func (b *Bridge) Apply(cfg Config) error {
	b.mu.Lock()
	old, client := b.cfg, b.client
	b.mu.Unlock()

	var cleanupErr error
	if client != nil {
		moved := !cfg.Enabled || old.Host != cfg.Host || old.Port != cfg.Port || old.TLS != cfg.TLS ||
			old.Username != cfg.Username || old.Password != cfg.Password ||
			old.Prefix != cfg.Prefix || old.Node != cfg.Node
		if !moved {
			b.mu.Lock()
			b.cfg = cfg
			b.mu.Unlock()
			b.Refresh()
			return nil
		}
		b.stop()
		cleanupErr = remove(client, old.topics())
		client.Disconnect(250)
	}

	b.mu.Lock()
	b.cfg, b.client, b.published, b.lastState = cfg, nil, nil, nil
	b.status = Status{}
	b.mu.Unlock()
	if !cfg.Enabled {
		return cleanupErr
	}
	b.connect(cfg)
	return cleanupErr
}

// Close marks the device unavailable and disconnects, leaving its entities in
// Home Assistant for the next start.
func (b *Bridge) Close() {
	b.stop()
	b.mu.Lock()
	client, cfg := b.client, b.cfg
	b.client = nil
	b.mu.Unlock()
	if client == nil {
		return
	}
	if client.IsConnected() {
		client.Publish(cfg.topics().Availability(), 1, true, "offline").WaitTimeout(waitFor)
	}
	client.Disconnect(250)
}

func (b *Bridge) connect(cfg Config) {
	t := cfg.topics()
	scheme := "tcp://"
	if cfg.TLS {
		scheme = "tls://"
	}
	opts := mqtt.NewClientOptions().
		AddBroker(scheme+net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))).
		SetClientID("bombvault-"+cfg.Node).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetCleanSession(true).
		SetKeepAlive(30*time.Second).
		SetConnectTimeout(10*time.Second).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(15*time.Second).
		SetMaxReconnectInterval(time.Minute).
		SetOrderMatters(false).
		SetBinaryWill(t.Availability(), []byte("offline"), 1, true)
	if cfg.TLS {
		opts.SetTLSConfig(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
	}
	opts.SetOnConnectHandler(func(c mqtt.Client) { b.onConnect(c, cfg) })
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		log.Printf("homeassistant: lost the connection to %s: %v", cfg.Host, err)
		b.setStatus(Status{Error: err.Error()})
	})

	client := b.newClient(opts)
	b.mu.Lock()
	b.client = client
	b.mu.Unlock()

	token := client.Connect()
	go func() {
		// With connect retry on, the token only completes once a connection
		// stands, so one still open after the wait means the broker does not
		// answer.
		if token.WaitTimeout(waitFor) && token.Error() != nil {
			b.setStatus(Status{Error: token.Error().Error()})
		} else if !client.IsConnected() {
			b.setStatus(Status{Error: "the broker at " + cfg.Host + " does not answer"})
		}
	}()
	b.startLoop()
}

func (b *Bridge) onConnect(c mqtt.Client, cfg Config) {
	log.Printf("homeassistant: connected to %s", cfg.Host)
	b.setStatus(Status{Connected: true})
	t := cfg.topics()
	c.Subscribe(t.CommandFilter(), 1, func(_ mqtt.Client, m mqtt.Message) {
		domain := t.DomainOfCommand(m.Topic())
		if domain == "" || string(m.Payload()) != "PRESS" {
			return
		}
		// The broker hands a retained press to every new subscriber, so it would
		// start a backup on each reconnect.
		if m.Retained() {
			log.Printf("homeassistant: ignored a retained press of the %s button", domain)
			return
		}
		go b.press(domain)
	})
	b.mu.Lock()
	// A reconnect republishes everything: a broker without persistence has
	// forgotten the retained messages.
	b.published, b.lastState = nil, nil
	b.mu.Unlock()
	c.Publish(t.Availability(), 1, true, "online")
	b.Refresh()
}

// press starts a domain for its button, unless the buttons are off, a press of
// the same domain is still being started, or the minute's presses are used up.
func (b *Bridge) press(domain string) {
	b.mu.Lock()
	buttons := b.cfg.Buttons
	b.mu.Unlock()
	if !buttons {
		log.Printf("homeassistant: ignored the %s button, buttons are switched off", domain)
		return
	}
	if why := b.takePress(domain); why != "" {
		log.Printf("homeassistant: ignored the %s button, %s", domain, why)
		return
	}
	defer func() {
		b.pressMu.Lock()
		delete(b.pending, domain)
		b.pressMu.Unlock()
	}()
	outcome := b.src.Start(context.Background(), domain)
	log.Printf("homeassistant: %s button -> %s", domain, outcome)
}

// takePress books a press, or says why it is refused.
func (b *Bridge) takePress(domain string) string {
	b.pressMu.Lock()
	defer b.pressMu.Unlock()
	if b.pending[domain] {
		return "the last press is still being started"
	}
	now := b.now()
	b.pressed = slices.DeleteFunc(b.pressed, func(at time.Time) bool { return now.Sub(at) >= time.Minute })
	if len(b.pressed) >= pressesPerMinute {
		return "too many presses in the last minute"
	}
	b.pressed = append(b.pressed, now)
	b.pending[domain] = true
	return ""
}

// Refresh reads the state and publishes what changed: the discovery configs
// of entities that came or went or look different, then the state.
func (b *Bridge) Refresh() {
	b.refreshMu.Lock()
	defer b.refreshMu.Unlock()
	b.mu.Lock()
	client, cfg, published, last := b.client, b.cfg, b.published, b.lastState
	b.mu.Unlock()
	if client == nil || !client.IsConnected() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := b.src.State(ctx)
	if err != nil {
		log.Printf("homeassistant: could not read the state: %v", err)
		return
	}
	t := cfg.topics()
	want := t.Discovery(b.src.Device(), st.domains(), cfg.Buttons)
	for topic := range published {
		if _, ok := want[topic]; !ok {
			client.Publish(topic, 1, true, []byte{})
		}
	}
	for topic, payload := range want {
		if !bytes.Equal(published[topic], payload) {
			client.Publish(topic, 1, true, payload)
		}
	}
	payload := st.payload()
	if !bytes.Equal(payload, last) {
		client.Publish(t.State(), 1, true, payload)
	}
	b.mu.Lock()
	b.published, b.lastState = want, payload
	b.mu.Unlock()
}

func (b *Bridge) startLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	b.mu.Lock()
	b.stopLoop, b.loopDone = cancel, done
	b.mu.Unlock()
	go func() {
		defer close(done)
		tick := time.NewTicker(pollEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				b.Refresh()
			}
		}
	}()
}

func (b *Bridge) stop() {
	b.mu.Lock()
	cancel, done := b.stopLoop, b.loopDone
	b.stopLoop, b.loopDone = nil, nil
	b.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (b *Bridge) setStatus(s Status) {
	b.mu.Lock()
	b.status = s
	b.mu.Unlock()
}

var errNotConnected = errors.New("not connected to the broker, so Home Assistant still lists the device; remove it there or switch this on and off again once the broker answers")

// remove clears every retained message the instance published, which makes
// Home Assistant drop the device and its entities.
func remove(client mqtt.Client, t Topics) error {
	if !client.IsConnected() {
		return errNotConnected
	}
	var tokens []mqtt.Token
	for _, topic := range append(t.AllDiscoveryTopics(), t.State(), t.Availability()) {
		tokens = append(tokens, client.Publish(topic, 1, true, []byte{}))
	}
	deadline := time.Now().Add(waitFor)
	for _, token := range tokens {
		if !token.WaitTimeout(time.Until(deadline)) {
			return errNotConnected
		}
		if err := token.Error(); err != nil {
			return err
		}
	}
	return nil
}
