package api

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// zfsReplicaQuickTimeout bounds a look at a target that a page waits for: a
// pool listing, a test, the snapshots kept there.
const zfsReplicaQuickTimeout = 15 * time.Second

// errZFSReplicaBusy is what a second start on an item answers while its
// replica runs or restores.
var errZFSReplicaBusy = errors.New("a replica run or restore of this item is in progress")

// zfsReplicaRuntime is the replica's part of the Service: one lock per item,
// separate from the domain lock so a first full send of several terabytes
// never holds up a backup, and the pools each server reported last.
type zfsReplicaRuntime struct {
	mu      sync.Mutex
	running map[string]bool
	pools   map[string]zfs.Pool
	// work counts the runs started in the background, so a test can wait for
	// them.
	work sync.WaitGroup

	// The seams a test fills in. Left nil, the ends are reached over SSH and
	// the engine runs as it is.
	hostEnd   func() (zfsrepl.End, error)
	serverEnd func(srv store.ZFSReplicaServer, knownHosts string) (zfsrepl.End, error)
	run       func(ctx context.Context, source, target zfsrepl.End, e zfsrepl.Entry) (zfsrepl.Result, error)
	bringBack func(ctx context.Context, from, to zfsrepl.End, r zfsrepl.Restore) (zfsrepl.Restored, error)
}

// lockZFSReplica takes the item's replica lock, or reports that a run or a
// restore holds it.
func (s *Service) lockZFSReplica(id string) (func(), bool) {
	rt := &s.replica
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.running[id] {
		return nil, false
	}
	if rt.running == nil {
		rt.running = map[string]bool{}
	}
	rt.running[id] = true
	return func() {
		rt.mu.Lock()
		delete(rt.running, id)
		rt.mu.Unlock()
	}, true
}

func (s *Service) zfsReplicaRunning(id string) bool {
	s.replica.mu.Lock()
	defer s.replica.mu.Unlock()
	return s.replica.running[id]
}

func (s *Service) rememberZFSReplicaPool(serverID string, p zfs.Pool) {
	s.replica.mu.Lock()
	defer s.replica.mu.Unlock()
	if s.replica.pools == nil {
		s.replica.pools = map[string]zfs.Pool{}
	}
	s.replica.pools[serverID] = p
}

func (s *Service) knownZFSReplicaPool(serverID string) zfs.Pool {
	s.replica.mu.Lock()
	defer s.replica.mu.Unlock()
	return s.replica.pools[serverID]
}

// zfsReplicaKeyDir holds the one key this instance offers every ZFS server,
// and below it one known_hosts per server.
func (s *Service) zfsReplicaKeyDir() string { return filepath.Join(s.cfg.DataDir, "ssh-replica") }

func (s *Service) zfsReplicaKnownHosts(serverID string) string {
	return filepath.Join(s.zfsReplicaKeyDir(), serverID, "known_hosts")
}

// ZFSReplicaPublicKey returns the line a ZFS server has to put into
// authorized_keys, creating the key on first use.
func (s *Service) ZFSReplicaPublicKey() (string, error) {
	c := sshconn.NewIsolated("", "", "", s.zfsReplicaKeyDir(), "")
	if err := c.EnsureKey(); err != nil {
		return "", err
	}
	return c.PublicKey()
}

// zfsReplicaHostEnd is this instance's own pool owner as a replica end: the
// commands go through the ZFS domain's host, the streams through the same SSH
// connection.
func (s *Service) zfsReplicaHostEnd() (zfsrepl.End, error) {
	if s.replica.hostEnd != nil {
		return s.replica.hostEnd()
	}
	host, ok := s.zfs.(*zfs.SSHHost)
	if !ok {
		return nil, zfsRefuse("ssh-missing", "")
	}
	conn, ok := s.ssh.(zfsrepl.Streamer)
	if !ok {
		return nil, zfsRefuse("ssh-missing", "")
	}
	return zfsrepl.NewSSHEnd(host, conn), nil
}

