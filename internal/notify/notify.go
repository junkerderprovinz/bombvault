// Package notify sends BombVault's events through the channels the user set
// up. Delivery, retries and the services themselves are the parleyport kit's;
// this package decides what BombVault tells and when: the policy of each
// channel, the summary of a scheduled run, and the Healthchecks phases. Every
// send is best-effort and time-bounded, so a failed notification never affects
// a backup.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/notify/legacy"

	"github.com/junkerderprovinz/bombvault/internal/redact"
)

// source names BombVault where a service shows a sender.
const source = "BombVault"

// hcSuppressKey marks a context in which Send and SendStart skip the
// Healthchecks ping. A scheduled per-domain run sets it on every item and pings
// once for the whole run instead (PingDomainStart, PingDomainResult).
type hcSuppressKey struct{}

// WithHealthchecksSuppressed returns a context in which Send and SendStart skip
// the Healthchecks ping. The other channels still fire.
func WithHealthchecksSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, hcSuppressKey{}, true)
}

func healthchecksSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(hcSuppressKey{}).(bool)
	return v
}

// msgSuppressKey marks the items of a scheduled run. When
// Config.ScheduledSummary is on, Send drops their result messages, so the run
// ends in one "N of M" summary. It is separate from hcSuppressKey because a
// container update notice skips only the Healthchecks ping and must still
// deliver its message.
type msgSuppressKey struct{}

// WithMessagesSuppressed marks ctx as an item of a scheduled run, whose
// Event.ItemResult messages Send drops when ScheduledSummary is on. Alerts
// sent on the same ctx still go out, since the summary does not carry them.
func WithMessagesSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, msgSuppressKey{}, true)
}

// MessagesSuppressed reports whether ctx is an item of a scheduled run.
func MessagesSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(msgSuppressKey{}).(bool)
	return v
}

// Config is the stored notification settings: where messages go, and the two
// switches that decide what BombVault sends beyond its alerts.
type Config struct {
	Channels []kit.Channel `json:"channels"`
	// ScheduledSummary replaces the per-item messages of a scheduled run with
	// one "N of M" summary. Healthchecks is aggregated either way, and manual
	// backups always report per item.
	ScheduledSummary bool `json:"scheduledSummary"`
	// NotifyOnUpdate sends "Updated <name> to a newer image" when the
	// post-backup image update replaces a container. It goes out per
	// container, never folded into the summary.
	NotifyOnUpdate bool `json:"notifyOnUpdate"`

	unraid kit.Runner
}

// channelForm is Config without its methods, so decoding into it neither
// recurses into UnmarshalJSON nor hides the fields from DisallowUnknownFields.
type channelForm struct {
	Channels         []kit.Channel `json:"channels"`
	ScheduledSummary bool          `json:"scheduledSummary"`
	NotifyOnUpdate   bool          `json:"notifyOnUpdate"`
}

// Legacy reports whether data is a config stored before channels: one flat
// object with a key per setting, such as webhookUrl and smtpHost.
func Legacy(data []byte) bool {
	var keys map[string]json.RawMessage
	if json.Unmarshal(data, &keys) != nil {
		return false
	}
	_, channels := keys["channels"]
	return !channels && len(keys) > 0
}

// UnmarshalJSON reads a config in either form. The stored blob is encrypted
// with APP_KEY and settings exports carry it in cleartext, so a config from
// before channels can turn up long after the start that converted the stored
// one.
func (c *Config) UnmarshalJSON(data []byte) error {
	if Legacy(data) {
		channels, err := legacy.FromFlat(data)
		if err != nil {
			return err
		}
		var switches channelForm
		if err := json.Unmarshal(data, &switches); err != nil {
			return err
		}
		*c = Config{Channels: channels, ScheduledSummary: switches.ScheduledSummary, NotifyOnUpdate: switches.NotifyOnUpdate}
		return nil
	}
	var f channelForm
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*c = Config{Channels: f.Channels, ScheduledSummary: f.ScheduledSummary, NotifyOnUpdate: f.NotifyOnUpdate}
	return nil
}

// DecodeStrict reads a config in the channel form and rejects unknown keys, so a
// misspelled field is refused instead of dropped.
func (c *Config) DecodeStrict(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f channelForm
	if err := dec.Decode(&f); err != nil {
		return err
	}
	*c = Config{Channels: f.Channels, ScheduledSummary: f.ScheduledSummary, NotifyOnUpdate: f.NotifyOnUpdate}
	return nil
}

// WithUnraid returns c with the runner that reaches the Unraid host. Without
// one, Unraid channels stay silent.
func (c Config) WithUnraid(r kit.Runner) Config {
	c.unraid = r
	return c
}

// WantsUnraid reports whether an Unraid channel is switched on.
func (c Config) WantsUnraid() bool {
	for _, ch := range c.Channels {
		if ch.Enabled && ch.Kind == kit.KindUnraid {
			return true
		}
	}
	return false
}

