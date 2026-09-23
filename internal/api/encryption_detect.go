package api

// Settings.EncryptionEnabled describes how the repositories were created: with
// the APP_KEY-derived password or with --insecure-no-password. That is fixed
// at init, and restic opens a repository only in its own mode, so opening it
// reveals the mode. This file probes every configured repository read-only
// with `restic cat config` and applies a definite verdict to the stored
// setting, so a user restoring onto a fresh instance does not have to know
// it. A probe failure is never read as "unencrypted": that answer would create
// a second, empty repository next to the real backups.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// RepoEncryptionState is one repository's detected encryption mode.
type RepoEncryptionState string

const (
	// RepoEncrypted means the repo opened with the APP_KEY-derived password.
	RepoEncrypted RepoEncryptionState = "encrypted"
	// RepoPlain means the repo opened with --insecure-no-password.
	RepoPlain RepoEncryptionState = "plain"
	// RepoAbsent means the location is reachable but holds no repository yet,
	// so the user's choice decides how it will be created.
	RepoAbsent RepoEncryptionState = "absent"
	// RepoUnreachable means the probe failed for another reason: a bad path, a
	// dead backend, wrong credentials, an unmounted share. The repo may exist
	// and be encrypted, so this is never folded in with RepoAbsent.
	RepoUnreachable RepoEncryptionState = "unreachable"
)

// RepoEncryption is the per-repository detection result. Err is set only for
// RepoUnreachable and is already scrubbed for the client.
type RepoEncryption struct {
	Domain string              `json:"domain"`
	Source string              `json:"source"` // "local" | "offsite"
	Name   string              `json:"name,omitempty"`
	State  RepoEncryptionState `json:"state"`
	Err    string              `json:"error,omitempty"`
}

// EncryptionVerdict is the fold of every probed repository into one answer.
type EncryptionVerdict string

const (
	// VerdictEncrypted and VerdictPlain are definite and get applied.
	VerdictEncrypted EncryptionVerdict = "encrypted"
	VerdictPlain     EncryptionVerdict = "plain"
	// VerdictConflict means some repositories opened encrypted and others
	// plain. One global flag cannot open both, so nothing is applied and the
	// user has to fix the odd one out.
	VerdictConflict EncryptionVerdict = "conflict"
	// VerdictAbsent means every configured location is reachable but empty.
	VerdictAbsent EncryptionVerdict = "absent"
	// VerdictUnknown means at least one repository could not be opened for a
	// reason other than not existing yet, and none gave a definite answer.
	VerdictUnknown EncryptionVerdict = "unknown"
	// VerdictUnconfigured means no repository location is configured.
	VerdictUnconfigured EncryptionVerdict = "unconfigured"
)

// EncryptionDetection is the whole result: the verdict, whether the stored
// setting was changed to match, the effective setting afterwards, and the
// per-repository detail the UI shows for the two undecided cases.
type EncryptionDetection struct {
	Verdict           EncryptionVerdict `json:"verdict"`
	Applied           bool              `json:"applied"`
	EncryptionEnabled bool              `json:"encryptionEnabled"`
	Repos             []RepoEncryption  `json:"repos"`
}

// encryptionDetectDomains lists the domains whose repositories are probed. A
// domain is probed whenever its location is set, even with its Enabled flag
// off: on a fresh recovery instance those flags have not been restored yet.
var encryptionDetectDomains = []string{"containers", "vms", "flash", "files", "config"}

// encryptionProbeTimeout bounds each probe attempt on its own. With one budget
// for the encrypted/plain pair, a cold sftp connection over a VPN can use it
// up on the first attempt and leave the second none, so a reachable repo would
// read as unreachable.
const encryptionProbeTimeout = 30 * time.Second

// encryptionProbeParallel caps concurrent probes, so several dead backends do
// not add up to minutes and not every repository forks restic at once.
const encryptionProbeParallel = 4

// encryptionDetectBudget bounds a whole detection pass. The pass is detached
// from the callers' requests, so without it a backend that never answers would
// hold the single-flight slot for the life of the process. The worst case per
// repository is 2 x encryptionProbeTimeout with encryptionProbeParallel running
// at once, so this covers 60 repositories; cutting a real pass short would
// report every repository as unreachable.
const encryptionDetectBudget = 15 * time.Minute

