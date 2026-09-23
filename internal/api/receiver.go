package api

// A box that receives off-site copies from another BombVault registers the repo
// and monitors it read-only: receiverInventory groups the snapshots by source,
// and receiverCheck runs an independent restic check on the receiving hardware.
//
// As in foreign.go, the repo is opened with the sending instance's APP_KEY
// through RepoOpens, never EnsureRepo, which would initialize a missing repo,
// and every probe sets NoLock. Nothing here writes to the received repo. The
// sending key is stored encrypted, decrypted only here and never logged.

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ReceiverSource is one backup source found in a received repository: a unique
// (hostname, BombVault item tag) pair.
type ReceiverSource struct {
	Host          string `json:"host"`
	Item          string `json:"item"` // the BombVault item tag, e.g. "container:web", "vm:db", "flash"
	SnapshotCount int    `json:"snapshotCount"`
	LastReceived  string `json:"lastReceived"` // newest snapshot time (RFC3339, as restic reports it); "" if none
	TotalSize     int64  `json:"totalSize"`    // restore size of the newest snapshot (bytes), best-effort
}

// ReceiverInventory is the read-only snapshot picture of a received repository:
// every source (grouped), plus repo-wide totals. Slices are non-nil so the JSON
// is always an array.
type ReceiverInventory struct {
	Sources       []ReceiverSource `json:"sources"`
	SnapshotCount int              `json:"snapshotCount"` // total snapshots across all sources
	LastReceived  string           `json:"lastReceived"`  // newest snapshot time across the whole repo; "" if empty
	TotalSize     int64            `json:"totalSize"`     // physical (deduplicated) repo size, best-effort
}

// ReceiverCheckResult is the outcome of an independent integrity check run on the
// receiving hardware. Error is a scrubbed message ("" on success). RanReadData is
// true when a deep --read-data-subset check ran (not just the structural check).
type ReceiverCheckResult struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	RanReadData bool   `json:"ranReadData"`
	At          int64  `json:"at"` // Unix time the check finished
}

// receiverOpen opens a received repository read-only and returns the resolved
// location and the mode to read it with. It never initializes a missing repo
// and logs nothing.
func (s *Service) receiverOpen(ctx context.Context, rr store.ReceivedRepo) (string, restic.Mode, error) {
	loc := strings.TrimSpace(rr.Repo)
	if loc == "" {
		return "", restic.Mode{}, errors.New("missing repository location")
	}
	// resolveRepo passes remote backends through and resolves anything else, so
	// a host path such as /mnt/user/backups, which does not exist inside the
	// container, gets an error that suggests the relative path instead of one
	// that blames the key. An absolute path already inside the host mount is
	// accepted as is, because existing rows may have been stored that way.
	var repo string
	// A prefix check on slash-normalised copies rather than paths.Within, which
	// requires a leading "/" and never matches on Windows. restic still gets the
	// location as configured.
	mountRoot := path.Clean(filepath.ToSlash(s.cfg.HostMountRoot))
	inMount := mountRoot != "." && mountRoot != "/" &&
		strings.HasPrefix(path.Clean(filepath.ToSlash(loc)), mountRoot+"/")
	if !restic.IsRemoteRepo(loc) && inMount {
		repo = loc
	} else {
		resolved, err := s.resolveRepo(loc)
		if err != nil {
			return "", restic.Mode{}, err
		}
		repo = resolved
	}
	keyBytes, err := secret.Decrypt(s.cfg.AppKey, rr.AppKeyEnc)
	if err != nil {
		return "", restic.Mode{}, errors.New("could not decrypt the stored sending APP_KEY for this received repo")
	}
	sendingKey := string(keyBytes)
	// restickey.Derive panics on non-hex input, so check the shape first.
	if !foreignKeyRe.MatchString(sendingKey) {
		return "", restic.Mode{}, errors.New("the stored sending APP_KEY is not 64 lowercase hex characters")
	}
	// NoLock keeps the read-only probe from writing a lock file into the received
	// (append-only) repo. Try the encrypted mode a BombVault sender always uses,
	// then fall back to a plain repo.
	encMode := restic.Mode{Encrypted: true, Password: restickey.Derive(sendingKey), NoLock: true}
	plainMode := restic.Mode{NoLock: true}
	switch {
	case s.engine.RepoOpens(ctx, repo, encMode):
		return repo, encMode, nil
	case s.engine.RepoOpens(ctx, repo, plainMode):
		return repo, plainMode, nil
	default:
		// No slash in "BombVault or restic": scrubError would redact "/restic" as
		// a path.
		return "", restic.Mode{}, errors.New("could not open the received repository: wrong APP_KEY, or the location is not a BombVault or restic repository")
	}
}

