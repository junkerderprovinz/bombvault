package api

import (
	"context"
	"database/sql"
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
	own, from := s.cfg.AppKey, ""
	sess, _ := s.currentEmergency()
	foreign := sess != nil && sess.sealingKey != own
	if foreign {
		from = sess.sealingKey
	}
	var opened, moved, lost int
	err = store.ResealDatabase(db, func(sealed []byte) []byte {
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
		lost++
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
			log.Printf("api: settings restore: %d sealed values open under this server's key from here on, %d open under neither key and stay as they are", moved, lost)
		}
	}
	return err
}

// finishTakeover puts the adopted backup key and, for another install's
// settings, this server's login into the staged database.
func (s *Service) finishTakeover(db *sql.DB, sess *emergencySession, foreign bool, opened, lost int) error {
	own := s.cfg.AppKey
	if !foreign {
		if lost > 0 && opened == 0 {
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
		return store.SetAdoptedKeyIn(db, adopted.KeyEnc, adopted.Source)
	}
	if sess.backupKey != own {
		enc, err := secret.Encrypt(own, []byte(sess.backupKey))
		if err != nil {
			return err
		}
		if err := store.SetAdoptedKeyIn(db, enc, sess.source); err != nil {
			return err
		}
	}
	return s.store.CarryLoginInto(db)
}