// DetectEncryption probes every configured repository, folds the results into
// a verdict and, if the verdict is definite, writes that mode into the stored
// settings. The probe is read-only (`restic cat config`); EnsureRepo would
// initialize a missing repository at the location the user is trying to attach.
//
// Concurrent callers share one pass. The Recovery page runs this on mount, and
// a second tab or a reload would otherwise start another set of restic
// processes against the same repositories.
func (s *Service) DetectEncryption(ctx context.Context) (EncryptionDetection, error) {
	s.detectMu.Lock()
	if flight := s.detectFlight; flight != nil {
		s.detectMu.Unlock()
		select {
		case <-flight.done:
			return flight.det, flight.err
		case <-ctx.Done():
			// This caller gave up; the leader's pass carries on for the others.
			return EncryptionDetection{}, ctx.Err()
		}
	}
	flight := &encryptionDetectFlight{done: make(chan struct{}), err: errEncryptionDetectAborted}
	s.detectFlight = flight
	s.detectMu.Unlock()

	// Clearing the slot in a defer keeps a panic from leaving the flight
	// installed and unclosed, which would park every later caller forever. The
	// pre-seeded err tells followers released that way that there is no result.
	defer func() {
		s.detectMu.Lock()
		s.detectFlight = nil
		s.detectMu.Unlock()
		close(flight.done)
	}()

	// The pass is shared, so no single caller may cancel it. On the leader's
	// request context a closed tab would cancel every probe, a cancelled probe
	// reads as unreachable, and the other callers would get "unknown". Each
	// caller still honours its own ctx while waiting on flight.done above.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), encryptionDetectBudget)
	defer cancel()

	flight.det, flight.err = s.detectEncryption(pctx)
	return flight.det, flight.err
}

// errEncryptionDetectAborted is returned to followers when the leading pass
// ended without a result.
var errEncryptionDetectAborted = errors.New("encryption detection did not complete")

// encryptionDetectFlight is one in-flight DetectEncryption pass. done is
// closed once det and err are final. err starts as errEncryptionDetectAborted,
// so a flight closed without a result reads as a failure.
type encryptionDetectFlight struct {
	done chan struct{}
	det  EncryptionDetection
	err  error
}

// detectEncryption runs one pass for DetectEncryption.
func (s *Service) detectEncryption(ctx context.Context) (EncryptionDetection, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return EncryptionDetection{}, fmt.Errorf("read settings: %w", err)
	}

	repos := s.encryptionProbeTargets(settings)
	results := s.probeEncryptionModes(ctx, repos)
	verdict := foldEncryption(results)

	det := EncryptionDetection{
		Verdict:           verdict,
		EncryptionEnabled: settings.EncryptionEnabled,
		Repos:             results,
	}

	// Only a definite verdict is applied. The settings snapshot above can be
	// minutes old by now (a dead off-site host costs 2 x encryptionProbeTimeout),
	// and writing it back would revert whatever the user saved meanwhile.
	// MutateSettings re-reads the row under its lock, changes only the
	// encryption flag and returns the row the caller reports.
	want, definite := verdict.encryptionEnabled()
	applied := false
	current, mErr := s.store.MutateSettings(func(cur *store.Settings) error {
		if !definite || cur.EncryptionEnabled == want {
			return nil
		}
		cur.EncryptionEnabled = want
		applied = true
		return nil
	})
	if mErr != nil {
		return det, fmt.Errorf("apply detected encryption mode: %w", mErr)
	}
	if applied {
		log.Printf("api: encryption mode auto-detected as %s, Settings.EncryptionEnabled set to %v", verdict, want)
	}
	det.Applied = applied
	det.EncryptionEnabled = current.EncryptionEnabled
	return det, nil
}

// encryptionEnabled maps a verdict to the setting it implies. definite is false
// for every verdict that must not touch the setting.
func (v EncryptionVerdict) encryptionEnabled() (enabled, definite bool) {
	switch v {
	case VerdictEncrypted:
		return true, true
	case VerdictPlain:
		return false, true
	default:
		return false, false
	}
}

// encryptionProbeTarget is one resolved repository to probe. A non-nil err is
// the resolution failure; such a target is unreachable without running restic.
type encryptionProbeTarget struct {
	domain string
	source string
	name   string
	repo   string
	mode   restic.Mode // encrypted mode; the plain probe is derived from it
	err    error
}

