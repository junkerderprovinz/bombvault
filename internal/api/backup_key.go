package api

import (
	"errors"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
)

// The two things APP_KEY does are kept apart here. It seals what this install
// stores, and it gives the backups their key: the restic password and the key
// container and VM definitions are sealed with. An install that takes over
// another one's backups keeps sealing with its APP_KEY and adopts that
// install's backup key, stored sealed like every other secret.

// The states of the backup key, as GET /api/status names them.
const (
	backupKeyOwn     = "own"
	backupKeyAdopted = "adopted"
	// backupKeyLocked is an adopted key that APP_KEY does not open, because
	// APP_KEY changed after the adoption.
	backupKeyLocked = "locked"
)

var errBackupKeyLocked = errors.New("the key of the backups this server took over does not open, because APP_KEY changed after it was adopted. Upload the recovery kit again under Settings, Restore, or set APP_KEY back to what it was")

// lockedPassword stands in for the restic password while the backup key is
// locked. It opens no repository, so nothing is read or written under the
// wrong key in the meantime.
const lockedPassword = "backup-key-locked"

// errBackupKeyUnread is what backups are refused with while the store does
// not say whether a key was adopted.
var errBackupKeyUnread = errors.New("the settings database did not say which key the backups are under, so nothing is backed up or handed out until it does")

// backupKeyState is the backup key as it was last read or set. unread marks a
// locked state that only stands for a store that could not be asked.
type backupKeyState struct {
	state     string
	key       string
	source    string
	adoptedAt int64
	unread    bool
}

// currentBackupKey reads the adopted key once and keeps it. An install without
// one answers with APP_KEY.
func (s *Service) currentBackupKey() backupKeyState {
	s.backupKeyMu.Lock()
	defer s.backupKeyMu.Unlock()
	if s.backupKeyRead {
		return s.backupKeyNow
	}
	own := backupKeyState{state: backupKeyOwn, key: s.cfg.AppKey}
	if s.store == nil {
		s.backupKeyNow, s.backupKeyRead = own, true
		return own
	}
	adopted, found, err := s.store.AdoptedKey()
	if err != nil {
		// APP_KEY in place of an adopted key would write backups, the kit and
		// the deposits under the wrong one. The answer is not kept, so the
		// next reader asks again.
		log.Printf("api: backup key: %v; backups are refused until the key can be read", err)
		return backupKeyState{state: backupKeyLocked, unread: true}
	}
	s.backupKeyRead = true
	if !found {
		s.backupKeyNow = own
		return own
	}
	s.backupKeyNow = backupKeyState{state: backupKeyLocked, source: adopted.Source, adoptedAt: adopted.AdoptedAt}
	if plain, err := secret.Decrypt(s.cfg.AppKey, adopted.KeyEnc); err == nil && foreignKeyRe.Match(plain) {
		s.backupKeyNow.state, s.backupKeyNow.key = backupKeyAdopted, string(plain)
	} else {
		log.Printf("api: backup key: the adopted key does not open under this APP_KEY, so backups are refused until it is adopted again")
	}
	return s.backupKeyNow
}

// forgetBackupKey makes the next reader ask the store again.
func (s *Service) forgetBackupKey() {
	s.backupKeyMu.Lock()
	s.backupKeyRead = false
	s.backupKeyMu.Unlock()
}

// backupKey returns the key the backups are under.
func (s *Service) backupKey() (string, error) {
	k := s.currentBackupKey()
	switch {
	case k.unread:
		return "", errBackupKeyUnread
	case k.state == backupKeyLocked:
		return "", errBackupKeyLocked
	}
	return k.key, nil
}

// backupPassword is the restic password of this install's repositories.
func (s *Service) backupPassword() string {
	key, err := s.backupKey()
	if err != nil {
		return lockedPassword
	}
	return restickey.Derive(key)
}

// sealDefinition seals a container or VM definition for the backup storage.
func (s *Service) sealDefinition(def []byte) ([]byte, error) {
	key, err := s.backupKey()
	if err != nil {
		return nil, err
	}
	return secret.Encrypt(key, def)
}

// openDefinition opens what sealDefinition sealed.
func (s *Service) openDefinition(enc []byte) ([]byte, error) {
	key, err := s.backupKey()
	if err != nil {
		return nil, err
	}
	return secret.Decrypt(key, enc)
}