// receiverInventory lists a received repo's snapshots once and groups them by
// source.
func (s *Service) receiverInventory(ctx context.Context, rr store.ReceivedRepo) (ReceiverInventory, error) {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return ReceiverInventory{}, err
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return ReceiverInventory{}, err
	}

	type agg struct {
		host, item string
		count      int
		newestID   string
		newest     time.Time
		newestRaw  string
	}
	groups := map[string]*agg{}
	var overallNewest time.Time
	var overallNewestRaw string
	for _, snap := range snaps {
		item := receiverItemTag(snap)
		key := snap.Hostname + "\x00" + item
		g := groups[key]
		if g == nil {
			g = &agg{host: snap.Hostname, item: item}
			groups[key] = g
		}
		g.count++
		when := parseSnapshotTime(snap.Time)
		if g.newestID == "" || when.After(g.newest) {
			g.newest = when
			g.newestID = snap.ID
			g.newestRaw = snap.Time
		}
		if overallNewestRaw == "" || when.After(overallNewest) {
			overallNewest = when
			overallNewestRaw = snap.Time
		}
	}

	sources := make([]ReceiverSource, 0, len(groups))
	for _, g := range groups {
		var size int64
		// A failed stats read leaves the size at 0 instead of failing the
		// inventory.
		if g.newestID != "" {
			if _, bytes, sErr := s.engine.StatsRestoreSize(ctx, repo, g.newestID, mode); sErr == nil {
				size = bytes
			}
		}
		sources = append(sources, ReceiverSource{
			Host:          g.host,
			Item:          g.item,
			SnapshotCount: g.count,
			LastReceived:  g.newestRaw,
			TotalSize:     size,
		})
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Host != sources[j].Host {
			return sources[i].Host < sources[j].Host
		}
		return sources[i].Item < sources[j].Item
	})

	inv := ReceiverInventory{
		Sources:       sources,
		SnapshotCount: len(snaps),
		LastReceived:  overallNewestRaw,
	}
	// Physical repo size after dedup and compression, best-effort as above.
	if st, sErr := s.engine.Stats(ctx, repo, "raw-data", mode); sErr == nil {
		inv.TotalSize = st.TotalSize
	}
	return inv, nil
}

// claimReceiverCheck takes the in-flight slot for one received repo and
// reports false when a check is already running on it (see receiverCheckMu).
func (s *Service) claimReceiverCheck(id string) bool {
	s.receiverCheckMu.Lock()
	defer s.receiverCheckMu.Unlock()
	if s.receiverChecking[id] {
		return false
	}
	if s.receiverChecking == nil {
		s.receiverChecking = map[string]bool{}
	}
	s.receiverChecking[id] = true
	return true
}

// releaseReceiverCheck frees the slot. Deferred by every claimant, so a panic
// inside a check cannot wedge that repo's checking for the life of the process.
func (s *Service) releaseReceiverCheck(id string) {
	s.receiverCheckMu.Lock()
	delete(s.receiverChecking, id)
	s.receiverCheckMu.Unlock()
}

// errReceiverCheckBusy refuses a second check rather than queueing it: it
// would re-read the pack data the first one is reading.
var errReceiverCheckBusy = errors.New("a check is already running on this repository")

// receiverCheckExclusive is receiverCheck behind the in-flight slot; ok is
// false when another check holds it and nothing ran. The guard sits outside
// receiverCheck because both callers persist whatever result they get, and a
// busy refusal recorded as a failed check would fire an integrity alert.
func (s *Service) receiverCheckExclusive(ctx context.Context, rr store.ReceivedRepo, readData bool) (ReceiverCheckResult, bool) {
	if !s.claimReceiverCheck(rr.ID) {
		return ReceiverCheckResult{}, false
	}
	defer s.releaseReceiverCheck(rr.ID)
	return s.receiverCheck(ctx, rr, readData), true
}

// receiverCheck runs an independent integrity check of the received repo on
// the receiving hardware: a structural restic check, or a deep
// --read-data-subset check when readData is set and the repo has a percent
// configured. A failed check comes back as a result rather than an error, so it
// is recorded as a verdict. Error is scrubbed.
func (s *Service) receiverCheck(ctx context.Context, rr store.ReceivedRepo, readData bool) ReceiverCheckResult {
	res := ReceiverCheckResult{At: time.Now().Unix()}
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		res.Error = scrubError(err)
		return res
	}
	deep := readData && rr.ReadDataPercent > 0
	if deep {
		err = s.engine.CheckData(ctx, repo, rr.ReadDataPercent, mode)
		res.RanReadData = true
	} else {
		err = s.engine.Check(ctx, repo, mode)
	}
	res.At = time.Now().Unix()
	if err != nil {
		res.Error = scrubError(err)
		return res
	}
	res.OK = true
	return res
}

// receiverItemTag returns the first recognized BombVault item tag of a
// snapshot: container:, vm: or fileset: with a name, or flash or config.
// Snapshots without one return "untagged" so they still show up.
func receiverItemTag(snap restic.Snapshot) string {
	for _, tag := range snap.Tags {
		switch {
		case strings.HasPrefix(tag, "container:") && len(tag) > len("container:"),
			strings.HasPrefix(tag, "vm:") && len(tag) > len("vm:"),
			strings.HasPrefix(tag, "fileset:") && len(tag) > len("fileset:"):
			return tag
		case tag == "flash", tag == "config":
			return tag
		}
	}
	return "untagged"
}

// parseSnapshotTime parses a restic snapshot Time string (RFC3339, usually with
// nanoseconds and a numeric zone). A parse failure yields the zero time, which
// sorts oldest, so a malformed timestamp never wins "newest".
func parseSnapshotTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
