package api

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	kitmember "github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

// fortressLAN finds the members on this network as well as through the relay.
// The tests leave it off, so that none of them listens on the multicast group.
var fortressLAN = true

var (
	errInFortress = errors.New("BombVault is in a fortress already")
	errNoFortress = errors.New("BombVault is in no fortress")
)

// fortressPlace is this install's membership of a fortress, open while
// BombVault runs.
type fortressPlace struct {
	// door is held while the fortress opens, closes or changes its phrase,
	// which takes a while, so that mu stays quick for those who only read.
	door sync.Mutex

	mu  sync.Mutex
	f   *fortress.Fortress
	lan *lan.Service
	// device is the server the membership names this instance after.
	device string
	src    status.Source
	// gen counts the fortresses opened, so that a removal arriving late does
	// not end a membership opened after it.
	gen int
	// changing is set while this instance changes the phrase itself, when the
	// keeper must not open the fortress again under it.
	changing bool
	// forget opens the fortress again once the secret of the phrase before
	// the last change is due to go.
	forget *time.Timer
	// direct answers the direct calls of f and the groups beside it. It
	// bounds the calls it reads for more than one group, so it is kept as
	// long as those groups stay the same.
	direct directRoute

	// logos are the marks the apps publish in their status, by app and
	// version, and logoAsked when each was last asked for.
	logos     map[string]string
	logoAsked map[string]time.Time

	// requests are the backups members asked for, by id, the oldest first in
	// order.
	requests map[string]*houseRequest
	order    []string
	// start begins a container backup, StartBackup outside tests, and retry
	// is how long a request waits before it tries again.
	start func(ctx context.Context, name string) (bool, error)
	retry time.Duration
}

// fortressDir holds the membership `fortress join -dir` wrote, and the state
// the fortress keeps next to it.
func (s *Service) fortressDir() string { return filepath.Join(s.cfg.DataDir, "fortress") }

// fortressConfig fills in what BombVault is and does in the fortress a
// membership describes.
func (s *Service) fortressConfig(cfg fortress.Config, src status.Source) fortress.Config {
	cfg.App = fortress.App{ID: "bombvault", Name: "BombVault", Version: Version}
	cfg.Self.Deployment = "container"
	if addr := s.selfDirectURL(); addr != "" {
		cfg.Self.Address, cfg.Self.DirectURL = addr, addr
	}
	cfg.Status = src
	cfg.Actions = []fortress.Action{
		{Name: actionBackupLast, Run: s.backupLast},
		{Name: actionBackupRequest, Run: s.backupRequest},
		{Name: actionSnapshotsBrowse, Run: s.browseSnapshots},
	}
	for _, use := range []string{fortress.ExportAction, fortress.ImportAction, actionUpdatesManaged} {
		if !slices.Contains(cfg.Uses, use) {
			cfg.Uses = append(cfg.Uses, use)
		}
	}
	return cfg
}

// OpenFortress opens the fortress this install has a membership of, and does
// nothing without one.
func (s *Service) OpenFortress(src status.Source) error {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	s.house.mu.Lock()
	s.house.src = src
	s.house.mu.Unlock()
	sec, err := s.groupSecretToAdopt()
	if err != nil || sec == nil {
		return errors.Join(err, s.openFortress())
	}
	return s.joinFortress(sec)
}

// AdoptGroupPhrase makes the group's phrase the fortress's as well, so that
// another app given the same words finds BombVault there. It does nothing
// while BombVault is in a fortress or in no group.
func (s *Service) AdoptGroupPhrase() error {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	sec, err := s.groupSecretToAdopt()
	if err != nil || sec == nil {
		return err
	}
	return s.joinFortress(sec)
}

