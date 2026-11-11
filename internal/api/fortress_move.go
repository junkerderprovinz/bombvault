package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
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

// kitMovedPath is where fortress.Moved hands a phone the keys of the
// pairing the group moved into the fortress with.
const kitMovedPath = "/kit/fortress/v1/moved"

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
			if !current || s.moveOn(f, sec) {
				return
			}
			<-t.C
		}
	}()
}

// moveOn hands the fortress phrase to the instances of the old group that
// are in reach and have not followed into f, and ends the old group once
// every member did. It reports whether there is nobody left to wait for.
func (s *Service) moveOn(f *fortress.Fortress, sec []byte) bool {
	former := s.pairing().Former()
	if former == nil {
		return true
	}
	if len(f.Left(former)) == 0 {
		s.endFormerGroup()
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), moveTimeout)
	defer cancel()
	fortress.Move(ctx, former, sec)
	return false
}

// onOldPhrase names the members of the old group that have not followed
// into the fortress yet.
func (s *Service) onOldPhrase() []string {
	out := []string{}
	f, former := s.fortressNow(), s.pairing().Former()
	if f == nil || former == nil {
		return out
	}
	for _, m := range f.Left(former) {
		out = append(out, cmp.Or(m.Name, m.ID))
	}
	slices.Sort(out)
	return out
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

// handleOldMoved hands a phone of an earlier release the keys of the pairing
// in the fortress where it asks for them. Who gets them is the kit's to
// decide, and such a phone takes them only with ok set.
// GET /api/group/peer/moved
func (s *Service) handleOldMoved(w http.ResponseWriter, r *http.Request) {
	r = r.Clone(r.Context())
	r.URL.Path = kitMovedPath
	rec := httptest.NewRecorder()
	fortress.Moved(http.NotFoundHandler(), s.fortressNow).ServeHTTP(rec, r)
	var keys map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &keys) != nil {
		maps.Copy(w.Header(), rec.Header())
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		return
	}
	keys["ok"] = true
	writeJSON(w, http.StatusOK, keys)
}
