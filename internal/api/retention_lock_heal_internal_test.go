package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// lockHealEngine exercises forgetWithLockHeal's contract after the #92/#94
// reversal: a plain (removeAll=false) stale-orphan unlock always runs before
// forget, and forget itself is called exactly once — there is no more
// force-unlock-and-retry, because force-removing a lock cannot fix a live
// holder and would strip protection off a running restic op. Non-overridden
// engine calls panic loudly via the nil embed, proving the heal path touches
// nothing else.
type lockHealEngine struct {
	ResticEngine // nil — non-overridden calls panic loudly
	forgetErr    error
	forgetCalls  int
	unlockCalls  int
	unlockAll    bool
}

func (e *lockHealEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, _ string, _ bool) error {
	e.forgetCalls++
	return e.forgetErr
}

func (e *lockHealEngine) Unlock(_ context.Context, _ string, removeAll bool, _ restic.Mode) error {
	e.unlockCalls++
	e.unlockAll = removeAll
	return nil
}

// TestForgetWithLockHealClearsStaleOrphanThenForgetsOnce pins the happy path:
// a plain stale-orphan unlock (removeAll=false) runs once before a single
// forget call.
func TestForgetWithLockHealClearsStaleOrphanThenForgetsOnce(t *testing.T) {
	eng := &lockHealEngine{}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, "container:x", true)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if eng.forgetCalls != 1 {
		t.Fatalf("want exactly 1 forget attempt, got %d", eng.forgetCalls)
	}
	if eng.unlockCalls != 1 || eng.unlockAll {
		t.Fatalf("want exactly one plain Unlock(removeAll=false) before forget, got calls=%d removeAll=%v", eng.unlockCalls, eng.unlockAll)
	}
}

// TestForgetWithLockHealDoesNotForceUnlockOrRetryOnLockErr pins the reversal of
// #94: a lock error from forget is no longer force-unlocked (removeAll=true)
// and retried — it surfaces as-is so applyRetention notifies. Waiting out a
// transient lock is now the engine's job (--retry-lock), and a lock that
// survives the routine stale-clear is either a real orphan (already handled
// above) or a live holder that force-unlock cannot safely clear.
func TestForgetWithLockHealDoesNotForceUnlockOrRetryOnLockErr(t *testing.T) {
	lockErr := errors.New(`restic forget failed: unable to create lock in backend: repository is already locked by PID 12339 on 87379e1b0ca6 by root (UID 0, GID 0)`)
	eng := &lockHealEngine{forgetErr: lockErr}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, "container:x", true)
	if !errors.Is(err, lockErr) {
		t.Fatalf("want the original lock error surfaced unchanged, got %v", err)
	}
	if eng.forgetCalls != 1 {
		t.Fatalf("want exactly 1 forget attempt (no retry), got %d", eng.forgetCalls)
	}
	if eng.unlockCalls != 1 || eng.unlockAll {
		t.Fatalf("want exactly one plain Unlock(removeAll=false), got calls=%d removeAll=%v", eng.unlockCalls, eng.unlockAll)
	}
}

// TestForgetWithLockHealPassesThroughOtherErrors: a non-lock error is likewise
// passed straight through — nothing about forgetWithLockHeal is lock-error
// specific anymore.
func TestForgetWithLockHealPassesThroughOtherErrors(t *testing.T) {
	boom := errors.New("repository does not exist")
	eng := &lockHealEngine{forgetErr: boom}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, "container:x", true)
	if !errors.Is(err, boom) {
		t.Fatalf("want the original error back, got %v", err)
	}
	if eng.forgetCalls != 1 || eng.unlockCalls != 1 {
		t.Fatalf("want 1 forget + 1 (unconditional) unlock, got forget=%d unlock=%d", eng.forgetCalls, eng.unlockCalls)
	}
}
