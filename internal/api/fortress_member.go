package api

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	"github.com/junkerderprovinz/parleyport/kit/status"
)

// fortressPeerHeader names the sender of a direct call of the fortress, which
// arrives on the path the group's own calls use.
const fortressPeerHeader = "X-Fortress-Peer"

// fortressPlace is this install's membership of a fortress, open while
// BombVault runs.
type fortressPlace struct {
	mu  sync.Mutex
	f   *fortress.Fortress
	lan *lan.Service
	// device is the server the membership names this instance after.
	device string

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
	if !slices.Contains(cfg.Uses, fortress.ExportAction) {
		cfg.Uses = append(cfg.Uses, fortress.ExportAction)
	}
	cfg.OnRemoved = s.leftFortress
	return cfg
}

// OpenFortress opens the fortress this install has a membership of, and does
// nothing without one.
func (s *Service) OpenFortress(src status.Source) error {
	cfg, err := fortress.Load(s.fortressDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	cfg = s.fortressConfig(cfg, src)
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

// CloseFortress closes the fortress for this run and keeps the membership
// for the next.
func (s *Service) CloseFortress() {
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

// leftFortress forgets the membership once another member took this
// instance out, so that it does not come back on the next start.
func (s *Service) leftFortress() {
	log.Printf("fortress: another member took this instance out of the fortress")
	s.CloseFortress()
	if err := os.RemoveAll(s.fortressDir()); err != nil {
		log.Printf("fortress: forget the membership: %v", err)
	}
}

// fortressNow is the open fortress, or nil.
func (s *Service) fortressNow() *fortress.Fortress {
	s.house.mu.Lock()
	defer s.house.mu.Unlock()
	return s.house.f
}
