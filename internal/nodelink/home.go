package nodelink

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
)

// homePrefix starts every NODE_HOME, so a value from a later format is told
// apart instead of read wrong.
const homePrefix = "bvhome1."

// Home is what a node knows of the home instance it belongs to before they
// first talk: its id, its member key and the relay both ends use.
type Home struct {
	ID       string            `json:"id"`
	Key      ed25519.PublicKey `json:"key"`
	Relay    group.Mode        `json:"relay"`
	RelayURL string            `json:"url,omitempty"`
}

var errHome = errors.New("NODE_HOME does not belong to this APP_KEY; copy both from the same node on the home instance")

func (h Home) check() error {
	switch {
	case h.ID == "":
		return errors.New("nodelink: home has no id")
	case len(h.Key) != ed25519.PublicKeySize:
		return errors.New("nodelink: home's key is not an Ed25519 key")
	case !group.ValidMode(h.Relay) || h.Relay == group.ModeOff:
		return errors.New("nodelink: a node needs a relay to reach home")
	case h.Relay == group.ModeOwn && strings.TrimSpace(h.RelayURL) == "":
		return errors.New("nodelink: the own relay has no address")
	}
	return nil
}

// EncodeHome spells h as the NODE_HOME of the node with nodeKey. The value is
// sealed under that key, so it is refused next to any other APP_KEY and when
// a character of it changed.
func EncodeHome(nodeKey string, h Home) (string, error) {
	if err := h.check(); err != nil {
		return "", err
	}
	mac, err := homeMAC(nodeKey)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	mac.Write(body)
	return homePrefix + base64.RawURLEncoding.EncodeToString(append(mac.Sum(nil), body...)), nil
}

// DecodeHome reads the NODE_HOME of the node with nodeKey.
func DecodeHome(nodeKey, raw string) (Home, error) {
	b64, ok := strings.CutPrefix(strings.TrimSpace(raw), homePrefix)
	if !ok {
		return Home{}, fmt.Errorf("NODE_HOME has to start with %q", homePrefix)
	}
	sealed, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil || len(sealed) <= sha256.Size {
		return Home{}, errHome
	}
	mac, err := homeMAC(nodeKey)
	if err != nil {
		return Home{}, err
	}
	sum, body := sealed[:sha256.Size], sealed[sha256.Size:]
	mac.Write(body)
	if !hmac.Equal(sum, mac.Sum(nil)) {
		return Home{}, errHome
	}
	var h Home
	if err := json.Unmarshal(body, &h); err != nil {
		return Home{}, errHome
	}
	return h, h.check()
}

func homeMAC(nodeKey string) (hash.Hash, error) {
	key, err := derive(nodeKey, "bombvault-node-home")
	if err != nil {
		return nil, err
	}
	return hmac.New(sha256.New, key), nil
}

// NodeID is the id the node with nodeKey goes by on its link.
func NodeID(nodeKey string) (string, error) {
	id, err := nodeIdentity(nodeKey)
	if err != nil {
		return "", err
	}
	return id.ID(), nil
}

// NewNodeKey returns a fresh APP_KEY for a node.
func NewNodeKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func nodeIdentity(nodeKey string) (member.Identity, error) {
	seed, err := derive(nodeKey, "bombvault-node-member")
	if err != nil {
		return member.Identity{}, err
	}
	return member.FromSeed(member.KindInstance, seed)
}
