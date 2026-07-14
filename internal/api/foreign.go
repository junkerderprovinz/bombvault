package api

// Foreign-repo read sessions (#61): open ANOTHER BombVault instance's restic
// repository READ-ONLY with the OTHER instance's APP_KEY, inventory what it
// contains, and later (Task 10) restore single items from it. Two hard
// guarantees distinguish this from the Recovery "attach" flow:
//
//  1. Nothing is persisted. A session lives in memory only (Service.
//     foreignSessions) with a 30-minute TTL — the foreign location and key are
//     NEVER written to Settings (the attach flow's putSettings/UpdateSettings
//     path is deliberately not used here).
//  2. The foreign repo is never written to. The open probe is RepoOpens
//     (`restic cat config`) — NOT EnsureRepo, which would INITIALIZE a missing
//     repo, i.e. create an empty repository on someone else's storage.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
)

// ForeignItem is one restorable item (container, VM or file set) found in a
// foreign repository, with all of its snapshots (oldest-first, as restic
// reports them).
type ForeignItem struct {
	Name      string            `json:"name"`
	Snapshots []restic.Snapshot `json:"snapshots"`
}

// ForeignInventory groups a foreign repository's snapshots by the same tag
// prefixes Discover cuts (container:/vm:/fileset:), so the Recovery UI can
// offer a browse-and-restore tree without any local state.
type ForeignInventory struct {
	Containers []ForeignItem `json:"containers"`
	VMs        []ForeignItem `json:"vms"`
	FileSets   []ForeignItem `json:"fileSets"`
}

// foreignSession is one open read-only session onto a foreign repository. It
// carries everything a later restore needs (resolved repo, detected mode, and
// the FOREIGN APP_KEY for def decryption) so nothing has to be re-entered or —
// crucially — persisted. In memory only; expired sessions are swept on access.
type foreignSession struct {
	id      string
	repo    string // resolved repo location (paths.Resolve'd local path or remote URL verbatim)
	key     string // the OTHER instance's APP_KEY (64 hex) — decrypts foreign defs, never ours
	mode    restic.Mode
	expires time.Time
}

// foreignSessionTTL is how long an open foreign session stays usable. Long
// enough to browse and run several restores, short enough that the foreign key
// does not linger in memory indefinitely.
const foreignSessionTTL = 30 * time.Minute

// foreignKeyRe validates the foreign APP_KEY shape: exactly 64 lowercase hex
// characters (the same shape config validates for our own APP_KEY; that regexp
// is unexported, so the shape is mirrored here). Validated BEFORE any use —
// restickey.Derive panics on non-hex input by design.
var foreignKeyRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// errForeignSession is returned for an unknown or expired session id — the UI
// answer is the same either way: connect again.
var errForeignSession = errors.New("foreign session expired or unknown — connect to the repository again")