// encryptionProbeTargets lists every configured repository: each domain's
// local location plus its off-site destinations, resolved the way replication
// resolves them (target rows, else the legacy Settings column).
func (s *Service) encryptionProbeTargets(settings store.Settings) []encryptionProbeTarget {
	var out []encryptionProbeTarget
	base := s.ModeFor(settings)

	for _, domain := range encryptionDetectDomains {
		// An empty location is unconfigured, not a repository.
		if loc := localRepoLocation(settings, domain); strings.TrimSpace(loc) != "" {
			repo, rErr := s.resolveRepo(loc)
			out = append(out, encryptionProbeTarget{
				domain: domain, source: "local", repo: repo, mode: base, err: rErr,
			})
		}

		for _, target := range s.offsiteReplicationTargets(domain, settings) {
			if strings.TrimSpace(target.Repo) == "" {
				continue
			}
			repo, rErr := s.resolveRepo(target.Repo)
			out = append(out, encryptionProbeTarget{
				domain: domain, source: "offsite", name: target.Name, repo: repo,
				mode: s.offsiteModeForTarget(settings, target), err: rErr,
			})
		}
	}
	return out
}

// localRepoLocation returns a domain's local repo location from Settings,
// unresolved.
func localRepoLocation(settings store.Settings, domain string) string {
	switch domain {
	case "containers":
		return settings.ContainersPath
	case "vms":
		return settings.VMsPath
	case "flash":
		return settings.FlashPath
	case "files":
		return settings.FilesPath
	case "config":
		return settings.ConfigPath
	}
	return ""
}

// probeEncryptionModes probes the targets concurrently. Results keep the order
// of targets so the UI list is stable across runs.
func (s *Service) probeEncryptionModes(ctx context.Context, targets []encryptionProbeTarget) []RepoEncryption {
	results := make([]RepoEncryption, len(targets))
	sem := make(chan struct{}, encryptionProbeParallel)
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(i int, t encryptionProbeTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = s.probeOneEncryptionMode(ctx, t)
		}(i, t)
	}
	wg.Wait()
	return results
}

// probeOneEncryptionMode classifies one repository. It tries the encrypted
// mode first, then the plain one, as OpenForeign and the receiver's attach
// probe do.
func (s *Service) probeOneEncryptionMode(ctx context.Context, t encryptionProbeTarget) RepoEncryption {
	out := RepoEncryption{Domain: t.domain, Source: t.source, Name: t.name}

	// A location that did not resolve says nothing about encryption.
	if t.err != nil {
		out.State = RepoUnreachable
		out.Err = scrubError(t.err)
		return out
	}

	encMode := t.mode
	encMode.Encrypted = true
	encMode.Password = restickey.Derive(s.cfg.AppKey)
	plainMode := t.mode
	plainMode.Encrypted = false
	plainMode.Password = ""
	// Asking for the mode must not write a lock file. cat config takes none;
	// NoLock keeps it that way, as in the foreign-session probe.
	encMode.NoLock = true
	plainMode.NoLock = true

	probe := func(m restic.Mode) error {
		pctx, cancel := context.WithTimeout(ctx, encryptionProbeTimeout)
		defer cancel()
		return s.engine.RepoOpensErr(pctx, t.repo, m)
	}

	encErr := probe(encMode)
	if encErr == nil {
		out.State = RepoEncrypted
		return out
	}
	if plainErr := probe(plainMode); plainErr == nil {
		out.State = RepoPlain
		return out
	}

	// Neither mode opened it. "No repository yet" and "could not reach it" look
	// alike in restic's output but mean opposite things here.
	out.State, out.Err = s.classifyClosedRepo(t.repo, encErr)
	return out
}

// classifyClosedRepo decides whether a repository that opened under neither
// mode is absent or unreachable.
//
// Remote backends go through isRepoDefinitelyAbsent. A local path follows
// EnsureRepo: a missing `config` at a location BombVault established earlier,
// whose backing store is not in the mount table, is a vanished mount and so a
// real repository that cannot be seen right now. Any other missing `config` is
// a fresh location.
func (s *Service) classifyClosedRepo(repo string, probeErr error) (RepoEncryptionState, string) {
	if restic.IsRemoteRepo(repo) {
		if isRepoDefinitelyAbsent(probeErr) {
			return RepoAbsent, ""
		}
		return RepoUnreachable, scrubError(probeErr)
	}
	if !localRepoMissing(repo) {
		// A `config` file exists but neither mode opened it: a foreign or
		// corrupt repository, or a permissions problem.
		return RepoUnreachable, scrubError(probeErr)
	}
	if s.repoEstablished(repo) && !s.destinationMounted(repo) {
		return RepoUnreachable, scrubError(ErrBackupPathNotMounted)
	}
	return RepoAbsent, ""
}

