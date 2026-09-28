package api

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const brokerPassword = "broker-secret-7f3a"

// seedIntegrations gives an instance a Home Assistant link that differs from
// the defaults in every field but enabled, and switches the network
// announcement off. An enabled link that is imported connects in the
// background, which costs its test a few seconds at cleanup.
func seedIntegrations(t *testing.T, st *store.Repo, appKey string, enabled bool) {
	t.Helper()
	sealed, err := secret.Encrypt(appKey, []byte(brokerPassword))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMQTTSettings(store.MQTTSettings{
		Enabled: enabled, Host: "127.0.0.1", Port: 1, Username: "bv", PasswordEnc: sealed,
		TLS: true, Prefix: "home/bv", Buttons: false, NodeID: "a1b2c3d4",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMDNSEnabled(false); err != nil {
		t.Fatal(err)
	}
}

func brokerPasswordOf(t *testing.T, st *store.Repo, appKey string) string {
	t.Helper()
	s, err := st.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.PasswordEnc) == 0 {
		return ""
	}
	plain, err := secret.Decrypt(appKey, s.PasswordEnc)
	if err != nil {
		t.Fatalf("the stored broker password does not open with this instance's key: %v", err)
	}
	return string(plain)
}

func TestExportImportCarriesHomeAssistantAndTheNetworkSwitch(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedIntegrations(t, srcStore, appKeyA, true)

	body, exp := doExport(t, src, "?includeCredentials=true")
	if exp.Credentials == nil || exp.Credentials.MQTTPassword != brokerPassword {
		t.Fatalf("the credentialed export lacks the broker password: %+v", exp.Credentials)
	}
	if bytes.Contains(body, []byte("a1b2c3d4")) {
		t.Fatal("the export carries the node id, so two instances built from one file would share their topics")
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	preview := doImport(t, dst, body, "")
	groups, _ := preview["summary"].(map[string]any)["settingsGroups"].([]any)
	if !slices.Contains(groups, any("homeAssistant")) || !slices.Contains(groups, any("network")) {
		t.Fatalf("the preview names %v, want homeAssistant and network among them", groups)
	}
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}

	got, err := dstStore.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.Host != "127.0.0.1" || got.Port != 1 || got.Username != "bv" || !got.TLS ||
		got.Prefix != "home/bv" || got.Buttons {
		t.Fatalf("the Home Assistant settings were not reproduced: %+v", got)
	}
	if got.NodeID == "" || got.NodeID == "a1b2c3d4" {
		t.Fatalf("the imported instance has node id %q, want its own", got.NodeID)
	}
	if pw := brokerPasswordOf(t, dstStore, appKeyB); pw != brokerPassword {
		t.Fatalf("the broker password after import is %q", pw)
	}
	if on, err := dstStore.MDNSEnabled(); err != nil || on {
		t.Fatalf("the network announcement is %v after importing a file that has it off (err=%v)", on, err)
	}
}

func TestPlainExportLeavesTheBrokerPasswordOutAndTheImportKeepsTheOneThere(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedIntegrations(t, srcStore, appKeyA, false)

	body, exp := doExport(t, src, "")
	if bytes.Contains(body, []byte(brokerPassword)) {
		t.Fatal("the broker password leaked into a credential-free export")
	}
	if exp.HomeAssistant == nil || exp.HomeAssistant.Host != "127.0.0.1" {
		t.Fatalf("the plain export lacks the Home Assistant settings: %+v", exp.HomeAssistant)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	seedIntegrations(t, dstStore, appKeyB, false)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	if pw := brokerPasswordOf(t, dstStore, appKeyB); pw != brokerPassword {
		t.Fatalf("a plain import changed the stored broker password to %q", pw)
	}
}

func TestImportOfFileWithoutIntegrationsKeepsThem(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "homeAssistant")
	delete(raw, "mdnsEnabled")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	seedIntegrations(t, dstStore, appKeyB, false)
	before, err := dstStore.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	after, err := dstStore.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.Host != before.Host || after.NodeID != before.NodeID || after.Enabled || !bytes.Equal(after.PasswordEnc, before.PasswordEnc) {
		t.Fatalf("a file without Home Assistant settings changed them: %+v", after)
	}
	if on, err := dstStore.MDNSEnabled(); err != nil || on {
		t.Fatalf("a file without the network switch turned it to %v (err=%v)", on, err)
	}
}

func TestImportRefusesHomeAssistantSettingsTheCardRefuses(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedIntegrations(t, srcStore, appKeyA, false)
	_, exp := doExport(t, src, "")
	exp.HomeAssistant.Port = 0
	body, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, ""); env["ok"] != false {
		t.Fatalf("a file with broker port 0 passed the preview: %v", env)
	}
	if s, err := dstStore.GetMQTTSettings(); err != nil || s.Port != 1883 {
		t.Fatalf("the refused file changed the settings: %+v (err=%v)", s, err)
	}
}

// A plain file pointing at another broker brings no password along, and the
// one stored here is not sent to that broker either.
func TestImportingAnotherBrokerWithoutItsPasswordDropsTheStoredOne(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedIntegrations(t, srcStore, appKeyA, false)
	s, err := srcStore.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.Host = "10.9.9.9"
	if err := srcStore.SaveMQTTSettings(s); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	seedIntegrations(t, dstStore, appKeyB, false)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	if pw := brokerPasswordOf(t, dstStore, appKeyB); pw != "" {
		t.Fatalf("the password of 127.0.0.1 went along to 10.9.9.9: %q", pw)
	}
}
