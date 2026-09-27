package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// lockHealEngine records the Unlock and ForgetPolicy calls of
// forgetWithLockHeal. Any other engine call panics on the nil embed.
type lockHealEngine struct {
	ResticEngine
	forgetErr   error
	forgetCalls int
	unlockCalls int
	unlockAll   bool
}

func (e *lockHealEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, _ []string, _ bool) error {
	e.forgetCalls++
	return e.forgetErr
}

func (e *lockHealEngine) Unlock(_ context.Context, _ string, removeAll bool, _ restic.Mode) error {
	e.unlockCalls++
	e.unlockAll = removeAll
	return nil
}

func TestForgetWithLockHealClearsStaleOrphanThenForgetsOnce(t *testing.T) {
	eng := &lockHealEngine{}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, []string{"container:x"}, true)
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

// A lock error from forget surfaces as-is so applyRetention notifies. restic's
// --retry-lock waits out a transient lock, and a lock that survives the stale
// clear belongs to a live holder that a forced unlock would strip of its
// protection (#94).
func TestForgetWithLockHealDoesNotForceUnlockOrRetryOnLockErr(t *testing.T) {
	lockErr := errors.New(`restic forget failed: unable to create lock in backend: repository is already locked by PID 12339 on 87379e1b0ca6 by root (UID 0, GID 0)`)
	eng := &lockHealEngine{forgetErr: lockErr}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, []string{"container:x"}, true)
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

func TestForgetWithLockHealPassesThroughOtherErrors(t *testing.T) {
	boom := errors.New("repository does not exist")
	eng := &lockHealEngine{forgetErr: boom}
	s := &Service{engine: eng}

	err := s.forgetWithLockHeal(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}, []string{"container:x"}, true)
	if !errors.Is(err, boom) {
		t.Fatalf("want the original error back, got %v", err)
	}
	if eng.forgetCalls != 1 || eng.unlockCalls != 1 {
		t.Fatalf("want 1 forget + 1 (unconditional) unlock, got forget=%d unlock=%d", eng.forgetCalls, eng.unlockCalls)
	}
}
