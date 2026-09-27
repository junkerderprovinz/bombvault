package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAFailedDRDrillNamesTheTargetItRestoredFrom(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
	}))
	defer srv.Close()
	eng := &fakeResticEngine{
		snaps:             []restic.Snapshot{{ID: "aaaa1111bbbb2222", Time: "2026-07-01T00:00:00Z"}},
		lsEntries:         []restic.FileEntry{{Path: "/config/go", Type: "file", Size: 42}},
		statsRestoreBytes: 9_000_000,
	}
	svc, st := drDrillService(t, eng, "flash", "", "")
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "B2", Repo: "s3:b2/flash", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	hetzner, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "Hetzner", Repo: "sftp:u1@hetzner:/flash", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RunRestoreDrill(context.Background(), "flash", "offsite:"+hetzner.ID, "dr", true); err == nil {
		t.Fatal("a verification mismatch must fail the drill")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 || !strings.Contains(bodies[0], "(Hetzner)") || strings.Contains(bodies[0], "(offsite)") {
		t.Fatalf("notifications = %q, want one naming Hetzner", bodies)
	}
}