// groupSecretToAdopt is the secret of the group's phrase while BombVault is
// in no fortress, and nil otherwise. A membership from the console counts as
// a fortress.
func (s *Service) groupSecretToAdopt() ([]byte, error) {
	m, err := s.store.FortressMembership()
	if err != nil || m.Joined() || s.fortressNow() != nil {
		return nil, err
	}
	if _, err := fortress.Load(s.fortressDir()); !errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return nil, err
	}
	return s.groupSecret(g)
}

// onGroupsPhrase reports whether the fortress BombVault is in runs on the
// group's own phrase, the fortress it adopted.
func (s *Service) onGroupsPhrase() bool {
	m, err := s.store.FortressMembership()
	if err != nil || !m.Joined() {
		return false
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return false
	}
	own, err := s.groupSecret(g)
	if err != nil || own == nil {
		return false
	}
	cur, err := secret.Decrypt(s.cfg.AppKey, m.SecretEnc)
	return err == nil && bytes.Equal(cur, own)
}

// openFortress opens the membership kept in the database, or else the one
// written by the console, with door held.
func (s *Service) openFortress() error {
	cfg, ok, err := s.fortressMembership()
	if err != nil || !ok {
		return err
	}
	s.house.mu.Lock()
	src := s.house.src
	s.house.gen++
	gen := s.house.gen
	s.house.mu.Unlock()
	cfg = s.fortressConfig(cfg, src)
	cfg.OnRemoved = func() { s.removedFrom(gen) }
	if cfg.LAN != nil {
		cfg.LAN.Start()
	}
	f, err := fortress.Open(cfg)
	if err != nil {
		if cfg.LAN != nil {
			_ = cfg.LAN.Close()
		}
		return err
	}
	var forget *time.Timer
	if len(cfg.Previous) > 0 {
		m, err := s.store.FortressMembership()
		if err != nil {
			log.Printf("fortress: read when the phrase changed: %v", err)
		} else {
			forget = time.AfterFunc(time.Until(m.Changed.Add(previousKept)), s.reopenFortress)
		}
	}
	s.house.mu.Lock()
	s.house.f, s.house.lan, s.house.device, s.house.forget = f, cfg.LAN, cfg.Self.Name, forget
	s.house.mu.Unlock()
	s.followPairing(f, cfg.Secret, gen)
	return nil
}

// previousKept is how long the fortress is given the secret of the phrase
// before a change. The kit answers on that phrase meanwhile, so that the
// members that hold only keys, such as the Unraid plugins, fetch the new ones.
const previousKept = 30 * 24 * time.Hour

// fortressMembership is the Config of the membership this install keeps,
// and false for none.
func (s *Service) fortressMembership() (fortress.Config, bool, error) {
	m, err := s.store.FortressMembership()
	if err != nil {
		return fortress.Config{}, false, err
	}
	if !m.Joined() {
		cfg, err := fortress.Load(s.fortressDir())
		if errors.Is(err, fs.ErrNotExist) {
			return fortress.Config{}, false, nil
		}
		return cfg, err == nil, err
	}
	sec, err := secret.Decrypt(s.cfg.AppKey, m.SecretEnc)
	if err != nil {
		return fortress.Config{}, false, err
	}
	seed, err := secret.Decrypt(s.cfg.AppKey, m.SeedEnc)
	if err != nil {
		return fortress.Config{}, false, err
	}
	id, err := kitmember.FromSeed(kitmember.KindInstance, seed)
	if err != nil {
		return fortress.Config{}, false, err
	}
	var previous []byte
	switch {
	case len(m.PreviousEnc) == 0:
	case time.Since(m.Changed) >= previousKept:
		if err := s.store.ForgetFortressPrevious(); err != nil {
			return fortress.Config{}, false, err
		}
	default:
		if previous, err = secret.Decrypt(s.cfg.AppKey, m.PreviousEnc); err != nil {
			return fortress.Config{}, false, err
		}
	}
	// The fortress reaches the members on other networks the way the
	// pairing group does.
	g, err := s.store.GetGroupState()
	if err != nil {
		return fortress.Config{}, false, err
	}
	cfg := fortress.Config{
		Secret:   sec,
		Previous: previous,
		Identity: id,
		Self:     group.Self{Name: s.pairName()},
		Relay:    group.RelayMode(g.RelayMode),
		RelayURL: g.RelayURL,
		Store:    fortressStore{s},
		Keeper:   fortressKeeper{s},
	}
	if fortressLAN {
		cfg.LAN = lan.New(lan.Config{Group: fortress.Multicast})
	}
	cfg, err = s.withPairing(cfg, g)
	return cfg, err == nil, err
}

