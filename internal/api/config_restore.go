package api

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ResetSignInsAfterConfigRestore ends every way in a restored configuration
// database may bring back, while the restore is marked as applied: it can hold
// MCP keys and API tokens revoked after it was saved, passkeys removed since
// and the session epoch from before a sign-out everywhere. Its rows cannot
// tell which of them were meant to stay, so all of them go. The mark goes only
// once that went through, so a boot that fails in between leaves it to the
// next one, and an error here has to stop the boot rather than serve them.
func ResetSignInsAfterConfigRestore(st *store.Repo, dataDir string, now time.Time) error {
	marker := selfrestore.AppliedMarkerPath(dataDir)
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		return nil
	}
	n, err := st.RevokeAllMCPKeys("config-restore", now.Unix())
	if err != nil {
		return fmt.Errorf("revoke MCP keys after the configuration restore: %w", err)
	}
	if n > 0 {
		log.Printf("selfrestore: revoked %d MCP key(s) and API token(s): a restored configuration may contain keys that were revoked after it was saved; create new ones under Settings, Integrations", n)
	}
	epoch, err := newSessionEpoch()
	if err != nil {
		return err
	}
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.SessionEpoch = epoch
		return nil
	}); err != nil {
		return fmt.Errorf("end the sessions after the configuration restore: %w", err)
	}
	removed, err := st.DeleteAllPasskeys()
	if err != nil {
		return fmt.Errorf("remove the passkeys after the configuration restore: %w", err)
	}
	log.Printf("selfrestore: ended every session and removed %d passkey(s): a restored configuration may bring back sessions and passkeys that were ended after it was saved; sign in with the password and add your passkeys again under Settings, Security", removed)
	return os.Remove(marker)
}
