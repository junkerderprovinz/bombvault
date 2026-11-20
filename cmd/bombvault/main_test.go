package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

func TestPlatformForTrueNAS(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	p := platformFor(platform.KindTrueNAS)
	if _, ok := p.(platform.TrueNAS); !ok {
		t.Fatalf("platformFor(KindTrueNAS) = %T, want platform.TrueNAS", p)
	}
	if got := p.Kind(); got != platform.KindTrueNAS {
		t.Fatalf("platformFor(KindTrueNAS).Kind() = %q, want %q", got, platform.KindTrueNAS)
	}
	if strings.Contains(buf.String(), "no implementation") {
		t.Fatalf("platformFor(KindTrueNAS) must not log the Phase A not-implemented-yet warning anymore, log=%q", buf.String())
	}
}

func TestPlatformForUnraid(t *testing.T) {
	if _, ok := platformFor(platform.KindUnraid).(platform.Unraid); !ok {
		t.Fatalf("platformFor(KindUnraid) = %T, want platform.Unraid", platformFor(platform.KindUnraid))
	}
}

func TestPlatformForGeneric(t *testing.T) {
	if _, ok := platformFor(platform.KindGeneric).(platform.Generic); !ok {
		t.Fatalf("platformFor(KindGeneric) = %T, want platform.Generic", platformFor(platform.KindGeneric))
	}
}

func TestPlatformForUnknownKindFallsBackToGeneric(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	p := platformFor(platform.Kind("amiga-os"))
	if _, ok := p.(platform.Generic); !ok {
		t.Fatalf("platformFor(unknown) = %T, want platform.Generic", p)
	}
	if !strings.Contains(buf.String(), "amiga-os") {
		t.Fatalf("an unrecognized Kind must still be logged, log=%q", buf.String())
	}
}

// TestLogSchedulerTimezone sets time.Local directly because Go caches it at
// startup, so changing TZ alone would not move the clock. A fixed zone keeps
// the test independent of tzdata.
func TestLogSchedulerTimezone(t *testing.T) {
	prevLocal := time.Local
	defer func() { time.Local = prevLocal }()

	cases := []struct {
		name     string
		tz       string
		local    *time.Location
		want     []string
		unwanted []string
	}{
		{
			name:     "unset falls back to UTC and says so",
			tz:       "",
			local:    time.UTC,
			want:     []string{"planning in UTC", "TZ is NOT set", "02:30 UTC"},
			unwanted: []string{"did NOT resolve"},
		},
		{
			name:     "unresolvable zone is called out, not silently UTC",
			tz:       "Europe/Berln",
			local:    time.UTC,
			want:     []string{"did NOT resolve", "Europe/Berln", "fell back to UTC"},
			unwanted: []string{"TZ is NOT set"},
		},
		{
			name:     "resolved zone is named",
			tz:       "Europe/Berlin",
			local:    time.FixedZone("CEST", 2*60*60),
			want:     []string{"planning in CEST", "Europe/Berlin"},
			unwanted: []string{"TZ is NOT set", "did NOT resolve"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TZ", tc.tz)
			time.Local = tc.local

			var buf strings.Builder
			prev := log.Writer()
			log.SetOutput(&buf)
			defer log.SetOutput(prev)

			logSchedulerTimezone()

			got := buf.String()
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("log missing %q\ngot: %s", want, got)
				}
			}
			for _, bad := range tc.unwanted {
				if strings.Contains(got, bad) {
					t.Errorf("log must not contain %q\ngot: %s", bad, got)
				}
			}
		})
	}
}

func TestARoundCountsAnEntryWithNowhereToGoAsFailed(t *testing.T) {
	if err := roundItemErr(fmt.Errorf("files: %w", api.ErrNoPlace)); err == nil {
		t.Fatal("roundItemErr passed over an entry with nowhere to back up to")
	}
	if err := roundItemErr(errors.New("disk full")); err == nil {
		t.Fatal("roundItemErr dropped a real failure")
	}
}

// memStore is a migrated store in memory.
func memStore(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func TestANodePlansNoBackupsOfItsOwn(t *testing.T) {
	st := memStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled = true
	settings.ContainersSchedule = "30 2 * * *"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir()}, st, nil, nil, nil)

	planned := func(mode config.Mode) []schedule.NextRun {
		s := newScheduler(mode, svc, st)
		s.Start()
		defer s.Stop()
		return s.NextRuns()
	}
	if len(planned(config.ModeFull)) == 0 {
		t.Fatal("the full app plans nothing, so the test proves nothing")
	}
	if runs := planned(config.ModeNode); len(runs) != 0 {
		t.Errorf("a node plans %+v", runs)
	}
}

func TestANodeJoinsTheHomeItsVariablesName(t *testing.T) {
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	t.Cleanup(rs.CloseClientConnections)
	appKey := strings.Repeat("b", 64)
	homeKey := make([]byte, 32)
	nodeHome, err := nodelink.EncodeHome(appKey, nodelink.Home{ID: "home-1", Key: homeKey, Relay: group.ModeOwn, RelayURL: rs.URL})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppKey: appKey, DataDir: t.TempDir(), Mode: config.ModeNode}
	svc := api.NewService(cfg, memStore(t), nil, nil, nil)

	leave, err := joinHome(cfg, svc)
	if err != nil {
		t.Fatalf("without NODE_HOME: %v", err)
	}
	leave()

	cfg.NodeHome = nodeHome
	leave, err = joinHome(cfg, svc)
	if err != nil {
		t.Fatal(err)
	}
	leave()

	typo := []byte(nodeHome)
	typo[len(typo)/2] ^= 1
	cfg.NodeHome = string(typo)
	if _, err := joinHome(cfg, svc); err == nil {
		t.Error("a NODE_HOME with a typo was taken")
	}
}

func TestANodeRunsWhatHomePassesOnThroughItsOwnRoutes(t *testing.T) {
	st := memStore(t)
	cfg := config.Config{AppKey: strings.Repeat("c", 64), DataDir: t.TempDir(), Mode: config.ModeNode}
	svc := api.NewService(cfg, st, nil, nil, nil)
	router := nodeRouter(cfg, st, nil, svc, newScheduler(config.ModeNode, svc, st), progress.NewStore())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/runs", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("GET /api/runs on a node = %d %s", w.Code, w.Body)
	}
}