// CloseFortress closes the fortress for this run and keeps the membership
// for the next.
func (s *Service) CloseFortress() {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	s.closeFortress()
}

func (s *Service) closeFortress() {
	s.house.mu.Lock()
	f, l := s.house.f, s.house.lan
	if s.house.forget != nil {
		s.house.forget.Stop()
	}
	s.house.f, s.house.lan, s.house.forget = nil, nil, nil
	s.house.mu.Unlock()
	if f != nil {
		s.pairing().Follow(nil, "")
		if err := f.Close(); err != nil {
			log.Printf("fortress: close: %v", err)
		}
	}
	if l != nil {
		_ = l.Close()
	}
}

// reopenFortress closes the fortress and opens it on what the membership keeps.
func (s *Service) reopenFortress() {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	s.closeFortress()
	if err := s.openFortress(); err != nil {
		log.Printf("fortress: open again: %v", err)
	}
}

// JoinFortress joins the fortress of words under a new member key. A phrase
// that does not decode comes back as the *phrase.DecodeError.
func (s *Service) JoinFortress(words string) error {
	sec, err := phrase.Decode(words)
	if err != nil {
		return err
	}
	defer clear(sec)
	s.house.door.Lock()
	defer s.house.door.Unlock()
	return s.joinFortress(sec)
}

// joinFortress joins the fortress of sec with door held. It signs with the
// key the pairing group knows this instance by, so the pairing can move
// into the fortress without anybody approving it again. The fortress on the
// group's own phrase gives way to it, and the group moves along.
func (s *Service) joinFortress(sec []byte) error {
	if s.fortressNow() != nil {
		if !s.onGroupsPhrase() {
			return errInFortress
		}
		if err := s.forgetFortress(); err != nil {
			return err
		}
	}
	own, err := s.depositSeed()
	if err != nil {
		return err
	}
	sealed, err := s.sealFortress(sec)
	if err != nil {
		return err
	}
	seed, err := s.sealFortress(own)
	if err != nil {
		return err
	}
	if err := s.store.JoinFortress(sealed, seed); err != nil {
		return err
	}
	if err := s.openFortress(); err != nil {
		return errors.Join(err, s.store.LeaveFortress())
	}
	return nil
}

// LeaveFortress takes BombVault out of its fortress and forgets the
// membership. The other members keep it as away until somebody removes it,
// and the phrase brings it back.
func (s *Service) LeaveFortress() error {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	m, err := s.store.FortressMembership()
	if err != nil {
		return err
	}
	if s.fortressNow() == nil && !m.Joined() {
		return errNoFortress
	}
	return s.forgetFortress()
}

// forgetFortress closes the fortress and drops what the install keeps of it,
// with door held.
func (s *Service) forgetFortress() error {
	s.closeFortress()
	return errors.Join(s.store.LeaveFortress(), os.RemoveAll(s.fortressDir()))
}

// removedFrom forgets the membership once another member took this instance
// out, so that it does not come back on the next start. The kit calls it on
// a goroutine of the fortress, which closing waits for, so the work runs on
// one of its own.
func (s *Service) removedFrom(gen int) {
	go func() {
		s.house.door.Lock()
		defer s.house.door.Unlock()
		s.house.mu.Lock()
		current := s.house.gen == gen && s.house.f != nil
		s.house.mu.Unlock()
		if !current {
			return
		}
		log.Printf("fortress: another member took this instance out of the fortress")
		if err := s.forgetFortress(); err != nil {
			log.Printf("fortress: forget the membership: %v", err)
		}
		if err := s.store.SetFortressRemovedHere(true); err != nil {
			log.Printf("fortress: keep that another member took this instance out: %v", err)
		}
	}()
}

