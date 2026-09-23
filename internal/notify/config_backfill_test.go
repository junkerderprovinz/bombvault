package notify_test

import (
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// These blobs predate the webhookEnabled, matrixEnabled and appriseEnabled
// keys. Decoding them must keep a working channel on.
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

func TestLegacyWebhookStaysConfigured(t *testing.T) {
	c := decode(t, legacyWebhookBlob)
	if !c.WebhookEnabled {
		t.Fatal("a stored webhook from before the gate existed must stay enabled: this is an upgrade, not a decision to switch it off")
	}
	if !c.Configured() {
		t.Fatal("Configured() reports no channel at all, so failure alerts stop and the Settings card renders as never-set-up")
	}
}

func TestLegacyMatrixAndAppriseStayConfigured(t *testing.T) {
	c := decode(t, legacyAllChannelsBlob)
	if !c.WebhookEnabled || !c.MatrixEnabled || !c.AppriseEnabled {
		t.Fatalf("webhook=%v matrix=%v apprise=%v; every channel that was live before the upgrade must stay live",
			c.WebhookEnabled, c.MatrixEnabled, c.AppriseEnabled)
	}
}

// TestLegacyIncompleteChannelStaysOff checks that a Matrix config without a
// token, which could never send, stays off.
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

// TestRoundTripKeepsExplicitState checks that a saved config writes the switch
// keys, so loading it again does not back-fill them.
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

// TestUnmarshalRejectsGarbage checks that a decrypt that produced junk is an
// error, not an empty config.
func TestUnmarshalRejectsGarbage(t *testing.T) {
	var c notify.Config
	if err := json.Unmarshal([]byte(`{"on":`), &c); err == nil {
		t.Fatal("expected a decode error for a truncated blob")
	}
}
