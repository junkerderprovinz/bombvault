package api

// Encryption-mode auto-detection.
//
// Settings.EncryptionEnabled is NOT a preference — it is a FACT about the
// repositories. ModeFor turns it into exactly one thing: restic runs with a
// password derived from APP_KEY (true) or with --insecure-no-password (false).
// A repository was created one way or the other at init time and can never
// change afterwards; setting the flag the other way just makes restic fail to
// open the repo (EnsureRepo already reports that mismatch, see issue #14).
//
// Because it is a fact, it is DETECTABLE: opening the repository reveals which
// mode it is in. The two modes are mutually exclusive by construction — restic
// cannot open a password-less repo with a password, nor a keyed repo without
// one — so a repo that opens under one mode definitively is that mode. This
// file probes the configured repositories with the SAME read-only
// `restic cat config` probe EnsureRepo/OpenForeign/probeOffsiteRepo already
// use (ResticEngine.RepoOpensErr), classifies the outcome per repository, and
// folds the results into one verdict.
//
// The whole point is that a user restoring onto a fresh instance must not have
// to KNOW or GUESS the mode. So a definite verdict is APPLIED to the stored
// setting. Everything else is reported honestly as undecided — a probe failure
// is never silently read as "unencrypted", which would be the one wrong answer
// that quietly creates a second, empty repository next to the real backups.

import (
	"context"
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
	// RepoEncrypted: the repo opened with the APP_KEY-derived password.
	RepoEncrypted RepoEncryptionState = "encrypted"
	// RepoPlain: the repo opened with --insecure-no-password.
	RepoPlain RepoEncryptionState = "plain"
	// RepoAbsent: the location is reachable but holds no repository yet, so
	// there is genuinely nothing to detect — a first-time setup, where the
	// user's own choice really does decide how the repo will be created.
	RepoAbsent RepoEncryptionState = "absent"
	// RepoUnreachable: the probe failed for a reason that is NOT "no repo here"
	// — a bad path, a dead backend, wrong backend credentials, an unmounted
	// share. The repo may well exist and be encrypted; we simply cannot tell.
	// This state must never be folded in with RepoAbsent (see foldEncryption).
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
	// VerdictEncrypted / VerdictPlain: detected, unambiguous, APPLIED.
	VerdictEncrypted EncryptionVerdict = "encrypted"
	VerdictPlain     EncryptionVerdict = "plain"
	// VerdictConflict: repositories genuinely disagree — some opened encrypted,
	// some opened plain. One global flag cannot open both, so there is no
	// correct value to apply. Never applied; the user must fix the odd one out.
	VerdictConflict EncryptionVerdict = "conflict"
	// VerdictAbsent: every configured location is reachable but empty. Nothing
	// to detect; the user's choice decides how the repos get created.
	VerdictAbsent EncryptionVerdict = "absent"
	// VerdictUnknown: at least one repository could not be opened for a reason
	// other than "not created yet", and nothing else gave a definite answer.
	// The honest "cannot tell". Never applied.
	VerdictUnknown EncryptionVerdict = "unknown"
	// VerdictUnconfigured: no repository location is configured at all.
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

// encryptionDetectDomains is the set of domains whose repositories carry a
// mode. Every domain with a NON-EMPTY configured location is probed, including
// domains whose *Enabled flag is off: on a fresh recovery instance those flags
// have not been restored yet, so gating on them would skip exactly the repos
// the user is trying to attach to. A configured location is the signal here,
// not the enabled flag.
var encryptionDetectDomains = []string{"containers", "vms", "flash", "files", "config"}

// encryptionProbeTimeout bounds ONE probe attempt. Per attempt, not shared
// across the encrypted/plain pair: a cold sftp connection over a VPN can eat a
// shared budget on the first try and leave the second attempt zero time, which
// would report a reachable repo as unreachable (the #93 mistake, see
// probeOffsiteRepo's own comment).
const encryptionProbeTimeout = 30 * time.Second

// encryptionProbeParallel caps how many repositories are probed at once, so a
// box with several domains and several off-site destinations does not serialize
// into minutes of wall clock when everything is dead, and does not fork a
// restic process per repo all at once either.
const encryptionProbeParallel = 4

// DetectEncryption probes every configured repository, folds the results into a
// verdict, and — for a DEFINITE verdict only — writes that mode into the stored
// settings so the user never has to assert it. It returns the detection either
// way. The probe itself is strictly read-only: `restic cat config`, never
// EnsureRepo (which would INITIALIZE a missing repo, i.e. write an empty
// repository over the location the user is still trying to attach to).
//
// Two callers arriving at once share ONE probe pass (encryptionDetectFlight):
// the Recovery page fires this on mount, so a second tab — or a reload while a
// dead off-site host is still burning its 2x30s probe budget — would otherwise
// fork a second set of restic processes against the same repositories for an
// answer the first pass is already computing.
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
	flight := &encryptionDetectFlight{done: make(chan struct{})}
	s.detectFlight = flight
	s.detectMu.Unlock()

	flight.det, flight.err = s.detectEncryption(ctx)

	s.detectMu.Lock()
	s.detectFlight = nil
	s.detectMu.Unlock()
	close(flight.done)
	return flight.det, flight.err
}