// ChangeFortressPhrase gives the fortress a new phrase and opens it on that.
// It returns the new words and the devices that were out of reach, which
// still hold the old ones.
func (s *Service) ChangeFortressPhrase(ctx context.Context) (string, []string, error) {
	s.house.door.Lock()
	defer s.house.door.Unlock()
	f := s.fortressNow()
	if f == nil {
		return "", nil, errNoFortress
	}
	s.house.mu.Lock()
	s.house.changing = true
	s.house.mu.Unlock()
	took, err := f.ChangePhrase(ctx)
	s.house.mu.Lock()
	s.house.changing = false
	s.house.mu.Unlock()
	if err != nil {
		return "", nil, err
	}
	left := []string{}
	noted := map[string]bool{}
	note := func(id, name string) {
		if slices.Contains(took, id) || noted[id] {
			return
		}
		noted[id] = true
		if !slices.Contains(left, name) {
			left = append(left, name)
		}
	}
	for _, m := range f.Members() {
		if m.Kind == kitmember.KindInstance {
			note(m.ID, m.Device)
		}
	}
	// Every instance the pairing let in counts, whether or not the fortress
	// met it: one still moving in may have taken the phrase from before
	// without arriving. Those left behind are listed apart.
	behind := map[string]bool{}
	for _, m := range f.Behind() {
		behind[m.ID] = true
	}
	if p := f.Pairing(); p != nil {
		for _, m := range p.Members() {
			if m.Kind == kitmember.KindInstance && m.State == group.StateApproved && !behind[m.ID] {
				note(m.ID, cmp.Or(m.Name, m.ID))
			}
		}
	}
	slices.Sort(left)
	s.closeFortress()
	if err := s.openFortress(); err != nil {
		return "", nil, err
	}
	words, err := s.fortressPhrase()
	return words, left, err
}

// renamed hands a new instance name to the group and to an open fortress,
// so that the members see it without a restart.
func (s *Service) renamed() {
	s.applyGroup()
	s.house.mu.Lock()
	device := s.house.device
	open := s.house.f != nil
	s.house.mu.Unlock()
	if open && device != s.pairName() {
		s.reopenFortress()
	}
}

// pairName is the one name this instance goes by, in its group and in
// the fortress: the instance's name, or else the server's. ShipLog and
// TowerTorch find BombVault on their own server by it.
func (s *Service) pairName() string {
	if settings, err := s.store.GetSettings(); err == nil && strings.TrimSpace(settings.InstanceName) != "" {
		return instanceDisplayName(settings)
	}
	return s.defaultPairName()
}

// defaultPairName is the name an instance without one of its own goes
// by: the Unraid server's, or BombVault.
func (s *Service) defaultPairName() string {
	if name := unraidServerName(s.cfg.FlashDir); name != "" {
		return name
	}
	return "BombVault"
}

// unraidServerName reads the server's name from config/ident.cfg on the
// flash, empty where there is none.
func unraidServerName(flash string) string {
	if flash == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(flash, "config", "ident.cfg")) //nolint:gosec // G304: the flash mount the operator names
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "NAME="); ok {
			return strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"`))
		}
	}
	return ""
}

// fortressPhrase is the phrase of the fortress BombVault is in.
func (s *Service) fortressPhrase() (string, error) {
	m, err := s.store.FortressMembership()
	if err != nil {
		return "", err
	}
	if !m.Joined() {
		return "", errNoFortress
	}
	sec, err := secret.Decrypt(s.cfg.AppKey, m.SecretEnc)
	if err != nil {
		return "", err
	}
	return phrase.Encode(sec)
}