// repoAbsenceMarkers are the phrasings with which a backend says the object
// restic asked for does not exist. It is an allow-list, so any message not
// listed here, such as a 502 from a proxy in front of a dead rest-server,
// resolves to the safe "unreachable" rather than to "absent". The entries come
// from what the backends print:
//
//	rest      Fatal: unable to open config file: <config/> does not exist
//	restic    Fatal: repository does not exist: unable to open config file
//	s3/minio  Stat: The specified key does not exist. / NoSuchKey
//	b2        b2_download_file_by_name: 404: … does not exist
//	azure     BlobNotFound
//	gcs       storage: object doesn't exist
//	sftp/swift  file does not exist / Object Not Found / no such file or directory
//
// A plain "404 Not Found" is not listed: a reverse proxy returns it for a
// mis-routed path in front of a repository that exists.
var repoAbsenceMarkers = []string{
	"does not exist", "doesn't exist", "no such file or directory",
	"file not found", "object not found", "key not found",
	"nosuchkey", "nosuchbucket", "blobnotfound",
}

// transportFailureMarkers name failures to reach the backend at all. For a host
// with nothing listening restic prints
//
//	Fatal: unable to open config file: Head "http://…/config":
//	dial tcp 192.168.20.199:8000: connect: no route to host
//	Is there a repository at the following location?
//
// These markers veto the allow-list above rather than decide on their own:
// "dial tcp: lookup backup.example: no such host" reads like a missing object,
// and a layered backend can print "open /mnt/x/config: permission denied: no
// such file or directory". A message that names a transport failure is never
// evidence of an empty location.
var transportFailureMarkers = []string{
	"dial tcp", "no route to host", "connection refused", "connection reset",
	"network is unreachable", "no such host", "i/o timeout", "timeout",
	"context deadline exceeded", "temporary failure in name resolution",
	"server misbehaving", "broken pipe", "unexpected eof",
	"tls", "certificate", "x509",
	"401", "403", "unauthorized", "forbidden", "access denied",
	"permission denied", "operation not permitted",
}

// isRepoDefinitelyAbsent reports whether err means the location is reachable
// and holds no repository. It is stricter than isRepoUninitialized: a transport
// failure named anywhere in the message vetoes even restic's own "repository
// does not exist". Anything ambiguous resolves to unreachable, because wrongly
// answering "absent" can create an empty repository next to real backups, while
// wrongly answering "unreachable" only costs the user one "can't tell" line.
func isRepoDefinitelyAbsent(err error) bool {
	return isRepoUninitialized(err) && !containsAny(strings.ToLower(err.Error()), transportFailureMarkers)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// foldEncryption folds the per-repository states into one verdict. The order
// of the checks matters:
//
//   - encrypted and plain both present: conflict.
//   - any definite detection: that mode. Absent or unreachable repositories
//     alongside it do not weaken it; a real mismatch among them still
//     surfaces from EnsureRepo on the next backup.
//   - anything unreachable: unknown. This comes before absent because the
//     unreachable repository may be the encrypted one the user wants back.
//   - everything reachable and empty: absent, a first-time setup.
func foldEncryption(repos []RepoEncryption) EncryptionVerdict {
	if len(repos) == 0 {
		return VerdictUnconfigured
	}
	var enc, plain, absent, unreachable int
	for _, r := range repos {
		switch r.State {
		case RepoEncrypted:
			enc++
		case RepoPlain:
			plain++
		case RepoAbsent:
			absent++
		case RepoUnreachable:
			unreachable++
		}
	}
	switch {
	case enc > 0 && plain > 0:
		return VerdictConflict
	case enc > 0:
		return VerdictEncrypted
	case plain > 0:
		return VerdictPlain
	case unreachable > 0:
		return VerdictUnknown
	case absent > 0:
		return VerdictAbsent
	default:
		return VerdictUnconfigured
	}
}