// Active reports whether any channel hears anything: one that is on, can be
// reached and does not say never.
func (c Config) Active() bool {
	for _, ch := range c.usable() {
		if ch.On != kit.OnNever {
			return true
		}
	}
	return false
}

// TakesMessages reports whether a channel carries messages, which a
// Healthchecks check does not.
func (c Config) TakesMessages() bool {
	return len(c.messageChannels()) > 0
}

// Carries reports whether a channel's policy takes ev, so a message that went
// nowhere can tell a channel that failed from one that never wanted it.
func (c Config) Carries(ev Event) bool {
	for _, ch := range c.messageChannels() {
		if ch.On == kit.OnAlways || ev.severity() != kit.SeveritySuccess {
			return true
		}
	}
	return false
}

// IsDefault reports whether c says nothing a never saved config would not.
func (c Config) IsDefault() bool {
	return len(c.Channels) == 0 && !c.ScheduledSummary && !c.NotifyOnUpdate
}

// Validate reports every problem of the channels as kit.Problems, or nil.
func (c Config) Validate() error {
	return kit.Validate(c.Channels)
}

func (c Config) messageChannels() []kit.Channel {
	var out []kit.Channel
	for _, ch := range c.usable() {
		if ch.Kind != kit.KindHealthchecks && ch.On != kit.OnNever {
			out = append(out, ch)
		}
	}
	return out
}

// usable are the channels that are on and can be sent to. One that fails
// validation is left out, so it cannot keep the others from sending.
func (c Config) usable() []kit.Channel {
	var out []kit.Channel
	for _, ch := range c.Channels {
		if !ch.Enabled || (ch.Kind == kit.KindUnraid && c.unraid == nil) {
			continue
		}
		if err := kit.Validate([]kit.Channel{ch}); err != nil {
			log.Printf("notify: channel %s is left out: %v", ch.ID, err)
			continue
		}
		out = append(out, ch)
	}
	return out
}

func (c Config) dispatcher(channels []kit.Channel) *kit.Dispatcher {
	d, err := kit.NewDispatcher(channels, kit.Options{Source: source, Unraid: c.unraid})
	if err != nil {
		log.Printf("notify: %v", err)
		return nil
	}
	return d
}

// Event is something BombVault tells. Title is the subject line, which
// Unraid and mail show on their own.
type Event struct {
	Title   string
	Message string
	OK      bool
	// ItemResult marks the outcome of one item of a run, the message a
	// scheduled summary replaces.
	ItemResult bool
	// Warning marks a success that left something out, or a backup that waits
	// for its app, which reaches the channels of somebody who hears only about
	// failures. A health check still counts it as a success.
	Warning bool
	// Critical marks a failure that needs a hand at once, such as a VM left
	// frozen. Unraid shows it as an alert.
	Critical bool
	// CheckHeld names a failure a success must not hide on the health check it
	// shares with other backups. The check then hears this instead of a success.
	CheckHeld string
}

func (ev Event) severity() kit.Severity {
	switch {
	case ev.Critical:
		return kit.SeverityCritical
	case !ev.OK:
		return kit.SeverityFailure
	case ev.Warning:
		return kit.SeverityWarning
	}
	return kit.SeveritySuccess
}

// kind is the event's name for a channel's event filter and a webhook's
// %%event%%, such as "container.failed".
func (ev Event) kind(key string) string {
	outcome := "done"
	switch ev.severity() {
	case kit.SeverityCritical, kit.SeverityFailure:
		outcome = "failed"
	case kit.SeverityWarning:
		outcome = "warning"
	}
	if key == "" {
		return outcome
	}
	return strings.ToLower(key) + "." + outcome
}

// Send delivers ev to the channels whose policy takes it and logs each
// channel's failure. It reports whether at least one channel took the message,
// so an alert sent once per episode can be tried again when none did.
// Healthchecks is pinged on both outcomes, since a check needs its success
// pings to stay green.
func Send(ctx context.Context, c Config, domain string, ev Event) bool {
	key := checkKey(domain)
	delivered := false
	// In summary mode Service.ScheduledNotifyResult reports the item instead.
	folded := ev.ItemResult && MessagesSuppressed(ctx) && c.ScheduledSummary
	if channels := c.messageChannels(); !folded && len(channels) > 0 {
		if d := c.dispatcher(channels); d != nil {
			delivered = logged(d.Send(ctx, kit.Event{
				Kind:     ev.kind(key),
				Severity: ev.severity(),
				Title:    redact.Credentials(ev.Title),
				Body:     redact.Credentials(ev.Message),
				Key:      key,
			}))
		}
	}
	if !healthchecksSuppressed(ctx) {
		ping, body := kit.PingSuccess, ""
		switch {
		case !ev.OK:
			ping = kit.PingFail
		case ev.CheckHeld != "":
			ping, body = kit.PingFail, redact.Credentials(ev.CheckHeld)
		}
		c.ping(ctx, kit.Event{Key: key, Ping: ping, Body: body})
	}
	return delivered
}

