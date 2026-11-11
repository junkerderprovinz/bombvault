package api

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// followEvery is how often the instances still on the group's old phrase
// are offered the fortress phrase.
var followEvery = 15 * time.Second

// movedPath is where a phone asks for the keys of the pairing the group
// moved into the fortress with.
const movedPath = "/api/group/peer/moved"

// moveTimeout bounds one round of handing the fortress phrase over.
const moveTimeout = 30 * time.Second

// withPairing opens the pairing group in the fortress cfg describes: under
// the fortress phrase, with whoever the group approved or took out under
// the old one.
func (s *Service) withPairing(cfg fortress.Config, g store.GroupState) (fortress.Config, error) {
	old, err := s.groupSecret(g)
	if err != nil {
		return cfg, err
	}
	cfg.Pairing = &fortress.Pairing{
		Group: kitgroup.Config{
			LAN:            s.pairing().LAN(),
			Store:          pairingStore{s},
			Previous:       old,
			Kinds:          group.Kinds,
			Ports:          group.Ports,
			MaxAnswerBytes: group.MaxAnswerBytes,
			OnChange:       s.pairing().Changed,
		},
		Serve: s.pairing().Handler(),
	}
	cfg.MaxAnswerBytes = group.MaxAnswerBytes
	return cfg, nil
}

// followPairing makes the members of the pairing in f the ones BombVault
// reaches, and keeps offering the instances on the old phrase to follow.
func (s *Service) followPairing(f *fortress.Fortress, sec []byte, gen int) {
	if f.Pairing() == nil {
		return
	}
	s.pairing().Follow(f.Pairing(), group.Domain.GroupKey(sec))
	sec = bytes.Clone(sec)
	go func() {
		t := time.NewTicker(followEvery)
		defer t.Stop()
		for {
			s.house.mu.Lock()
			current := s.house.gen == gen && s.house.f == f
			s.house.mu.Unlock()
			if !current || s.moveOn(sec) {
				return
			}
			<-t.C
		}
	}()
}

// moveOn hands the fortress phrase to the instances of the old group that
// are in reach and have not followed, and ends the old group once every
// member did. It reports whether there is nobody left to wait for.
func (s *Service) moveOn(sec []byte) bool {
	former := s.pairing().Former()
	if former == nil {
		return true
	}
	if len(s.pairing().Left()) == 0 {
		s.endFormerGroup()
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), moveTimeout)
	defer cancel()
	fortress.Move(ctx, former, sec)
	return false
}

// endFormerGroup closes the group on the old phrase. Its members are all in
// the fortress, and nobody is let in with its phrase.
func (s *Service) endFormerGroup() {
	if err := s.store.SetGroupSecret(nil, time.Now()); err != nil {
		log.Printf("group: end the group on the old phrase: %v", err)
		return
	}
	if err := s.store.SetGroupKitState(nil); err != nil {
		log.Printf("group: end the group on the old phrase: %v", err)
	}
	s.applyGroup()
}

// pairingStore keeps what the kit writes about the pairing in the fortress,
// sealed under the APP_KEY. Until it holds anything, it reads the group's
// state, which the kit then moves to the fortress phrase.
type pairingStore struct{ s *Service }

func (p pairingStore) Load() ([]byte, error) {
	m, err := p.s.store.FortressMembership()
	if err != nil {
		return nil, err
	}
	if len(m.PairingEnc) > 0 {
		return secret.Decrypt(p.s.cfg.AppKey, m.PairingEnc)
	}
	return kitStore(p).Load()
}

func (p pairingStore) Save(raw []byte) error {
	enc, err := p.s.sealFortress(raw)
	if err != nil {
		return err
	}
	return p.s.store.SetFortressPairing(enc)
}

// moveKeeper takes the fortress phrase a member of the group hands over as
// it moves into a fortress, and follows it there.
type moveKeeper struct{ s *Service }

func (k moveKeeper) Keep(sec []byte) error {
	m, err := k.s.store.FortressMembership()
	if err != nil {
		return err
	}
	if m.Joined() {
		cur, err := secret.Decrypt(k.s.cfg.AppKey, m.SecretEnc)
		if err != nil || !bytes.Equal(cur, sec) {
			return errInFortress
		}
		return nil
	}
	// The member that hands it over waits for this answer, and joining
	// calls members in turn.
	sec = bytes.Clone(sec)
	go func() {
		k.s.house.door.Lock()
		defer k.s.house.door.Unlock()
		if err := k.s.joinFortress(sec); err != nil && !errors.Is(err, errInFortress) {
			log.Printf("fortress: follow the group into the fortress: %v", err)
		}
	}()
	return nil
}

// movedKeys are the keys of the pairing in the fortress, all a phone needs
// to follow. A phone is never handed the fortress phrase, which opens every
// app of the fortress.
type movedKeys struct {
	OK    bool   `json:"ok"`
	Group string `json:"group"`
	Frame []byte `json:"frame"`
}

// handlePeerMoved hands a paired phone the keys of the pairing once the
// group moved into the fortress. It comes over the group on the old phrase,
// sealed and signed like every answer there, and only to a member the group
// let in. An instance follows with the phrase itself, by fortress.Move.
// GET /api/group/peer/moved
func (s *Service) handlePeerMoved(w http.ResponseWriter, r *http.Request) {
	if sender, ok := group.SenderOf(r.Context()); !ok || sender.Kind != group.KindPhone {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "code": "phonesOnly"})
		return
	}
	if !s.pairing().Following() {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "code": "notMoved"})
		return
	}
	m, err := s.store.FortressMembership()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	sec, err := secret.Decrypt(s.cfg.AppKey, m.SecretEnc)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, movedKeys{OK: true, Group: group.Domain.GroupKey(sec), Frame: group.Domain.FrameKey(sec)})
}
