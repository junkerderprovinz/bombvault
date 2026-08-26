package notify_test

// Upgrading must not switch a working alert channel off.
//
// The webhook, Matrix and Apprise channels grew explicit enable gates. The
// config they live in is a stored JSON blob, so every config written before the
// gates existed carries no such key, and a plain decode reads an absent key as
// false. That is an operator whose Discord webhook has been telling them about
// failed backups for a year, pulling the new image, and never being told again —
// with the Settings page showing the channel as if it had never been configured.
//
// The blobs below are what an older build actually wrote: the same fields,
// without the three gate keys.

import (
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

const legacyWebhookBlob = `{
	"on": "failure",
	"webhookUrl": "https://discord.com/api/webhooks/123/abc",
	"webhookFormat": "discord"
}`

const legacyAllChannelsBlob = `{
	"on": "failure",
	"webhookUrl": "https://hooks.slack.com/services/T/B/x",
	"webhookFormat": "slack",
	"matrixHomeserver": "https://matrix.example.org",
	"matrixToken": "syt_token",
	"matrixRoom": "!room:example.org",
	"appriseUrl": "http://apprise:8000/notify/key"
}`

func decode(t *testing.T, blob string) notify.Config {
	t.Helper()
	var c notify.Config
	if err := json.Unmarshal([]byte(blob), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return c
}

// TestLegacyWebhookStaysConfigured is the reported case, checked through the
// only thing that matters: Configured() is what the UI and the send paths ask.
func TestLegacyWebhookStaysConfigured(t *testing.T) {
	c := decode(t, legacyWebhookBlob)
	if !c.WebhookEnabled {
		t.Fatal("a stored webhook from before the gate existed must stay enabled — this is an upgrade, not a decision to switch it off")
	}
	if !c.Configured() {
		t.Fatal("Configured() reports no channel at all, so failure alerts stop and the Settings card renders as never-set-up")
	}
}

// TestLegacyMatrixAndAppriseStayConfigured covers the other two gates, each with
// its own field rule (Matrix needs all three fields, Apprise just the URL).
func TestLegacyMatrixAndAppriseStayConfigured(t *testing.T) {
	c := decode(t, legacyAllChannelsBlob)
	if !c.WebhookEnabled || !c.MatrixEnabled || !c.AppriseEnabled {
		t.Fatalf("webhook=%v matrix=%v apprise=%v — every channel that was live before the upgrade must stay live",
			c.WebhookEnabled, c.MatrixEnabled, c.AppriseEnabled)
	}
}

// TestLegacyIncompleteChannelStaysOff: the back-fill reproduces the OLD rule, it
// does not invent one. A half-filled Matrix config never sent before and must
// not be reported as enabled now.
func TestLegacyIncompleteChannelStaysOff(t *testing.T) {
	c := decode(t, `{"on":"failure","matrixHomeserver":"https://matrix.example.org","matrixRoom":"!r:example.org"}`)
	if c.MatrixEnabled {
		t.Fatal("a Matrix config with no access token could never send and must not be back-filled to enabled")
	}
	if c.WebhookEnabled || c.AppriseEnabled {
		t.Fatal("channels with no fields at all must stay off")
	}
	if c.Configured() {
		t.Fatal("a config with no usable channel must not report as configured")
	}
}

// TestExplicitFalseIsHonoured is the other half of the contract, and the one
// that stops the back-fill from becoming its own bug: a user who switched a
// channel off while keeping its URL must not have it switched back on at every
// load.
func TestExplicitFalseIsHonoured(t *testing.T) {
	c := decode(t, `{
		"on": "failure",
		"webhookEnabled": false,
		"webhookUrl": "https://discord.com/api/webhooks/123/abc",
		"matrixEnabled": false,
		"matrixHomeserver": "https://matrix.example.org",
		"matrixToken": "syt_token",
		"matrixRoom": "!room:example.org",
		"appriseEnabled": false,
		"appriseUrl": "http://apprise:8000/notify/key"
	}`)
	if c.WebhookEnabled || c.MatrixEnabled || c.AppriseEnabled {
		t.Fatalf("an explicit off was overridden: webhook=%v matrix=%v apprise=%v",
			c.WebhookEnabled, c.MatrixEnabled, c.AppriseEnabled)
	}
}

// TestRoundTripKeepsExplicitState pins that a config saved by THIS build carries
// the keys, so a later load takes the explicit path rather than the back-fill.
func TestRoundTripKeepsExplicitState(t *testing.T) {
	saved := notify.Config{On: "failure", WebhookURL: "https://example.invalid/hook", WebhookEnabled: false}
	blob, err := json.Marshal(saved)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back notify.Config
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.WebhookEnabled {
		t.Fatal("a round-trip through this build's own encoder must preserve an explicit off")
	}
}

// TestUnmarshalRejectsGarbage: the custom decoder must still fail on a malformed
// blob rather than quietly returning an empty config (a decrypt that produced
// junk is a real error, not "no notifications configured").
func TestUnmarshalRejectsGarbage(t *testing.T) {
	var c notify.Config
	if err := json.Unmarshal([]byte(`{"on":`), &c); err == nil {
		t.Fatal("expected a decode error for a truncated blob")
	}
}