// SendStart marks the start of a backup. It pings the Healthchecks /start
// endpoint, so the check can measure duration and catch a hung run, and posts
// an "info" message to the Apprise channels that hear everything, the only
// service with a type for it. The Apprise post is skipped for per-item sends
// in summary mode.
func SendStart(ctx context.Context, c Config, domain string) {
	if !healthchecksSuppressed(ctx) {
		c.ping(ctx, kit.Event{Key: checkKey(domain), Ping: kit.PingStart})
	}
	if MessagesSuppressed(ctx) && c.ScheduledSummary {
		return
	}
	var apprise []kit.Channel
	for _, ch := range c.usable() {
		if ch.Kind == kit.KindApprise && ch.On == kit.OnAlways {
			apprise = append(apprise, ch)
		}
	}
	if len(apprise) == 0 {
		return
	}
	body := "Backup started"
	if domain != "" {
		body = "Backup started (" + domain + ")"
	}
	if d := c.dispatcher(apprise); d != nil {
		logged(d.Send(ctx, kit.Event{Kind: "backup.started", Severity: kit.SeverityInfo, Title: source, Body: body}))
	}
}

// PingDomainStart pings /start once for a whole scheduled per-domain run, in
// place of the per-item SendStart pings the run suppresses.
func PingDomainStart(ctx context.Context, c Config, domain string) {
	c.ping(ctx, kit.Event{Key: checkKey(domain), Ping: kit.PingStart})
}

// Ping ends the backup a SendStart began on the domain's check without a
// message, for a backup that ended in a way nobody is told about each time.
func Ping(ctx context.Context, c Config, domain string, ok bool) {
	if healthchecksSuppressed(ctx) {
		return
	}
	PingDomainResult(ctx, c, domain, ok, "")
}

// PingDomainResult pings the domain's check once at the end of a scheduled
// run. summary, such as "1 of 3 items failed", lands in the check's event log.
func PingDomainResult(ctx context.Context, c Config, domain string, ok bool, summary string) {
	ping := kit.PingSuccess
	if !ok {
		ping = kit.PingFail
	}
	c.ping(ctx, kit.Event{Key: checkKey(domain), Ping: ping, Body: redact.Credentials(summary)})
}

func (c Config) ping(ctx context.Context, e kit.Event) {
	var checks []kit.Channel
	for _, ch := range c.usable() {
		if ch.Kind == kit.KindHealthchecks {
			checks = append(checks, ch)
		}
	}
	if len(checks) == 0 {
		return
	}
	if d := c.dispatcher(checks); d != nil {
		logged(d.Ping(ctx, e))
	}
}

// logged logs each failed result and reports whether any channel took the
// event.
func logged(results []kit.Result) bool {
	took := false
	for _, r := range results {
		if r.OK() {
			took = true
			continue
		}
		log.Printf("notify: %s (%s): %v", r.Channel, r.Kind, r.Err)
	}
	return took
}

// Checks lists the Healthchecks checks a domain pings, sorted. Domains that
// list the same checks share them, so a success of one can turn green what
// another left red.
func (c Config) Checks(domain string) []string {
	key := checkKey(domain)
	var out []string
	for _, ch := range c.usable() {
		if ch.Kind != kit.KindHealthchecks || ch.On == kit.OnNever {
			continue
		}
		u := ch.Healthchecks.ByKey[key]
		if u == "" {
			u = ch.Healthchecks.URL
		}
		if u != "" && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	slices.Sort(out)
	return out
}

// checkKey maps the domain spellings used across the codebase to the keys of
// a Healthchecks channel's own checks. Backups say "container" and "VM", the
// off-site and tamper notifiers "containers" and "vms"; without this a
// section's check would miss the replication, drill and tamper failures.
func checkKey(domain string) string {
	switch domain {
	case "containers", "stack":
		return "container"
	case "vms", "VMs", "vm":
		return "VM"
	default:
		return domain
	}
}

// TestEvent is what Test sends.
var TestEvent = kit.Event{Kind: "test", Severity: kit.SeverityInfo, Title: source, Body: "Test notification: notifications are working."}

// Test sends TestEvent through ch once, saved or not, and reports how it went.
func Test(ctx context.Context, ch kit.Channel, unraid kit.Runner) kit.Result {
	d, err := kit.NewDispatcher(nil, kit.Options{Source: source, Unraid: unraid})
	if err != nil {
		return kit.Result{Channel: ch.ID, Kind: ch.Kind, Code: kit.CodeInvalid, Err: fmt.Errorf("notify: %w", err)}
	}
	return d.Test(ctx, ch, TestEvent)
}
