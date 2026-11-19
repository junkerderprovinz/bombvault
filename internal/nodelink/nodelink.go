// Package nodelink is the private connection between a home instance and one
// node it runs backups on. Both ends derive the link from the node's APP_KEY
// and serve the other end's member key and nobody else, so the link opens no
// way into the pairing group and a node never meets another node.
package nodelink

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/junkerderprovinz/bombvault/internal/group"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
)

// Config is one end of a link. Home sets Seed, the node sets Home.
type Config struct {
	// NodeKey is the node's APP_KEY.
	NodeKey string
	// Seed is the seed of home's member key, on home's end.
	Seed []byte
	// Home is home's member key, on the node's end.
	Home ed25519.PublicKey
	// Name is what the other end sees of this one.
	Name     string
	Relay    group.Mode
	RelayURL string
}

// Link is one end of the connection.
type Link struct {
	g     *kitgroup.Group
	peer  string
	serve atomic.Pointer[http.Handler]
}

// Open starts this end of the link. It returns at once and reaches the relay
// in the background.
func Open(cfg Config) (*Link, error) {
	if (len(cfg.Seed) == 0) == (len(cfg.Home) == 0) {
		return nil, errors.New("nodelink: either the seed of home's key or home's key has to be set")
	}
	secret, err := linkSecret(cfg.NodeKey)
	if err != nil {
		return nil, err
	}
	nodeSeed, err := derive(cfg.NodeKey, "bombvault-node-member")
	if err != nil {
		return nil, err
	}
	node, err := member.FromSeed(member.KindInstance, nodeSeed)
	if err != nil {
		return nil, err
	}
	self, peerKey := node, cfg.Home
	if len(cfg.Seed) > 0 {
		if self, err = member.FromSeed(member.KindInstance, cfg.Seed); err != nil {
			return nil, err
		}
		peerKey = node.Public()
	}
	l := &Link{peer: member.ID(member.KindInstance, peerKey)}
	l.g, err = kitgroup.Open(kitgroup.Config{
		Domain:   group.Domain,
		Secret:   secret,
		Identity: self,
		Self:     kitgroup.Self{Name: cfg.Name, Deployment: "container"},
		Relay:    kitgroup.RelayMode(cfg.Relay),
		RelayURL: strings.TrimSpace(cfg.RelayURL),
		// The other end is vouched for again on its first call after every
		// start, so nothing here needs to outlive the process.
		Store: &memStore{},
		Kinds: []member.Kind{member.KindInstance},
		Vouch: func(id string, key ed25519.PublicKey) bool {
			return id == l.peer && bytes.Equal(key, peerKey)
		},
		VouchedOnly: true,
	}, http.HandlerFunc(l.handle))
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Serve answers the other end's calls with h from now on. Until it is called
// they are answered with 503.
func (l *Link) Serve(h http.Handler) { l.serve.Store(&h) }

func (l *Link) handle(w http.ResponseWriter, r *http.Request) {
	h := l.serve.Load()
	if h == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	(*h).ServeHTTP(w, r)
}

// Call posts body to path at the other end and returns its answer. An error
// means no answer came from the other end.
func (l *Link) Call(ctx context.Context, path string, body []byte) (int, []byte, error) {
	return l.g.Call(ctx, l.peer, http.MethodPost, path, body)
}

// Close leaves the relay and waits for the calls in flight.
func (l *Link) Close() error { return l.g.Close() }

// linkSecret is what the link's keys on the relay are derived from.
func linkSecret(nodeKey string) ([]byte, error) {
	return derive(nodeKey, "bombvault-node-link")
}

func derive(nodeKey, label string) ([]byte, error) {
	key, err := hex.DecodeString(nodeKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("nodelink: the node's APP_KEY is not 64 hex characters")
	}
	return hkdf.Key(sha256.New, key, nil, label, 32)
}

type memStore struct {
	mu  sync.Mutex
	raw []byte
}

func (s *memStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.raw), nil
}

func (s *memStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw = bytes.Clone(b)
	return nil
}
