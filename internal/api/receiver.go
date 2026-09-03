package api

// Receiver dashboard engine (read-only). A box that RECEIVES immutable off-site
// copies (an append-only rest-server / repo another BombVault pushes to) registers
// the received repo and monitors it READ-ONLY from the receiving hardware:
//
//   - receiverInventory: open read-only, list snapshots, group them by SOURCE
//     (hostname + the BombVault domain/item tag) and report per-source counts,
//     last-received time and size, plus repo totals.
//   - receiverCheck: run an INDEPENDENT `restic check` (optionally a deep
//     --read-data-subset check) on the receiving box.
//
// It reuses the foreign-repo read-only discipline (internal/api/foreign.go, #61):
// the repository is opened with the OTHER (sending) instance's APP_KEY via the
// RepoOpens (`restic cat config`) probe — NEVER EnsureRepo, which would INITIALIZE
// a missing repo — and every probe is lock-free (Mode.NoLock). Nothing in this
// engine writes to the received repo: no EnsureRepo, no backup, no prune, no
// forget. The sending APP_KEY is stored encrypted at rest and only decrypted here,
// in-engine; it is never logged.

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

// ReceiverSource is one backup SOURCE found in a received repository: a unique
// (hostname, BombVault item tag) pair, with its snapshot count, the time of the
// most recently received snapshot, and its size (the restore size of that newest
// snapshot). It answers "what is arriving from where, and is it still current".
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

// receiverOpen opens a received repository READ-ONLY and returns the RESOLVED repo
// location (a received repo may be any restic backend: rest://, s3:, rclone: — all
// passed through untouched — or a path under the host mount, which is resolved
// like every other repo path in this app) plus the read-only Mode. The stored
// SENDING APP_KEY is
// decrypted in-engine with THIS instance's APP_KEY, its shape is guarded (64 hex,
// the same foreignKeyRe the foreign flow uses), and the repo is probed with the
// key-derived encrypted mode first, then the plain (unencrypted) mode — every
// probe lock-free. EnsureRepo is deliberately NOT used: opening never initializes
// a missing repo. Nothing is logged.
func (s *Service) receiverOpen(ctx context.Context, rr store.ReceivedRepo) (string, restic.Mode, error) {
	loc := strings.TrimSpace(rr.Repo)
	if loc == "" {
		return "", restic.Mode{}, errors.New("missing repository location")
	}
	// Resolve the location the way EVERY other repo field in this app resolves
	// one ([554]). This was the single repo location taken verbatim, and the
	// field's own hint has always ended "…or a subpath under the host mount" —
	// a promise nothing here kept.
	//
	// What that cost: BombVault runs in a container, so a host path like
	// /mnt/user/backups does not exist inside it. restic could not open it, and
	// the only message the user got was this function's fallback — "wrong
	// APP_KEY, or the location is not a BombVault/restic repository". The path
	// was the problem and the message blamed the key. Reported from a working
	// setup where the key was fine.
	//
	// resolveRepo passes rest:/s3:/rclone: through untouched and sends anything
	// else through paths.Resolve, whose errors repoPathError turns into the
	// message that names the host root AND suggests the relative path to type
	// instead — which is exactly the help this report asked for.
	//
	// An already-absolute path INSIDE the host mount is accepted as-is: repos
	// added before this change may have been stored that way, and rejecting
	// them would break a working configuration to fix a broken one.
	// Declared, not initialised: both arms below assign it, and seeding it with
	// loc only looked like a safe default while hiding that one arm never used
	// the seed.
	var repo string
	// The "already absolute and inside the mount" test is a plain prefix check on
	// slash-normalised copies rather than paths.Within, which requires a leading
	// "/" and so can never be true on a Windows dev box. Only the COMPARISON is
	// normalised — restic still receives the location exactly as configured.
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
	// Guard the key shape BEFORE any use — restickey.Derive panics on non-hex input
	// by design (reuse the foreign flow's 64-lowercase-hex regexp).
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
		// Deliberately "BombVault or restic", not "BombVault/restic" ([584]): the
		// scrubber that runs over every error on its way out redacts absolute paths
		// with absPathRe, which cannot tell a filesystem path from a slash inside a
		// word. It turned this sentence into "not a BombVault[path] repository" for
		// every user who ever saw it, which is how it came back from the forum. The
		// redaction is right; the slash was ours to remove.
		return "", restic.Mode{}, errors.New("could not open the received repository: wrong APP_KEY, or the location is not a BombVault or restic repository")
	}
}

