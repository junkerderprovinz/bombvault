package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// zfsReplicaListTimeout caps the pool sizes the server list asks for, so one
// server that does not answer cannot hold the settings page.
const zfsReplicaListTimeout = 5 * time.Second

// zfsReplicaCleanTimeout bounds the removal of an item's replica snapshots
// and bookmarks from its tree.
const zfsReplicaCleanTimeout = 2 * time.Minute

// ZFSReplicaServerView is a ZFS server as the settings show it, with the
// size of its pool as it last answered and the items replicating there.
type ZFSReplicaServerView struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Host      string   `json:"host"`
	User      string   `json:"user"`
	Port      int      `json:"port"`
	Pool      string   `json:"pool"`
	Root      string   `json:"root"`
	Enabled   bool     `json:"enabled"`
	FreeBytes int64    `json:"freeBytes"`
	SizeBytes int64    `json:"sizeBytes"`
	UsedBy    []string `json:"usedBy"`
}

func (v ZFSReplicaServerView) fields() map[string]any {
	return map[string]any{
		"id": v.ID, "name": v.Name, "host": v.Host, "user": v.User, "port": v.Port,
		"pool": v.Pool, "root": v.Root, "enabled": v.Enabled,
		"freeBytes": v.FreeBytes, "sizeBytes": v.SizeBytes, "usedBy": v.UsedBy,
	}
}

// ZFSReplicaServerPatch carries the fields of a server edit. A nil field
// keeps its value.
type ZFSReplicaServerPatch struct {
	Name    *string `json:"name"`
	Host    *string `json:"host"`
	User    *string `json:"user"`
	Port    *int    `json:"port"`
	Pool    *string `json:"pool"`
	Root    *string `json:"root"`
	Enabled *bool   `json:"enabled"`
}

func (s *Service) zfsReplicaServerView(srv store.ZFSReplicaServer, users []string) ZFSReplicaServerView {
	pool := s.knownZFSReplicaPool(srv.ID)
	if users == nil {
		users = []string{}
	}
	return ZFSReplicaServerView{
		ID: srv.ID, Name: srv.Name, Host: srv.Host, User: srv.User, Port: srv.Port,
		Pool: srv.Pool, Root: srv.Root, Enabled: srv.Enabled,
		FreeBytes: pool.FreeBytes, SizeBytes: pool.SizeBytes, UsedBy: users,
	}
}

// ListZFSReplicaServers returns every server. The enabled ones are asked for
// their pool size first, all at once; one that does not answer in time keeps
// what it said last.
func (s *Service) ListZFSReplicaServers(ctx context.Context) ([]ZFSReplicaServerView, error) {
	servers, err := s.store.ListZFSReplicaServers()
	if err != nil {
		return nil, err
	}
	users, err := s.store.ZFSReplicaServerUsers()
	if err != nil {
		return nil, err
	}
	lctx, cancel := context.WithTimeout(ctx, zfsReplicaListTimeout)
	defer cancel()
	var wg sync.WaitGroup
	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.probeZFSReplicaServer(lctx, srv); err != nil {
				log.Printf("api: zfs replica: server %s did not report its pool: %v", srv.Name, err)
			}
		}()
	}
	wg.Wait()
	out := make([]ZFSReplicaServerView, 0, len(servers))
	for _, srv := range servers {
		out = append(out, s.zfsReplicaServerView(srv, users[srv.ID]))
	}
	return out, nil
}

// zfsReplicaServerRefusalErr is zfsReplicaServerRefusal as an error.
func zfsReplicaServerRefusalErr(srv store.ZFSReplicaServer) error {
	if msg := zfsReplicaServerRefusal(srv.Name, srv.Host, srv.User, srv.Port, srv.Pool, srv.Root); msg != "" {
		return errors.New("the ZFS server " + msg)
	}
	return nil
}

// CreateZFSReplicaServer stores a new server, switched on.
func (s *Service) CreateZFSReplicaServer(srv store.ZFSReplicaServer) (ZFSReplicaServerView, error) {
	srv.ID, srv.Name, srv.Enabled = "", strings.TrimSpace(srv.Name), true
	if err := zfsReplicaServerRefusalErr(srv); err != nil {
		return ZFSReplicaServerView{}, err
	}
	created, err := s.store.CreateZFSReplicaServer(srv)
	if err != nil {
		return ZFSReplicaServerView{}, err
	}
	return s.zfsReplicaServerView(created, nil), nil
}

// PatchZFSReplicaServer applies an edit. A server reached at another address
// is another machine, so the host key pinned for it goes.
func (s *Service) PatchZFSReplicaServer(id string, p ZFSReplicaServerPatch) (ZFSReplicaServerView, error) {
	srv, ok, err := s.store.GetZFSReplicaServer(id)
	if err != nil {
		return ZFSReplicaServerView{}, err
	}
	if !ok {
		return ZFSReplicaServerView{}, zfsRefuse("not-found", id)
	}
	was := srv
	for _, f := range []struct {
		from *string
		to   *string
	}{{p.Name, &srv.Name}, {p.Host, &srv.Host}, {p.User, &srv.User}, {p.Pool, &srv.Pool}, {p.Root, &srv.Root}} {
		if f.from != nil {
			*f.to = strings.TrimSpace(*f.from)
		}
	}
	if p.Port != nil {
		srv.Port = *p.Port
	}
	if p.Enabled != nil {
		srv.Enabled = *p.Enabled
	}
	if err := zfsReplicaServerRefusalErr(srv); err != nil {
		return ZFSReplicaServerView{}, err
	}
	if err := s.store.UpdateZFSReplicaServer(srv); err != nil {
		return ZFSReplicaServerView{}, err
	}
	if srv.Host != was.Host || srv.Port != was.Port || srv.User != was.User {
		s.removeZFSReplicaKnownHosts(id)
	}
	users, err := s.store.ZFSReplicaServerUsers()
	if err != nil {
		return ZFSReplicaServerView{}, err
	}
	return s.zfsReplicaServerView(srv, users[id]), nil
}

