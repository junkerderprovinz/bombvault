package api

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func sealedForTest(t *testing.T, s *Service, plain []byte) string {
	t.Helper()
	enc, err := secret.Encrypt(s.cfg.AppKey, plain)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(enc)
}

func TestTheStartConvertsANotifyConfigFromBeforeChannels(t *testing.T) {
	s := unraidNotifyService(t, nil)
	old, err := os.ReadFile("../notify/testdata/stored-before-channels.json")
	if err != nil {
		t.Fatal(err)
	}
	sealed := sealedForTest(t, s, old)
	if _, err := s.store.MutateSettings(func(st *store.Settings) error { st.NotifyConf = sealed; return nil }); err != nil {
		t.Fatal(err)
	}

	if err := s.ConvertNotifyChannels(); err != nil {
		t.Fatal(err)
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := base64.StdEncoding.DecodeString(settings.NotifyConf)
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		t.Fatal(err)
	}
	if notify.Legacy(plain) || !strings.Contains(string(plain), `"channels":[`) {
		t.Fatalf("the stored config is not in the channel form: %s", plain)
	}
	c, err := s.NotifyConfig()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, ch := range c.Channels {
		ids = append(ids, ch.ID)
	}
	if got := strings.Join(ids, ","); got != "webhook,matrix,gotify,smtp,apprise,healthchecks,unraid" {
		t.Errorf("channels after the start: %s", got)
	}
	if !c.ScheduledSummary || !c.NotifyOnUpdate {
		t.Error("the switches were lost")
	}
	kept, at, err := s.store.NotifyConfBeforeChannels()
	if err != nil || kept != sealed || at == 0 {
		t.Fatalf("the old config was not kept: %q at %d (%v)", kept, at, err)
	}

	if err := s.ConvertNotifyChannels(); err != nil {
		t.Fatal(err)
	}
	if again, atAgain, _ := s.store.NotifyConfBeforeChannels(); again != sealed || atAgain != at {
		t.Error("a second start converted again")
	}
}

func TestTheStartLeavesAnInstanceWithoutNotificationsAlone(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.ConvertNotifyChannels(); err != nil {
		t.Fatal(err)
	}
	if kept, _, _ := s.store.NotifyConfBeforeChannels(); kept != "" {
		t.Errorf("kept %q without a config", kept)
	}
}
