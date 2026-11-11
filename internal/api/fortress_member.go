package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log"
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

// fortressPeerHeader names the sender of a direct call of the fortress, which
// arrives on the path the group's own calls use.
const fortressPeerHeader = "X-Fortress-Peer"

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
	for _, use := range []string{fortress.ExportAction, actionUpdatesManaged} {
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
	return s.openFortress()
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
	s.house.mu.Lock()
	s.house.f, s.house.lan, s.house.device = f, cfg.LAN, cfg.Self.Name
	s.house.mu.Unlock()
	return nil
}

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
	if len(m.PreviousEnc) > 0 {
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
		Self:     group.Self{Name: s.fortressDevice()},
		Relay:    group.RelayMode(g.RelayMode),
		RelayURL: g.RelayURL,
		Store:    fortressStore{s},
		Keeper:   fortressKeeper{s},
	}
	if fortressLAN {
		cfg.LAN = lan.New(lan.Config{Group: fortress.Multicast})
	}
	return cfg, true, nil
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
	s.house.f, s.house.lan = nil, nil
	s.house.mu.Unlock()
	if f != nil {
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
// into the fortress without anybody approving it again.
func (s *Service) joinFortress(sec []byte) error {
	if s.fortressNow() != nil {
		return errInFortress
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
	for _, m := range f.Members() {
		if m.Kind == kitmember.KindInstance && !slices.Contains(took, m.ID) && !slices.Contains(left, m.Device) {
			left = append(left, m.Device)
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

// SetFortressDevice names this server for the other members, and an empty
// name goes back to the one BombVault finds. An open fortress takes it at
// once.
func (s *Service) SetFortressDevice(name string) error {
	if err := s.store.SetFortressDevice(strings.TrimSpace(name)); err != nil {
		return err
	}
	m, err := s.store.FortressMembership()
	if err != nil || !m.Joined() || s.fortressNow() == nil {
		return err
	}
	s.reopenFortress()
	return nil
}

// fortressDevice is the name the other members know this server by. ShipLog
// and TowerTorch find BombVault on their own server by it, so it is the
// server's name unless somebody gives another.
func (s *Service) fortressDevice() string {
	if m, err := s.store.FortressMembership(); err == nil && m.Device != "" {
		return m.Device
	}
	return s.defaultFortressDevice()
}

// defaultFortressDevice is the name of the Unraid server, or the name this
// instance has in the pairing group elsewhere.
func (s *Service) defaultFortressDevice() string {
	if name := unraidServerName(s.cfg.FlashDir); name != "" {
		return name
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "BombVault"
	}
	return instanceDisplayName(settings)
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
	if err := k.s.store.SetFortressSecret(sealed, m.SecretEnc); err != nil {
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

// fortressNow is the open fortress, or nil.
func (s *Service) fortressNow() *fortress.Fortress {
	s.house.mu.Lock()
	defer s.house.mu.Unlock()
	return s.house.f
}