// newForeignSessionID returns a URL-safe 24-character random session id
// (18 bytes of crypto/rand, base64url — same recipe as randomDeployPassword).
func newForeignSessionID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// OpenForeign opens a foreign BombVault repository read-only and returns a new
// session id plus the full inventory. location is either a relative subpath
// under the host mount root (a mounted share) or a restic remote URL used
// verbatim; foreignKey is the OTHER instance's APP_KEY. Remote backends reuse
// the LOCAL instance's already-stored cloud credentials (read-only settings
// access) — no new credentials are persisted, and Settings is never written.
//
// Mode detection is a pure read: probe RepoOpens with the key-derived
// encrypted mode first, then the plain (unencrypted) mode. EnsureRepo is
// deliberately NOT used — it would initialize a missing repo, i.e. write into
// the foreign location.
func (s *Service) OpenForeign(ctx context.Context, location, foreignKey string) (string, ForeignInventory, error) {
	if strings.TrimSpace(location) == "" {
		return "", ForeignInventory{}, errors.New("missing repository location")
	}
	if !foreignKeyRe.MatchString(foreignKey) {
		return "", ForeignInventory{}, errors.New("the APP_KEY must be exactly 64 lowercase hex characters")
	}
	repo, err := s.resolveRepo(location)
	if err != nil {
		return "", ForeignInventory{}, err
	}
	settings, err := s.store.GetSettings() // READ-only: cloud creds for remote backends; never written back
	if err != nil {
		return "", ForeignInventory{}, fmt.Errorf("read settings: %w", err)
	}
	cloudEnv := s.cloudEnvFor(settings)

	encMode := restic.Mode{Encrypted: true, Password: restickey.Derive(foreignKey), Env: cloudEnv}
	plainMode := restic.Mode{Env: cloudEnv}
	var mode restic.Mode
	switch {
	case s.engine.RepoOpens(ctx, repo, encMode):
		mode = encMode
	case s.engine.RepoOpens(ctx, repo, plainMode):
		mode = plainMode
	default:
		return "", ForeignInventory{}, errors.New("could not open the repository — wrong APP_KEY, or the location is not a BombVault/restic repository")
	}

	inv, err := s.foreignInventory(ctx, repo, mode)
	if err != nil {
		return "", ForeignInventory{}, err
	}

	id, err := newForeignSessionID()
	if err != nil {
		return "", ForeignInventory{}, err
	}
	now := time.Now()
	s.foreignMu.Lock()
	if s.foreignSessions == nil {
		s.foreignSessions = map[string]foreignSession{}
	}
	s.sweepForeignLocked(now)
	s.foreignSessions[id] = foreignSession{
		id:      id,
		repo:    repo,
		key:     foreignKey,
		mode:    mode,
		expires: now.Add(foreignSessionTTL),
	}
	s.foreignMu.Unlock()
	return id, inv, nil
}

// CloseForeign drops a session immediately (the UI calls it on leave/unmount).
// Closing an unknown or already-expired id is a harmless no-op.
func (s *Service) CloseForeign(id string) {
	s.foreignMu.Lock()
	delete(s.foreignSessions, id)
	s.foreignMu.Unlock()
}

// foreignSession returns the live session for id, sweeping expired sessions
// first so an expired id is indistinguishable from an unknown one.
func (s *Service) foreignSession(id string) (foreignSession, error) {
	s.foreignMu.Lock()
	defer s.foreignMu.Unlock()
	s.sweepForeignLocked(time.Now())
	sess, ok := s.foreignSessions[id]
	if !ok {
		return foreignSession{}, errForeignSession
	}
	return sess, nil
}

// sweepForeignLocked removes expired sessions. Caller holds foreignMu.
func (s *Service) sweepForeignLocked(now time.Time) {
	for id, sess := range s.foreignSessions {
		if now.After(sess.expires) {
			delete(s.foreignSessions, id)
		}
	}
}

// foreignInventory lists the repo ONCE and groups the snapshots by the
// container:/vm:/fileset: tag prefixes (the same prefixes Discover cuts).
// Items are sorted by name; slices are non-nil so the JSON is always [].
func (s *Service) foreignInventory(ctx context.Context, repo string, mode restic.Mode) (ForeignInventory, error) {
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return ForeignInventory{}, err
	}
	containers := map[string][]restic.Snapshot{}
	vms := map[string][]restic.Snapshot{}
	fileSets := map[string][]restic.Snapshot{}
	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			if rest, ok := strings.CutPrefix(tag, "container:"); ok && rest != "" {
				containers[rest] = append(containers[rest], snap)
			}
			if rest, ok := strings.CutPrefix(tag, "vm:"); ok && rest != "" {
				vms[rest] = append(vms[rest], snap)
			}
			if rest, ok := strings.CutPrefix(tag, "fileset:"); ok && rest != "" {
				fileSets[rest] = append(fileSets[rest], snap)
			}
		}
	}
	return ForeignInventory{
		Containers: foreignItems(containers),
		VMs:        foreignItems(vms),
		FileSets:   foreignItems(fileSets),
	}, nil
}

// foreignItems flattens a name→snapshots map into a name-sorted item list.
func foreignItems(m map[string][]restic.Snapshot) []ForeignItem {
	out := make([]ForeignItem, 0, len(m))
	for name, snaps := range m {
		out = append(out, ForeignItem{Name: name, Snapshots: snaps})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
