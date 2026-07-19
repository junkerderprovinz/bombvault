package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// lockHealEngine simulates the #94 situation: the first forget hits an orphaned
// restic lock, the retry (after the force-unlock) succeeds. Non-overridden
// engine calls panic loudly via the nil embed, proving the heal path touches
// nothing else.
type lockHealEngine struct {
	ResticEngine         // nil — non-overridden calls panic loudly
	forgetErrs   []error // popped per call; empty → nil
	forgetCalls  int
	unlockCalls  int
	unlockAll    bool
}

func (e *lockHealEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, _ string, _ bool) error {
	e.forgetCalls++
	if len(e.forgetErrs) == 0 {
		return nil
	}
	err := e.forgetErrs[0]
	e.forgetErrs = e.forgetErrs[1:]
	return err
}

func (e *lockHealEngine) Unlock(_ context.Context, _ string, removeAll bool, _ restic.Mode) error {
	e.unlockCalls++
	e.unlockAll = removeAll
	return nil
}

// #94: an orphaned lock (e.g. left by a container update mid-operation) used to
// fail every retention pass of the night — forget needs an EXCLUSIVE lock, so
// even a stale non-exclusive lock blocks it while backups keep succeeding. The
// heal must force-unlock (safe: every caller holds the domain lock, BombVault is
// the repo's sole writer) and retry exactly once.
func TestForgetWithLockHealRetriesOnceOnLockErr(t *testing.T) {
	lockErr := errors.New(`restic forget failed: unable to create lock in backend: repository is already locked by PID 12339 on 87379e1b0ca6 by root (UID 0, GID 0)`)
	eng := &lockHealEngine{forgetErrs: []error{lockErr}}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, "container:x", true)
	if err != nil {
		t.Fatalf("heal should succeed after unlock+retry, got %v", err)
	}
	if eng.forgetCalls != 2 {
		t.Fatalf("want exactly 2 forget attempts (original + one retry), got %d", eng.forgetCalls)
	}
	if eng.unlockCalls != 1 || !eng.unlockAll {
		t.Fatalf("want exactly one Unlock(removeAll=true), got calls=%d removeAll=%v", eng.unlockCalls, eng.unlockAll)
	}
}

// A non-lock error must NOT trigger the unlock+retry — it is returned as-is so
// the caller notifies (the pre-#94 behaviour for real failures).
func TestForgetWithLockHealPassesThroughOtherErrors(t *testing.T) {
	boom := errors.New("repository does not exist")
	eng := &lockHealEngine{forgetErrs: []error{boom, boom}}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, "container:x", true)
	if !errors.Is(err, boom) {
		t.Fatalf("want the original error back, got %v", err)
	}
	if eng.forgetCalls != 1 || eng.unlockCalls != 0 {
		t.Fatalf("non-lock error must not unlock/retry: forget=%d unlock=%d", eng.forgetCalls, eng.unlockCalls)
	}
}

// If the lock persists even after the force-unlock retry, the error surfaces so
// applyRetention notifies the user (nothing is silently swallowed).
func TestForgetWithLockHealSurfacesPersistentLock(t *testing.T) {
	lockErr := errors.New("unable to create lock in backend: repository is already locked")
	eng := &lockHealEngine{forgetErrs: []error{lockErr, lockErr}}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, "container:x", true)
	if !isLockErr(err) {
		t.Fatalf("persistent lock must surface, got %v", err)
	}
	if eng.forgetCalls != 2 || eng.unlockCalls != 1 {
		t.Fatalf("want 2 forgets + 1 unlock, got forget=%d unlock=%d", eng.forgetCalls, eng.unlockCalls)
	}
}
