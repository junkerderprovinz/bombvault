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

// stagedOwner says whose settings a staged database holds.
type stagedOwner int

const (
	// stagedOwn are this server's settings.
	stagedOwn stagedOwner = iota
	// stagedForeign are the settings of the install whose kit started the
	// emergency in progress.
	stagedForeign
	// stagedOther are settings of an install whose key this server has not.
	stagedOther
	// stagedUnknown are settings from before the check value with nothing
	// sealed in them, which could be anybody's.
	stagedUnknown
)

// whoseSettings decides by what the database itself shows: the check value
// of the key it ran with and the key its sealed values open under. own and
// from are the APP_KEY of this server and of the emergency in progress, from
// empty without one. That an emergency is in progress says nothing about
// settings that are shown to be this server's.
func whoseSettings(check, own, from string, opened, lost int) stagedOwner {
	switch {
	case check == secret.KeyCheck(own):
		return stagedOwn
	case from != "" && check == secret.KeyCheck(from):
		return stagedForeign
	// A server that ran under a mistyped APP_KEY for a while wrote that
	// key's check value over values that are still sealed with its own.
	case opened > 0:
		return stagedOwn
	case check != "":
		return stagedOther
	case from != "":
		return stagedForeign
	case lost > 0:
		return stagedOther
	}
	return stagedUnknown
}

// RecordKeyCheck writes the check value of this server's APP_KEY into its
// settings, where a later restore of them finds it.
func (s *Service) RecordKeyCheck() error {
	return s.store.SetKeyCheck(secret.KeyCheck(s.cfg.AppKey))
}

var errSettingsOtherKey = errors.New("these settings were sealed with the APP_KEY of another server, which its recovery kit names. Start the emergency with that kit on the Restore page under Emergency, then restore the settings again")

// takeOverStagedSettings makes a staged settings database usable under this
// server's APP_KEY before the next boot swaps it in.
//
// Settings of the install an emergency was started for are the old install's:
// what that install sealed is sealed again under this APP_KEY, the key of its
// backups goes in as the adopted one, and this server's login replaces the
// old one, whose password hash no other APP_KEY can check. Settings of any
// other install are refused. Where the database does not show whose it is,
// it is taken for this server's and gets this server's login all the same.
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
	// Only a restore that names an emergency is part of it. One that names
	// none goes by this server's key alone, whatever is in progress.
	own, from := s.cfg.AppKey, ""
	var sess *emergencySession
	if id := restoreEmergency(ctx); id != "" {
		if sess, err = s.emergencyByID(id); err != nil {
			_ = db.Close()
			return err
		}
		if sess.sealingKey != own {
			from = sess.sealingKey
		}
	}
	var opened, moved int
	var lost sealedLoss
	whose := stagedOwn
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
		whose, err = s.stagedOwnerOf(db, from, opened, lost.Count)
	}
	if err == nil {
		err = s.finishTakeover(db, sess, whose, lost)
	}
	if cErr := store.CloseStaged(db); err == nil {
		err = cErr
	}
	if whose == stagedUnknown && err == nil {
		log.Printf("api: settings restore: the settings do not say which server they are of, so this server's login stays")
	}
	if whose == stagedForeign {
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

type restoreEmergencyKey struct{}

// forEmergency marks a settings restore as part of the emergency id names.
func forEmergency(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, restoreEmergencyKey{}, id)
}

func restoreEmergency(ctx context.Context) string {
	id, _ := ctx.Value(restoreEmergencyKey{}).(string)
	return id
}

// stagedOwnerOf reads from a staged database whose settings it holds. opened
// and lost count its sealed values that open under this server's key and
// under none.
func (s *Service) stagedOwnerOf(db *sql.DB, from string, opened, lost int) (stagedOwner, error) {
	check, err := store.KeyCheckIn(db)
	if err != nil {
		return stagedOther, err
	}
	factor, err := store.SecondFactorSealIn(db)
	if err != nil {
		return stagedOther, err
	}
	if len(factor) > 0 {
		if _, err := secret.Decrypt(s.cfg.AppKey, factor); err == nil {
			opened++
		} else if _, err := secret.Decrypt(from, factor); from == "" || err != nil {
			lost++
		}
	}
	return whoseSettings(check, s.cfg.AppKey, from, opened, lost), nil
}

// finishTakeover puts the adopted backup key and, for settings that are not
// shown to be this server's, this server's login into the staged database.
func (s *Service) finishTakeover(db *sql.DB, sess *emergencySession, whose stagedOwner, lost sealedLoss) error {
	own := s.cfg.AppKey
	if whose == stagedOther {
		return errSettingsOtherKey
	}
	if whose != stagedForeign {
		if whose == stagedUnknown {
			if err := s.store.CarryLoginInto(db); err != nil {
				return err
			}
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
	// The restore stores the key of the emergency like an adoption does, so
	// it goes by the same refusals unless the key was adopted already.
	if now := s.currentBackupKey(); now.state == backupKeyLocked || now.key != sess.backupKey {
		if err := s.adoptionRefusal(sess, false); err != nil {
			return err
		}
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