// encryptionDetectFlight is one in-flight DetectEncryption pass, shared with
// every caller that arrives while it runs. done is closed once det/err are
// final, so a follower reads them only after they are written.
type encryptionDetectFlight struct {
	done chan struct{}
	det  EncryptionDetection
	err  error
}

// detectEncryption is the pass itself (see DetectEncryption for the contract).
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

	// Apply ONLY a definite verdict. conflict/absent/unknown/unconfigured all
	// leave the stored setting exactly as it was: there is no detected fact to
	// follow, and guessing is the failure mode this whole feature exists to
	// remove.
	//
	// The `settings` snapshot above is MINUTES old by now — probing a dead
	// off-site host burns 2x encryptionProbeTimeout on it — so it must never be
	// written back: doing that reverts every unrelated column the user saved
	// meanwhile (paths, schedules, the login password) while the UI reports
	// those saves as successful. MutateSettings re-reads the row under its own
	// lock and applies ONLY the one fact this pass established, so a detection
	// can change nothing it did not determine. It also returns the row as it now
	// stands, which is what the caller is told the effective setting is — the
	// stale snapshot cannot answer that either.
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
		log.Printf("api: encryption mode auto-detected as %s — Settings.EncryptionEnabled set to %v", verdict, want)
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

// encryptionProbeTarget is one repository to probe, already resolved (or
// carrying the resolution failure that made it unreachable before any restic
// call).
type encryptionProbeTarget struct {
	domain string
	source string
	name   string
	repo   string
	mode   restic.Mode // the ENCRYPTED-mode probe; the plain probe is derived
	err    error       // resolution failure — probed as unreachable, no restic run
}

