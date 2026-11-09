package notify_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// hit is one request a fake service received.
type hit struct {
	method, path, body string
}

type service struct {
	*httptest.Server
	mu   sync.Mutex
	hits []hit
}

func newService(t *testing.T) *service {
	t.Helper()
	s := &service{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.hits = append(s.hits, hit{r.Method, r.URL.Path, string(b)})
		s.mu.Unlock()
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *service) got() []hit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.hits)
}

func webhook(id, url string, on kit.On) kit.Channel {
	return kit.Channel{ID: id, Kind: kit.KindWebhook, Enabled: true, On: on,
		Webhook: &kit.Webhook{URL: url, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"title":"%%title%%","ok":%%ok%%}`}}
}

func checks(url string, byKey map[string]string) kit.Channel {
	return kit.Channel{ID: "checks", Kind: kit.KindHealthchecks, Enabled: true, On: kit.OnAlways,
		Healthchecks: &kit.Healthchecks{URL: url, ByKey: byKey}}
}

func channelByID(t *testing.T, cs []kit.Channel, id string) kit.Channel {
	t.Helper()
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no channel %q among %d", id, len(cs))
	return kit.Channel{}
}

func TestAStoredConfigFromBeforeChannelsKeepsEveryChannel(t *testing.T) {
	data, err := os.ReadFile("testdata/stored-before-channels.json")
	if err != nil {
		t.Fatal(err)
	}
	var c notify.Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if !notify.Legacy(data) {
		t.Error("the stored config is not recognised as the old form")
	}
	if !c.ScheduledSummary || !c.NotifyOnUpdate {
		t.Errorf("switches = %v, %v, want both on", c.ScheduledSummary, c.NotifyOnUpdate)
	}
	if len(c.Channels) != 7 {
		t.Fatalf("got %d channels, want 7: %+v", len(c.Channels), c.Channels)
	}

	discord := channelByID(t, c.Channels, "webhook")
	if discord.Kind != kit.KindDiscord || !discord.Enabled || discord.On != kit.OnFailure ||
		discord.Discord.URL != "https://discord.com/api/webhooks/1234567890/AbCdEfGhIjKlMnOpQrStUvWxYz" {
		t.Errorf("webhook became %+v %+v", discord, discord.Discord)
	}
	matrix := channelByID(t, c.Channels, "matrix")
	if matrix.Enabled || matrix.Matrix.Token != "syt_YmFja3Vwcw_token" || matrix.Matrix.Room != "!backups:example.org" {
		t.Errorf("matrix, switched off before, became %+v %+v", matrix, matrix.Matrix)
	}
	gotify := channelByID(t, c.Channels, "gotify")
	if !gotify.Enabled || gotify.Gotify.Token != "AbCdEfGhIjKlMnO" || *gotify.Gotify.PriorityFailure != 9 || *gotify.Gotify.PrioritySuccess != 0 {
		t.Errorf("gotify became %+v %+v", gotify, gotify.Gotify)
	}
	smtp := channelByID(t, c.Channels, "smtp")
	if !smtp.Enabled || smtp.SMTP.Port != 587 || smtp.SMTP.TLS != "starttls" || smtp.SMTP.Password != "app-password" || smtp.SMTP.To != "ops@example.org; admin@example.org" {
		t.Errorf("smtp became %+v %+v", smtp, smtp.SMTP)
	}
	apprise := channelByID(t, c.Channels, "apprise")
	if !apprise.Enabled || apprise.Apprise.URL != "http://apprise:8000/notify/backups" || apprise.Apprise.Tags != "backup" {
		t.Errorf("apprise became %+v %+v", apprise, apprise.Apprise)
	}
	hc := channelByID(t, c.Channels, "healthchecks")
	want := map[string]string{
		"container": "https://hc-ping.com/0c2d7e6b-3f19-4e8a-9a51-5f0b6c1e8d4a",
		"VM":        "https://hc-ping.com/9a510c2d-7e6b-4e8a-3f19-5f0b6c1e8d4a",
	}
	if !hc.Enabled || hc.Healthchecks.URL != "https://hc-ping.com/5f0b6c1e-8d4a-4e8a-9a51-0c2d7e6b3f19" || len(hc.Healthchecks.ByKey) != 2 ||
		hc.Healthchecks.ByKey["container"] != want["container"] || hc.Healthchecks.ByKey["VM"] != want["VM"] {
		t.Errorf("healthchecks became %+v %+v", hc, hc.Healthchecks)
	}
	unraid := channelByID(t, c.Channels, "unraid")
	if !unraid.Enabled || unraid.On != kit.OnFailure {
		t.Errorf("unraid became %+v", unraid)
	}
}

func TestAConvertedConfigIsStoredInTheChannelForm(t *testing.T) {
	data, err := os.ReadFile("testdata/stored-before-channels.json")
	if err != nil {
		t.Fatal(err)
	}
	var old notify.Config
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatal(err)
	}
	stored, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if notify.Legacy(stored) {
		t.Fatalf("the converted config still reads as the old form: %s", stored)
	}
	var again notify.Config
	if err := json.Unmarshal(stored, &again); err != nil {
		t.Fatal(err)
	}
	if len(again.Channels) != len(old.Channels) || !again.ScheduledSummary || !again.NotifyOnUpdate {
		t.Fatalf("reading it back gave %+v", again)
	}
}

func TestAStrictDecodeRefusesAMisspelledField(t *testing.T) {
	var c notify.Config
	if err := c.DecodeStrict([]byte(`{"channels":[],"scheduledSumary":true}`)); err == nil {
		t.Fatal("a misspelled switch was accepted")
	}
	if err := c.DecodeStrict([]byte(`{"channels":[{"id":"a","kind":"unraid","enabled":true,"onn":"always"}]}`)); err == nil {
		t.Fatal("a misspelled channel field was accepted")
	}
	if err := c.DecodeStrict([]byte(`{"channels":[{"id":"a","kind":"unraid","enabled":true}],"scheduledSummary":true}`)); err != nil || len(c.Channels) != 1 {
		t.Fatalf("a well-formed body: %v, %+v", err, c)
	}
}

func TestAFailurePolicyHearsOfFailuresAndOfSuccessesThatLeftSomethingOut(t *testing.T) {
	wh := newService(t)
	c := notify.Config{Channels: []kit.Channel{webhook("hook", wh.URL, kit.OnFailure)}}

	if notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault", Message: "fine", OK: true}) {
		t.Error("a plain success counts as delivered")
	}
	if !notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault: backup FAILED", Message: "broke"}) {
		t.Error("a failure was not delivered")
	}
	notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault: backup incomplete", Message: "some left out", OK: true, Warning: true})
	hits := wh.got()
	if len(hits) != 2 {
		t.Fatalf("webhook got %d messages, want the failure and the warning: %+v", len(hits), hits)
	}
	if hits[0].body != `{"title":"BombVault: backup FAILED","ok":false}` {
		t.Errorf("the failure was sent as %s", hits[0].body)
	}
}

func TestAnItemOfAScheduledRunIsFoldedIntoTheSummaryButStillPings(t *testing.T) {
	wh, hc := newService(t), newService(t)
	c := notify.Config{ScheduledSummary: true, Channels: []kit.Channel{webhook("hook", wh.URL, kit.OnAlways), checks(hc.URL, nil)}}
	ctx := notify.WithMessagesSuppressed(context.Background())

	notify.Send(ctx, c, "container", notify.Event{Title: "BombVault", Message: "item done", OK: true, ItemResult: true})
	if n := len(wh.got()); n != 0 {
		t.Errorf("webhook got %d messages for a folded item", n)
	}
	if hits := hc.got(); len(hits) != 1 || hits[0].path != "/" {
		t.Errorf("health check got %+v, want one success ping", hits)
	}
}

func TestAHeldCheckHearsAFailureWithItsReason(t *testing.T) {
	hc := newService(t)
	c := notify.Config{Channels: []kit.Channel{checks(hc.URL, nil)}}
	notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault", Message: "ok", OK: true, CheckHeld: "nextcloud failed earlier"})
	hits := hc.got()
	if len(hits) != 1 || hits[0].path != "/fail" || hits[0].body != "nextcloud failed earlier" {
		t.Fatalf("health check got %+v", hits)
	}
}

func TestSuppressedChecksAreNotPinged(t *testing.T) {
	hc := newService(t)
	c := notify.Config{Channels: []kit.Channel{checks(hc.URL, nil)}}
	notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "container", notify.Event{Message: "broke"})
	if n := len(hc.got()); n != 0 {
		t.Fatalf("a suppressed check got %d pings", n)
	}
}

func TestEachSectionPingsItsOwnCheck(t *testing.T) {
	hc := newService(t)
	c := notify.Config{Channels: []kit.Channel{checks(hc.URL+"/all", map[string]string{"container": hc.URL + "/containers", "VM": hc.URL + "/vms"})}}
	notify.PingDomainStart(context.Background(), c, "containers")
	notify.PingDomainResult(context.Background(), c, "vms", false, "1 of 2 items failed")
	notify.PingDomainResult(context.Background(), c, "flash", true, "")
	want := []hit{
		{http.MethodGet, "/containers/start", ""},
		{http.MethodPost, "/vms/fail", "1 of 2 items failed"},
		{http.MethodGet, "/all", ""},
	}
	if got := hc.got(); !slices.Equal(got, want) {
		t.Fatalf("checks got %+v, want %+v", got, want)
	}
}

func TestAStartPostsOnlyToAppriseChannelsThatHearEverything(t *testing.T) {
	wh, ap, hc := newService(t), newService(t), newService(t)
	c := notify.Config{Channels: []kit.Channel{
		webhook("hook", wh.URL, kit.OnAlways),
		{ID: "apprise", Kind: kit.KindApprise, Enabled: true, On: kit.OnAlways, Apprise: &kit.Apprise{URL: ap.URL + "/notify/key"}},
		checks(hc.URL, nil),
	}}
	notify.SendStart(context.Background(), c, "flash")
	if n := len(wh.got()); n != 0 {
		t.Errorf("the webhook heard of a start %d times", n)
	}
	hits := ap.got()
	if len(hits) != 1 || !strings.Contains(hits[0].body, `"type":"info"`) || !strings.Contains(hits[0].body, "Backup started (flash)") {
		t.Errorf("apprise got %+v", hits)
	}
	if hits := hc.got(); len(hits) != 1 || hits[0].path != "/start" {
		t.Errorf("health check got %+v", hits)
	}
}

type runner struct {
	mu   sync.Mutex
	runs [][]string
}

func (r *runner) Run(_ context.Context, name string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, append([]string{name}, args...))
	return nil
}

func TestUnraidHearsThroughTheRunnerItIsGiven(t *testing.T) {
	unraid := kit.Channel{ID: "unraid", Kind: kit.KindUnraid, Enabled: true, On: kit.OnFailure}
	c := notify.Config{Channels: []kit.Channel{unraid}}
	if c.TakesMessages() {
		t.Error("an Unraid channel without a way to the host takes messages")
	}
	if notify.Send(context.Background(), c, "vms", notify.Event{Title: "BombVault: VM may still be frozen", Message: "thaw it", Critical: true}) {
		t.Error("an Unraid channel without a way to the host delivered")
	}

	r := &runner{}
	c = c.WithUnraid(r)
	if !c.TakesMessages() {
		t.Error("an Unraid channel with a way to the host takes no messages")
	}
	notify.Send(context.Background(), c, "vms", notify.Event{Title: "BombVault: VM may still be frozen", Message: "thaw it", Critical: true})
	notify.Send(context.Background(), c, "container", notify.Event{Title: "BombVault: backup FAILED", Message: "broke"})
	want := [][]string{
		{kit.UnraidNotifyScript, "-e", "BombVault", "-s", "BombVault: VM may still be frozen", "-d", "thaw it", "-i", "alert"},
		{kit.UnraidNotifyScript, "-e", "BombVault", "-s", "BombVault: backup FAILED", "-d", "broke", "-i", "warning"},
	}
	if len(r.runs) != len(want) {
		t.Fatalf("runner ran %v", r.runs)
	}
	for i := range want {
		if !slices.Equal(r.runs[i], want[i]) {
			t.Errorf("run %d = %q, want %q", i, r.runs[i], want[i])
		}
	}
}

func TestOnlyAChannelThatCarriesMessagesTakesMessages(t *testing.T) {
	never := webhook("hook", "https://hooks.example.org/in", kit.OnNever)
	off := webhook("off", "https://hooks.example.org/in", kit.OnAlways)
	off.Enabled = false
	for _, c := range []notify.Config{
		{},
		{Channels: []kit.Channel{checks("https://hc-ping.com/x", nil)}},
		{Channels: []kit.Channel{never, off}},
	} {
		if c.TakesMessages() {
			t.Errorf("%+v takes messages", c.Channels)
		}
	}
	if !(notify.Config{Channels: []kit.Channel{webhook("hook", "https://hooks.example.org/in", kit.OnFailure)}}).TakesMessages() {
		t.Error("a webhook on failure takes no messages")
	}
}

func TestAHealthCheckAloneMakesNotificationsActive(t *testing.T) {
	if (notify.Config{}).Active() {
		t.Error("a config without channels is active")
	}
	if !(notify.Config{Channels: []kit.Channel{checks("https://hc-ping.com/x", nil)}}).Active() {
		t.Error("a health check alone is not active")
	}
	never := webhook("hook", "https://hooks.example.org/in", kit.OnNever)
	if (notify.Config{Channels: []kit.Channel{never}}).Active() {
		t.Error("a channel that hears nothing is active")
	}
}

func TestADefaultConfigSaysNothing(t *testing.T) {
	if !(notify.Config{}).IsDefault() {
		t.Error("the empty config is not the default")
	}
	if (notify.Config{NotifyOnUpdate: true}).IsDefault() {
		t.Error("a config with a switch on is the default")
	}
	if (notify.Config{Channels: []kit.Channel{{ID: "u", Kind: kit.KindUnraid}}}).IsDefault() {
		t.Error("a config with a channel is the default")
	}
}

func TestCarriesTellsWhetherAnyChannelWantsTheEvent(t *testing.T) {
	failure := notify.Config{Channels: []kit.Channel{webhook("hook", "https://hooks.example.org/in", kit.OnFailure)}}
	if failure.Carries(notify.Event{OK: true}) {
		t.Error("a channel on failure carries a plain success")
	}
	if !failure.Carries(notify.Event{OK: true, Warning: true}) || !failure.Carries(notify.Event{}) {
		t.Error("a channel on failure does not carry a warning or a failure")
	}
	always := notify.Config{Channels: []kit.Channel{webhook("hook", "https://hooks.example.org/in", kit.OnAlways)}}
	if !always.Carries(notify.Event{OK: true}) {
		t.Error("a channel on always does not carry a success")
	}
}

func TestChecksNamesWhatASectionPings(t *testing.T) {
	c := notify.Config{Channels: []kit.Channel{
		checks("https://hc-ping.com/all", map[string]string{"VM": "https://hc-ping.com/vms"}),
		{ID: "more", Kind: kit.KindHealthchecks, Enabled: true, Healthchecks: &kit.Healthchecks{ByKey: map[string]string{"VM": "https://hc-ping.com/vms-too"}}},
	}}
	if got := c.Checks("containers"); !slices.Equal(got, []string{"https://hc-ping.com/all"}) {
		t.Errorf("containers ping %q", got)
	}
	if got := c.Checks("vms"); !slices.Equal(got, []string{"https://hc-ping.com/vms", "https://hc-ping.com/vms-too"}) {
		t.Errorf("vms ping %q", got)
	}
	if got := (notify.Config{}).Checks("flash"); len(got) != 0 {
		t.Errorf("without a check flash pings %q", got)
	}
}