// DeleteZFSReplicaServer removes a server. While items replicate there it is
// refused with in-use, unless detach switches their replica off first, which
// cleans their trees the way switching it off by hand does. The copies on the
// server stay.
func (s *Service) DeleteZFSReplicaServer(ctx context.Context, id string, detach bool) error {
	if _, ok, err := s.store.GetZFSReplicaServer(id); err != nil || !ok {
		return err
	}
	users, err := s.store.ZFSReplicaServerUsers()
	if err != nil {
		return err
	}
	if len(users[id]) > 0 && !detach {
		return zfsRefuse("in-use", strings.Join(users[id], ", "))
	}
	for _, item := range users[id] {
		unlock, ok := s.lockZFSReplica(item)
		if !ok {
			return errZFSReplicaBusy
		}
		d, err := s.store.GetZFSDataset(item)
		if err == nil {
			s.cleanZFSReplicaSource(ctx, d)
		}
		unlock()
		if err != nil {
			return err
		}
	}
	if err := s.store.DeleteZFSReplicaServer(id); err != nil {
		return err
	}
	s.removeZFSReplicaKnownHosts(id)
	return nil
}

// cleanZFSReplicaSource removes the replica snapshots and bookmarks from an
// item's tree. What it cannot remove is logged: the setting changes either
// way, and the leftovers are BombVault's own names a later clean finds again.
func (s *Service) cleanZFSReplicaSource(ctx context.Context, d store.ZFSDataset) {
	end, err := s.zfsReplicaHostEnd()
	if err != nil {
		log.Printf("api: zfs replica: the replica snapshots of %s stay, the host cannot be reached: %v", d.Dataset, err)
		return
	}
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), zfsReplicaCleanTimeout)
	defer cancel()
	if err := zfsrepl.Clean(cctx, end, d.Dataset); err != nil {
		log.Printf("api: zfs replica: removing the replica snapshots of %s failed: %v", d.Dataset, err)
	}
}

// TestZFSReplicaConnection connects to a server that is not stored yet and
// lists its pools. The host key it sees is pinned in a file of its own that
// goes again afterwards: a stored server pins its key on its first run.
func (s *Service) TestZFSReplicaConnection(ctx context.Context, host, user string, port int) ([]zfs.Pool, error) {
	if !argvSafe(host) || !argvSafe(user) || port < 1 || port > 65535 {
		return nil, fmt.Errorf("the ZFS server needs a host and a user without spaces and a port between 1 and 65535")
	}
	if err := os.MkdirAll(s.zfsReplicaKeyDir(), 0o700); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(s.zfsReplicaKeyDir(), "test-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // a leftover test directory holds only a known_hosts file
	end, err := s.zfsReplicaServerEnd(store.ZFSReplicaServer{Host: host, User: user, Port: port}, filepath.Join(dir, "known_hosts"))
	if err != nil {
		return nil, err
	}
	qctx, cancel := context.WithTimeout(ctx, zfsReplicaQuickTimeout)
	defer cancel()
	return zfsReplicaPools(qctx, end)
}

// TestZFSReplicaServer lists the pools of a stored server.
func (s *Service) TestZFSReplicaServer(ctx context.Context, id string) ([]zfs.Pool, error) {
	srv, ok, err := s.store.GetZFSReplicaServer(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, zfsRefuse("not-found", id)
	}
	qctx, cancel := context.WithTimeout(ctx, zfsReplicaQuickTimeout)
	defer cancel()
	return s.probeZFSReplicaServer(qctx, srv)
}

// probeZFSReplicaServer lists a server's pools and keeps what its own pool
// reported for the server list.
func (s *Service) probeZFSReplicaServer(ctx context.Context, srv store.ZFSReplicaServer) ([]zfs.Pool, error) {
	end, err := s.zfsReplicaServerEnd(srv, s.zfsReplicaKnownHosts(srv.ID))
	if err != nil {
		return nil, err
	}
	pools, err := zfsReplicaPools(ctx, end)
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		if p.Name == srv.Pool {
			s.rememberZFSReplicaPool(srv.ID, p)
		}
	}
	return pools, nil
}

// ZFSLocalPools lists the pools of this instance's own host, for a replica a
// paired instance sends here.
func (s *Service) ZFSLocalPools(ctx context.Context) ([]zfs.Pool, error) {
	end, err := s.zfsReplicaHostEnd()
	if err != nil {
		return nil, err
	}
	qctx, cancel := context.WithTimeout(ctx, zfsReplicaQuickTimeout)
	defer cancel()
	return zfsReplicaPools(qctx, end)
}

func zfsReplicaPools(ctx context.Context, end zfsrepl.End) ([]zfs.Pool, error) {
	out, err := end.Run(ctx, zfs.PoolsArgs())
	if err != nil {
		return nil, err
	}
	return zfs.ParsePools(out)
}
