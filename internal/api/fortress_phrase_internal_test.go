package api

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// plugin is a member that holds only the fortress keys, kept in a folder of
// its own, the way an Unraid plugin joins.
type plugin struct {
	dir string
	id  string
	f   *fortress.Fortress
}

func joinPlugin(t *testing.T, url string, sec []byte) *plugin {
	t.Helper()
	p := &plugin{dir: filepath.Join(t.TempDir(), "fortress")}
	if err := fortress.Join(p.dir, sec, fortress.Membership{App: "firesquire", Device: "tower", Relay: url}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.close)
	return p
}

func (p *plugin) open(t *testing.T) {
	t.Helper()
	cfg, err := fortress.Load(p.dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.f, err = fortress.Open(cfg); err != nil {
		t.Fatal(err)
	}
	p.id = cfg.Identity.ID()
}

func (p *plugin) close() {
	if p.f != nil {
		_ = p.f.Close()
		p.f = nil
	}
}

// reaches reports whether p has another member online.
func (p *plugin) reaches() bool {
	for _, m := range p.f.Members() {
		if m.Online() {
			return true
		}
	}
	return false
}

// fortressOnRelay joins svc to a new fortress on a relay of the test's own
// and returns the relay's address and the secret.
func fortressOnRelay(t *testing.T) (*Service, string, []byte) {
	t.Helper()
	rs := httptest.NewServer(relay.New())
	t.Cleanup(rs.Close)
	svc, st := newFortressSvc(t)
	if err := st.SetGroupRelay("own", rs.URL, false); err != nil {
		t.Fatal(err)
	}
	words, sec := newPhrase(t)
	if err := svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)
	return svc, rs.URL, sec
}

func restartFortress(t *testing.T, svc *Service) {
	t.Helper()
	svc.CloseFortress()
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
}

// backdateChange moves the last change of the phrase ago into the past.
func backdateChange(t *testing.T, svc *Service, ago time.Duration) {
	t.Helper()
	m, err := svc.store.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SetFortressSecret(m.SecretEnc, m.PreviousEnc, time.Now().Add(-ago)); err != nil {
		t.Fatal(err)
	}
}

func keepsPrevious(t *testing.T, svc *Service) bool {
	t.Helper()
	m, err := svc.store.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	return len(m.PreviousEnc) > 0
}

func TestAPluginFollowsAPhraseChangeForThirtyDays(t *testing.T) {
	svc, url, oldSecret := fortressOnRelay(t)
	p := joinPlugin(t, url, oldSecret)
	p.open(t)
	waitLong(t, "BombVault to know the plugin's app", func() bool {
		for _, m := range svc.fortressNow().Members() {
			if m.ID == p.id && m.App.ID == "firesquire" {
				return true
			}
		}
		return false
	})
	// The plugin runs now and then, and is away while the phrase changes.
	p.close()
	words, _, err := svc.ChangeFortressPhrase(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	newSecret, err := phrase.Decode(words)
	if err != nil {
		t.Fatal(err)
	}
	restartFortress(t, svc)

	p.open(t)
	waitLong(t, "the plugin to follow the new phrase", func() bool {
		followed, err := fortress.Follow(context.Background(), p.dir, p.f)
		if err != nil {
			t.Logf("follow: %v", err)
		}
		return followed
	})
	p.close()
	cfg, err := fortress.Load(p.dir)
	if err != nil {
		t.Fatal(err)
	}
	want := fortress.Domain.Keys(newSecret)
	if cfg.Keys.Group != want.Group || !bytes.Equal(cfg.Keys.Frame, want.Frame) || !bytes.Equal(cfg.Keys.PeerAuth, want.PeerAuth) {
		t.Fatal("the plugin holds other keys than the fortress's under the new phrase")
	}
	p.open(t)
	waitLong(t, "the plugin to reach BombVault under the new phrase", p.reaches)
	p.close()

	late := joinPlugin(t, url, oldSecret)
	late.open(t)
	waitLong(t, "BombVault to answer on the old phrase", late.reaches)
	late.close()

	backdateChange(t, svc, 30*24*time.Hour+time.Hour)
	restartFortress(t, svc)
	if keepsPrevious(t, svc) {
		t.Fatal("the old secret is kept 30 days after the change")
	}
	late.open(t)
	time.Sleep(3 * time.Second)
	if late.reaches() {
		t.Error("BombVault answers on the old phrase 30 days after the change")
	}
}

func TestTheOldPhraseClosesAfterThirtyDaysWithoutARestart(t *testing.T) {
	svc, _, _ := fortressOnRelay(t)
	if _, _, err := svc.ChangeFortressPhrase(context.Background()); err != nil {
		t.Fatal(err)
	}
	backdateChange(t, svc, 30*24*time.Hour-2*time.Second)
	restartFortress(t, svc)
	if !keepsPrevious(t, svc) {
		t.Fatal("the old secret went before the 30 days were over")
	}
	waitLong(t, "the old secret to go", func() bool { return !keepsPrevious(t, svc) })
	waitLong(t, "the fortress to open again", func() bool { return svc.fortressNow() != nil })
}