// zfsReplicaServerEnd reaches a ZFS server with the replica key, pinning its
// host key in knownHosts.
func (s *Service) zfsReplicaServerEnd(srv store.ZFSReplicaServer, knownHosts string) (zfsrepl.End, error) {
	if s.replica.serverEnd != nil {
		return s.replica.serverEnd(srv, knownHosts)
	}
	c := sshconn.NewIsolated(srv.Host, srv.User, strconv.Itoa(srv.Port), s.zfsReplicaKeyDir(), knownHosts)
	if err := c.EnsureKey(); err != nil {
		return nil, err
	}
	return zfsrepl.NewSSHEnd(zfs.NewSSHHost(c), c), nil
}

func (s *Service) zfsReplicaEngine() func(context.Context, zfsrepl.End, zfsrepl.End, zfsrepl.Entry) (zfsrepl.Result, error) {
	if s.replica.run != nil {
		return s.replica.run
	}
	return zfsrepl.Run
}

func (s *Service) zfsReplicaBringBack() func(context.Context, zfsrepl.End, zfsrepl.End, zfsrepl.Restore) (zfsrepl.Restored, error) {
	if s.replica.bringBack != nil {
		return s.replica.bringBack
	}
	return zfsrepl.BringBack
}

// zfsReplicaTarget is where an item replicates to, resolved for a run.
type zfsReplicaTarget struct {
	end zfsrepl.End
	// base is <root>/<server folder> on a ZFS server. For a paired instance it
	// is the server folder alone, and the receiving side puts it under the
	// root it chose.
	base   string
	server *store.ZFSReplicaServer
}

// zfsReplicaTargetFor resolves the end an item sends to and where its
// members land there. A member that landed before keeps its folder, so a
// renamed instance goes on writing where it started.
func (s *Service) zfsReplicaTargetFor(ctx context.Context, d store.ZFSDataset) (zfsReplicaTarget, error) {
	folder, err := s.zfsReplicaFolder(d)
	if err != nil {
		return zfsReplicaTarget{}, err
	}
	switch d.Replica.TargetKind {
	case store.ZFSReplicaTargetServer:
		srv, ok, err := s.store.GetZFSReplicaServer(d.Replica.TargetID)
		if err != nil {
			return zfsReplicaTarget{}, err
		}
		if !ok {
			return zfsReplicaTarget{}, zfsRefuse("replica-off", d.Replica.TargetID)
		}
		if !srv.Enabled {
			return zfsReplicaTarget{}, zfsRefuse("server-disabled", srv.Name)
		}
		end, err := s.zfsReplicaServerEnd(srv, s.zfsReplicaKnownHosts(srv.ID))
		if err != nil {
			return zfsReplicaTarget{}, err
		}
		return zfsReplicaTarget{end: end, base: srv.Root + "/" + folder, server: &srv}, nil
	case store.ZFSReplicaTargetPeer:
		end, err := s.zfsReplicaPeerEnd(ctx, d)
		if err != nil {
			return zfsReplicaTarget{}, err
		}
		return zfsReplicaTarget{end: end, base: folder}, nil
	}
	return zfsReplicaTarget{}, zfsRefuse("replica-off", d.Dataset)
}

// zfsReplicaFolder is the folder the item's members land in below the
// target's root: the one an earlier run used, or this instance's name.
func (s *Service) zfsReplicaFolder(d store.ZFSDataset) (string, error) {
	st, ok, err := s.store.GetZFSReplicaState(d.ID, d.Dataset)
	if err != nil {
		return "", err
	}
	if ok {
		if base, cut := strings.CutSuffix(st.TargetPath, "/"+d.Dataset); cut {
			return base[strings.LastIndexByte(base, '/')+1:], nil
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", err
	}
	return zfsReplicaFolderName(instanceDisplayName(settings)), nil
}

// zfsReplicaFolderName turns an instance name into one dataset name
// component. Characters ZFS does not take become dashes.
func zfsReplicaFolderName(name string) string {
	folder := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '_' || r == '.' || r == ':' || r == '-':
			return r
		}
		return '-'
	}, strings.TrimSpace(name))
	folder = strings.TrimLeft(folder, "-.")
	if folder == "" {
		return "BombVault"
	}
	return folder
}

// removeZFSReplicaKnownHosts forgets the host key pinned for a server whose
// address changed or that is gone.
func (s *Service) removeZFSReplicaKnownHosts(serverID string) {
	if err := os.RemoveAll(filepath.Dir(s.zfsReplicaKnownHosts(serverID))); err != nil {
		log.Printf("api: zfs replica: removing the known_hosts of server %s failed: %v", serverID, err)
	}
}
