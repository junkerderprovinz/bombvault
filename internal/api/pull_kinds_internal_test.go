package api

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAPullSourceOnAnAzureSetOpensWithTheAccountAndKey(t *testing.T) {
	const theirKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{blobSet()}); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(theirKey))
	if err != nil {
		t.Fatal(err)
	}
	s.engine = &pullRecorder{}
	ps := store.PullSource{ID: "p1", Name: "Tower", Repo: "azure:backups:/their", AppKeyEnc: enc, CredsRef: "blob", Domain: "containers", Enabled: true}
	_, mode, err := s.pullOpen(context.Background(), ps, settingsOf(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"AZURE_ACCOUNT_NAME=acct", "AZURE_ACCOUNT_KEY=key"}; !slices.Equal(mode.Env, want) {
		t.Fatalf("env = %v, want %v", mode.Env, want)
	}
}