func (s *Service) sealFortress(plain []byte) ([]byte, error) {
	return secret.Encrypt(s.cfg.AppKey, plain)
}

// fortressStore keeps what the kit writes about the fortress, sealed under
// the APP_KEY.
type fortressStore struct{ s *Service }

func (k fortressStore) Load() ([]byte, error) {
	m, err := k.s.store.FortressMembership()
	if err != nil || len(m.StateEnc) == 0 {
		return nil, err
	}
	return secret.Decrypt(k.s.cfg.AppKey, m.StateEnc)
}

func (k fortressStore) Save(raw []byte) error {
	enc, err := k.s.sealFortress(raw)
	if err != nil {
		return err
	}
	return k.s.store.SetFortressState(enc)
}

// fortressKeeper keeps a new phrase, whether this instance changed it or
// another member handed it over. Another member's arrives inside a call the
// fortress serves, so the fortress opens on it afterwards.
type fortressKeeper struct{ s *Service }

func (k fortressKeeper) Keep(sec []byte) error {
	m, err := k.s.store.FortressMembership()
	if err != nil {
		return err
	}
	sealed, err := k.s.sealFortress(sec)
	if err != nil {
		return err
	}
	if err := k.s.store.SetFortressSecret(sealed, m.SecretEnc, time.Now()); err != nil {
		return err
	}
	if err := k.s.store.SetFortressRemoved(nil); err != nil {
		return err
	}
	k.s.house.mu.Lock()
	own := k.s.house.changing
	k.s.house.mu.Unlock()
	if !own {
		go k.s.reopenFortress()
	}
	return nil
}

// directRoute is the direct handler of a fortress and the group on its own
// phrase beside it.
type directRoute struct {
	f   *fortress.Fortress
	own *group.Group
	h   http.Handler
}

// fortressDirect answers the direct calls that reach f, of the fortress, of
// its pairing and of the group on its own phrase, which all arrive at one
// path. That phrase is the fortress's or the one the group moves away from.
func (s *Service) fortressDirect(f *fortress.Fortress) http.Handler {
	own := s.pairing().Own()
	s.house.mu.Lock()
	defer s.house.mu.Unlock()
	if d := s.house.direct; d.h == nil || d.f != f || d.own != own {
		s.house.direct = directRoute{f: f, own: own, h: f.DirectHandler(own)}
	}
	return s.house.direct.h
}

// logoAgain is how long an app that showed no logo is left alone before it
// is asked again.
const logoAgain = 10 * time.Minute

// appLogo is the logo the app of m publishes in its status, once BombVault
// has read it. Until then it asks m in the background, and a new version of
// the app is asked again.
func (s *Service) appLogo(f *fortress.Fortress, m fortress.Member) string {
	key := m.App.ID + " " + m.App.Version
	s.house.mu.Lock()
	defer s.house.mu.Unlock()
	if logo, ok := s.house.logos[key]; ok {
		return logo
	}
	if !m.Online() || time.Since(s.house.logoAsked[key]) < logoAgain {
		return ""
	}
	if s.house.logoAsked == nil {
		s.house.logoAsked = map[string]time.Time{}
	}
	s.house.logoAsked[key] = time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), moveTimeout)
		defer cancel()
		r, err := f.Status(ctx, m.ID, "en")
		if err != nil || status.CheckIcon(r.App.Icon) != nil {
			return
		}
		s.house.mu.Lock()
		defer s.house.mu.Unlock()
		if s.house.logos == nil {
			s.house.logos = map[string]string{}
		}
		s.house.logos[key] = r.App.Icon
	}()
	return ""
}

// fortressNow is the open fortress, or nil.
func (s *Service) fortressNow() *fortress.Fortress {
	s.house.mu.Lock()
	defer s.house.mu.Unlock()
	return s.house.f
}
