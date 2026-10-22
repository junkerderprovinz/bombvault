package api_test

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
)

// A repository another instance made without encryption takes no copy from an
// instance that encrypts, so its connection test says so rather than passing,
// and does not ask to switch encryption off for everything.
func TestAnOffsiteRepoInTheOtherEncryptionModeFailsItsTest(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = true
	s.ContainersOffsite = "rest:http://192.168.1.2:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	no := false
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{existingMode: &no})

	reachable, _, err := svc.TestOffsite(context.Background(), "containers")

	if err == nil || !reachable {
		t.Fatalf("TestOffsite = reachable %v, %v; want reachable and an error", reachable, err)
	}
	for _, want := range []string{"created unencrypted", "new, empty location"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
}
