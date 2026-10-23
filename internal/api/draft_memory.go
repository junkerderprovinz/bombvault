package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
)

// draftMemory keeps what rclone wrote into a draft while the wizard reached
// it, under the fields as the form sends them. The form sends them as typed
// on every call, and some of what rclone learned cannot be had twice: a login
// token is traded once, a one-time code opens one session, and Box voids the
// refresh token a refresh used.
type draftMemory struct {
	mu   sync.Mutex
	seen map[string]rememberedDraft
}

type rememberedDraft struct {
	learned map[string]string
	at      time.Time
}

const (
	// draftMemoryKeep is how long a wizard may sit between two of its calls.
	draftMemoryKeep = 6 * time.Hour
	// draftMemorySize bounds the drafts kept at once.
	draftMemorySize = 32
)

// setupAnswer starts a field that answers a question of rclone's own setup
// instead of setting the remote.
const setupAnswer = "config_"

// draftKey names a draft by its provider, the saved destination it reaches
// (empty in the wizard) and the fields the form sent.
func draftKey(provider, destination string, settings map[string]string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%q %q", provider, destination)
	for _, k := range slices.Sorted(maps.Keys(settings)) {
		_, _ = fmt.Fprintf(h, "\x00%q=%q", k, settings[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// recall puts what was learned under key into d.
func (m *draftMemory) recall(key string, d *remotes.Draft) {
	m.mu.Lock()
	defer m.mu.Unlock()
	got, ok := m.seen[key]
	if !ok || time.Since(got.at) > draftMemoryKeep {
		return
	}
	for k, v := range got.learned {
		d.Settings[k] = v
		if d.Learned == nil {
			d.Learned = map[string]string{}
		}
		d.Learned[k] = v
	}
}

// keep remembers what d learned under key. An empty key keeps nothing.
func (m *draftMemory) keep(key string, d remotes.Draft) {
	if key == "" || len(d.Learned) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.seen == nil {
		m.seen = map[string]rememberedDraft{}
	}
	for k, got := range m.seen {
		if time.Since(got.at) > draftMemoryKeep {
			delete(m.seen, k)
		}
	}
	for len(m.seen) >= draftMemorySize {
		oldest := ""
		for k, got := range m.seen {
			if oldest == "" || got.at.Before(m.seen[oldest].at) {
				oldest = k
			}
		}
		delete(m.seen, oldest)
	}
	m.seen[key] = rememberedDraft{learned: maps.Clone(d.Learned), at: time.Now()}
}

// setupAnswers are the fields that answer questions of rclone's setup.
func setupAnswers(settings map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range settings {
		if strings.HasPrefix(k, setupAnswer) {
			out[k] = strings.TrimSpace(v)
		}
	}
	return out
}

// oneTimeCode is the field Mega and Proton Drive take a two-factor code in,
// and sessionKey the setting that holds the session it opened. Seafile has a
// field of the same name that only says two-factor sign-in is on.
const oneTimeCode = "2fa"

var sessionKey = map[string]string{"mega": "session_id", "protondrive": "client_uid"}

// finishDraft readies a draft the wizard checked to be saved. One whose
// sign-in would not outlast its first run is refused.
func finishDraft(d *remotes.Draft) error {
	if remotes.SetupDue(*d) {
		return errors.New("test the connection first: the sign-in of this service is not set up yet")
	}
	dropUsedCode(d)
	if _, coded := sessionKey[d.Backend]; coded && d.Settings[oneTimeCode] != "" {
		return errors.New("a two-factor code works only once: enter a fresh one and test the connection again before saving")
	}
	// A prune deletes the packs it frees, and in a OneDrive recycle bin they
	// would still count against the quota. Personal drives cannot delete
	// outright.
	if d.Backend == "onedrive" && d.Settings["hard_delete"] == "" &&
		(d.Settings["drive_type"] == "business" || d.Settings["drive_type"] == "documentLibrary") {
		d.Settings["hard_delete"] = "true"
	}
	return nil
}

// dropUsedCode leaves out a one-time code once the session it opened is kept:
// the code works once, and a later run that sent it again would be refused.
func dropUsedCode(d *remotes.Draft) {
	key, coded := sessionKey[d.Backend]
	if !coded || d.Settings[oneTimeCode] == "" || d.Settings[key] == "" {
		return
	}
	d.Settings[oneTimeCode] = ""
	if d.Learned == nil {
		d.Learned = map[string]string{}
	}
	d.Learned[oneTimeCode] = ""
}