// receiverInventory opens the received repo read-only, lists its snapshots ONCE,
// and groups them by SOURCE (hostname + BombVault item tag). Per source it reports
// the snapshot count, the newest snapshot's time (lastReceived) and its restore
// size; plus repo totals (snapshot count, newest time, physical repo size). Only
// the received repo is READ — no write ever happens here.
func (s *Service) receiverInventory(ctx context.Context, rr store.ReceivedRepo) (ReceiverInventory, error) {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return ReceiverInventory{}, err
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return ReceiverInventory{}, err
	}

	// Group by (host, item). The item is the first recognized BombVault tag on the
	// snapshot; snapshots with no recognized tag fall under an "untagged" item so
	// they are still surfaced rather than silently dropped.
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
		// Restore size of the newest snapshot for this source — best-effort: a stats
		// failure must not sink the whole inventory (structure/metadata may still be
		// fine), so a failed size read leaves the source at size 0.
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
	// Physical (deduplicated + compressed) repo size — best-effort, same reasoning
	// as the per-source size.
	if st, sErr := s.engine.Stats(ctx, repo, "raw-data", mode); sErr == nil {
		inv.TotalSize = st.TotalSize
	}
	return inv, nil
}

// receiverCheck runs an INDEPENDENT integrity check of the received repo on the
// receiving hardware: a structural `restic check`, plus a deep
// `restic check --read-data-subset=<pct>%` when readData is true and the repo's
// configured percent is > 0. It is strictly read-only (no EnsureRepo/backup/prune/
// forget) and returns a typed result rather than a bare error so a failed check is
// a recorded verdict, not an exception. The returned Error is scrubbed; the
// sending key is never logged.
// claimReceiverCheck takes the in-flight slot for one received repo, reporting
// false when a check is already running on it. See the receiverCheckMu field for
// why this exists at all: the scheduled gate reads a timestamp its own work only
// writes at the end, and the manual endpoint had no gate whatsoever.
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

// errReceiverCheckBusy is what a caller is told when one is already running.
// A refusal, deliberately, rather than a queue: the second `restic check` would
// re-read the pack data the first one is reading, so waiting for a turn buys
// nothing and doubles the load in the meantime.
var errReceiverCheckBusy = errors.New("a check is already running on this repository")

// receiverCheckExclusive is receiverCheck behind the in-flight slot. ok=false
// means someone else holds it and NOTHING was run.
//
// The guard lives here rather than inside receiverCheck on purpose. A busy
// refusal must never travel as a ReceiverCheckResult: both call sites persist
// whatever they get via UpdateReceivedRepoCheckResult, so a result carrying
// "a check is already running" would be written down as a FAILED integrity
// check, and the scheduled path would then fire an integrity alert off it. A
// concurrency refusal is not a verdict about the repository.
func (s *Service) receiverCheckExclusive(ctx context.Context, rr store.ReceivedRepo, readData bool) (ReceiverCheckResult, bool) {
	if !s.claimReceiverCheck(rr.ID) {
		return ReceiverCheckResult{}, false
	}
	defer s.releaseReceiverCheck(rr.ID)
	return s.receiverCheck(ctx, rr, readData), true
}

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

// receiverItemTag returns the BombVault item tag a snapshot belongs to — the first
// recognized tag: a parameterized container:/vm:/fileset: tag, or the exact flash/
// config domain tags. Snapshots with no recognized tag return "untagged" so they
// are still grouped and surfaced.
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
// sorts oldest — so a malformed timestamp never wins "newest".
func parseSnapshotTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
