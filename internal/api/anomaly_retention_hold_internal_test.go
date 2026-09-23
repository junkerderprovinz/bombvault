package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A guard that cannot look must not wave the forget through: a retention pass
// whose anomaly check fails deletes nothing and says so.
func TestHoldCheckErrorSkipsTheForgetAndAlerts(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "plex")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	forget := &forgetTrackingEngine{}
	f.svc.engine = forget

	var sent atomic.Int64
	wh := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		sent.Add(1)
	}))
	defer wh.Close()
	// An unset policy, the state of an install where a channel was configured
	// and the policy never touched.
	if err := f.svc.SetNotifyConfig(notify.Config{WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic"}); err != nil {
		t.Fatal(err)
	}
	f.e.items = func(store.Settings) (map[string]anomalyItemRef, error) {
		return nil, errors.New("the item tables could not be read")
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 5
	f.svc.applyRetention(context.Background(), "/repo", settings, restic.Mode{},
		tagIdentity("container:plex"), "containers", anomalyScope{Kind: anomalyScopeItem, ID: id})

	if forget.forgetCalls() != 0 {
		t.Fatalf("a failed hold check must not forget anything, got %d call(s)", forget.forgetCalls())
	}
	if sent.Load() != 1 {
		t.Fatalf("want one alert about the failed check, got %d", sent.Load())
	}

	f.e.items = func(s store.Settings) (map[string]anomalyItemRef, error) { return f.svc.readAnomalyItems(s) }
	f.e.refresh()
	if got := f.e.summary().EvalErrors; got != 1 {
		t.Fatalf("evalErrors = %d, want the failed check counted once", got)
	}
}
