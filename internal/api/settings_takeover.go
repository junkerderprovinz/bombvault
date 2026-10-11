package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// keyEventSettings is the run a settings restore of another install writes on
// store.KeyTargetID.
const keyEventSettings = "keysettings"

// The kinds of thing an install stores sealed, as a restore names the ones it
// could not bring over.
const (
	sealedCloud         = "cloud"
	sealedRclone        = "rclone"
	sealedRegistry      = "registry"
	sealedNotifications = "notifications"
	sealedHomeAssistant = "home-assistant"
	sealedGroup         = "group"
	sealedPeers         = "peers"
	sealedOther         = "other"
)

// sealedKind says what a sealed column holds. Every column the list does not
// name belongs to a paired instance: its restic password, a login or a token.
func sealedKind(c store.SealedColumn) string {
	switch c.Table + "." + c.Column {
	case "settings.cloud_conf", "settings.cloud_cred_sets":
		return sealedCloud
	case "settings.rclone_conf":
		return sealedRclone
	case "settings.registry_auths":
		return sealedRegistry
	case "settings.notify_conf":
		return sealedNotifications
	case "mqtt_settings.password_enc":
		return sealedHomeAssistant
	case "group_state.secret_enc":
		return sealedGroup
	case "backup_key.key_enc":
		return sealedOther
	}
	return sealedPeers
}

// sealedLoss counts the sealed values of the old install that opened under
// neither key, by kind. The person enters those again.
type sealedLoss struct {
	Count int            `json:"count"`
	Kinds map[string]int `json:"kinds"`
}

func (l *sealedLoss) add(kind string) {
	if l.Kinds == nil {
		l.Kinds = map[string]int{}
	}
	l.Count++
	l.Kinds[kind]++
}

func readSealedLoss(stored string) *sealedLoss {
	var l sealedLoss
	if stored == "" || json.Unmarshal([]byte(stored), &l) != nil || l.Count == 0 {
		return nil
	}
	return &l
}

var errSettingsOtherKey = errors.New("these settings were sealed with the APP_KEY of another server, which its recovery kit names. Start the emergency with that kit under Settings, Restore, then restore the settings again")

// takeOverStagedSettings makes a staged settings database usable under this
// server's APP_KEY before the next boot swaps it in.
//
// During an emergency the database is the old install's: what that install
// sealed is sealed again under this APP_KEY, the key of its backups goes in as
// the adopted one, and this server's login replaces the old one, whose
// password hash no other APP_KEY can check. Outside an emergency the settings
// are taken to be this server's own, and refused when nothing in them opens.
func (s *Service) takeOverStagedSettings(ctx context.Context) error {
	path := filepath.Join(selfrestore.RestoredSnapshotDir(s.cfg.DataDir), "bombvault.sqlite")
	if _, err := os.Stat(path); err != nil {
		return nil //nolint:nilerr // without a staged database the boot has nothing to swap in
	}
	db, err := store.OpenStaged(path)
	if err != nil {
		return err
	}
	s.emergencyMu.Lock()
	s.takeoverLoss = nil
	s.emergencyMu.Unlock()
	own, from := s.cfg.AppKey, ""
	sess, _ := s.currentEmergency()
	foreign := sess != nil && sess.sealingKey != own
	if foreign {
		from = sess.sealingKey
	}
	var opened, moved int
	var lost sealedLoss
	err = store.ResealDatabase(db, func(c store.SealedColumn, sealed []byte) []byte {
		if _, err := secret.Decrypt(own, sealed); err == nil {
			opened++
			return nil
		}
		if plain, err := secret.Decrypt(from, sealed); from != "" && err == nil {
			if out, err := secret.Encrypt(own, plain); err == nil {
				moved++
				return out
			}
		}
		lost.add(sealedKind(c))
		return nil
	})
	if err == nil {
		err = s.finishTakeover(db, sess, foreign, opened, lost)
	}
	if cErr := store.CloseStaged(db); err == nil {
		err = cErr
	}
	if foreign {
		s.keyEvent(ctx, keyEventSettings, err)
		if err == nil {
			log.Printf("api: settings restore: %d sealed values open under this server's key from here on, %d open under neither key and stay as they are", moved, lost.Count)
			s.emergencyMu.Lock()
			s.takeoverLoss = readSealedLoss(lost.json())
			s.emergencyMu.Unlock()
		}
	}
	return err
}

func (l sealedLoss) json() string {
	if l.Count == 0 {
		return ""
	}
	b, _ := json.Marshal(l)
	return string(b)
}

// lastTakeoverLoss returns what the settings restore that just ran could not
// bring over, nil for nothing or for a restore of the server's own settings.
func (s *Service) lastTakeoverLoss() *sealedLoss {
	s.emergencyMu.Lock()
	defer s.emergencyMu.Unlock()
	return s.takeoverLoss
}

// finishTakeover puts the adopted backup key and, for another install's
// settings, this server's login into the staged database.
func (s *Service) finishTakeover(db *sql.DB, sess *emergencySession, foreign bool, opened int, lost sealedLoss) error {
	own := s.cfg.AppKey
	if !foreign {
		if lost.Count > 0 && opened == 0 {
			return errSettingsOtherKey
		}
		// Settings saved before this server took a key over would drop it.
		if has, err := store.HasAdoptedKey(db); err != nil || has {
			return err
		}
		adopted, found, err := s.store.AdoptedKey()
		if err != nil || !found {
			return err
		}
		return store.SetAdoptedKeyIn(db, adopted.KeyEnc, adopted.Source, "")
	}
	if sess.backupKey != own {
		enc, err := secret.Encrypt(own, []byte(sess.backupKey))
		if err != nil {
			return err
		}
		if err := store.SetAdoptedKeyIn(db, enc, sess.source, lost.json()); err != nil {
			return err
		}
	}
	return s.store.CarryLoginInto(db)
}