// encryptionProbeTargets enumerates every configured repository: each domain's
// local location, plus each of its off-site destinations (the real target rows
// when present, otherwise the legacy Settings column, exactly as replication
// resolves them — so what is probed is what actually gets written to).
func (s *Service) encryptionProbeTargets(settings store.Settings) []encryptionProbeTarget {
	var out []encryptionProbeTarget
	base := s.ModeFor(settings)

	for _, domain := range encryptionDetectDomains {
		// Local repo for the domain. An empty location means "not configured",
		// which is not a repository and must not count as one.
		if loc := localRepoLocation(settings, domain); strings.TrimSpace(loc) != "" {
			repo, rErr := s.resolveRepo(loc)
			out = append(out, encryptionProbeTarget{
				domain: domain, source: "local", repo: repo, mode: base, err: rErr,
			})
		}

		// Off-site destinations for the domain.
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

// localRepoLocation returns a domain's configured LOCAL repo location straight
// off Settings, without resolving it — the counterpart of
// offsiteRepoFromSettings for the local side.
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

// probeEncryptionModes probes every target concurrently (capped) and returns
// the per-repository results in the SAME order as targets, so the UI's list is
// stable across runs rather than ordered by whichever probe finished first.
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

// probeOneEncryptionMode classifies ONE repository. It tries the encrypted mode
// first, then the plain one (the same order OpenForeign and the receiver's
// attach probe already use), and classifies a double failure honestly.
func (s *Service) probeOneEncryptionMode(ctx context.Context, t encryptionProbeTarget) RepoEncryption {
	out := RepoEncryption{Domain: t.domain, Source: t.source, Name: t.name}

	// The location never resolved (containment rejection, malformed remote).
	// That is a configuration failure, not evidence about encryption.
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
	// Never write a lock file into a repository just to ask what mode it is in.
	// CatConfigArgs is lock-free already; NoLock keeps that explicit and matches
	// the read-only foreign-session probe.
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

	// Neither mode opened it. Now the honest part: "no repository here yet" and
	// "there might be a repository here but I could not reach it" look similar
	// in restic's output and mean opposite things for this feature.
	out.State, out.Err = s.classifyClosedRepo(t.repo, encErr)
	return out
}

// classifyClosedRepo decides whether a repository that opened under NEITHER
// mode is genuinely absent (nothing to detect) or unreachable (cannot tell).
//
// A REMOTE backend that answers restic's "repository does not exist" is the
// established "reachable, just not created yet" signal (#117/#130) — the same
// one listSnapshots and probeOffsiteRepo already treat as non-fatal. Detection
// needs a STRICTER reading of it than those callers do, see
// isRepoDefinitelyAbsent.
//
// A LOCAL path needs more care, and reuses EnsureRepo's own three-way
// distinction (#55/#120): a missing `config` at a location BombVault previously
// established, whose backing store is NOT in the kernel mount table, is a
// vanished mount — very much a real repository we cannot see right now, so
// reporting it as "absent" would be the exact wrong guess. Everything else that
// is simply missing its `config` is a genuine fresh location.
func (s *Service) classifyClosedRepo(repo string, probeErr error) (RepoEncryptionState, string) {
	if restic.IsRemoteRepo(repo) {
		if isRepoDefinitelyAbsent(probeErr) {
			return RepoAbsent, ""
		}
		return RepoUnreachable, scrubError(probeErr)
	}
	if !localRepoMissing(repo) {
		// A `config` file IS there but neither mode opened it: not a BombVault
		// repo, a corrupt one, or a permissions problem. Never "absent".
		return RepoUnreachable, scrubError(probeErr)
	}
	if s.repoEstablished(repo) && !s.destinationMounted(repo) {
		return RepoUnreachable, scrubError(ErrBackupPathNotMounted)
	}
	return RepoAbsent, ""
}

// transportFailureMarkers are substrings that prove a probe failed to REACH the
// backend, rather than reaching it and finding no repository. Caught on the live
// test box: pointing an off-site repo at a host with nothing listening makes
// restic answer
//
//	Fatal: unable to open config file: Head "http://…/config":
//	dial tcp 192.168.20.199:8000: connect: no route to host
//	Is there a repository at the following location?
//
// — which contains "unable to open config file", so isRepoUninitialized (and
// therefore a naive "absent") matches a dead network. For listSnapshots and
// probeOffsiteRepo that loose reading is harmless and deliberate (#117/#130:
// report "empty, not fatal"). For THIS feature it is the exact wrong answer:
// "absent" means "nothing exists, your choice decides", so a dead REST server
// would license the user to pick a mode and have BombVault create a second,
// empty repository beside backups that were there all along.
var transportFailureMarkers = []string{
	"dial tcp", "no route to host", "connection refused", "connection reset",
	"network is unreachable", "no such host", "i/o timeout", "timeout",
	"context deadline exceeded", "temporary failure in name resolution",
	"server misbehaving", "broken pipe", "unexpected eof",
	"tls", "certificate", "x509",
	"401", "403", "unauthorized", "forbidden", "access denied",
	"permission denied", "operation not permitted",
}

// isRepoDefinitelyAbsent reports whether err means "this location is reachable
// and holds no repository" — and nothing else. It is deliberately STRICTER than
// isRepoUninitialized: the uninitialized signal must be present AND no transport
// failure may be named alongside it.
//
// The asymmetry is intentional. Guessing "unreachable" when a repo is merely
// absent costs the user one visible "can't tell" line and a switch they set
// themselves. Guessing "absent" when a repo is merely unreachable can silently
// create an empty repository next to real backups. So anything ambiguous
// resolves to unreachable.
func isRepoDefinitelyAbsent(err error) bool {
	if !isRepoUninitialized(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range transportFailureMarkers {
		if strings.Contains(msg, marker) {
			return false
		}
	}
	return true
}

// foldEncryption folds the per-repository states into one verdict.
//
// The ORDER of the checks is the whole contract:
//
//   - encrypted AND plain both present → conflict. Two real repositories in
//     different modes; one global flag cannot open both.
//   - any definite detection, no contradiction → that mode. A repo that opened
//     is proof, and since the flag is global one proof settles it. Repos that
//     were absent or unreachable alongside it do not weaken that proof (a real
//     mismatch among them still surfaces from EnsureRepo on the next backup).
//   - no detection, but something was unreachable → unknown, NOT absent. This
//     is checked BEFORE absent on purpose: "one location is empty and another
//     could not be reached" must not read as "fresh install, pick anything" —
//     the unreachable one may be the encrypted repository the user is here to
//     restore.
//   - no detection, everything reachable and empty → absent. A genuine
//     first-time setup: nothing exists, so the user's choice decides.
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
