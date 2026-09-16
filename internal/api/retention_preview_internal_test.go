package api

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// TestPreviewRetentionTakesNoLockAndKeepsNoRecord guards the four things that
// separate a preview from a prune, by reading the source rather than by
// exercising it — the same technique the repo already uses where a behaviour is
// easier to state than to provoke.
//
//   - tryLockDomainFor would make the preview refuse with errDomainBusy while a
//     backup runs, i.e. unavailable exactly when an operator wants to know what
//     tonight's run is about to delete.
//   - unlockStale DELETES lock files. A read-only endpoint that does that is a
//     repository writer, and it would break the promise
//     readonly_never_unlocks_internal_test.go was written to defend.
//   - progBegin and StartRun would put phantom prune rows in the Activity Log
//     and the run history for an operation that changed nothing.
func TestPreviewRetentionTakesNoLockAndKeepsNoRecord(t *testing.T) {
	raw, err := os.ReadFile("retention_preview.go")
	if err != nil {
		t.Fatalf("read retention_preview.go: %v", err)
	}
	body := receiverFuncBody(t, string(raw), `\(s \*Service\)`, "PreviewRetention", "retention_preview.go")
	for _, forbidden := range []string{"tryLockDomainFor", "lockDomainFor", "unlockStale", "progBegin", "StartRun"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("PreviewRetention calls %s — a preview must neither lock the domain, "+
				"clear a lock, nor leave a record of an operation that changed nothing", forbidden)
		}
	}
}

// previewEngine records what a retention preview asks the engine for. Every
// call that is NOT overridden here panics through the nil ResticEngine embed,
// which is the point: it proves a preview reaches for nothing that writes.
// Unlock, ForgetPolicy, Forget and Prune are deliberately left unimplemented.
type previewEngine struct {
	ResticEngine // nil — any non-overridden call panics loudly

	snaps    []restic.Snapshot
	snapsErr error

	previewTags   []string
	previewRepos  []string
	previewGroups []restic.ForgetGroup
	previewErr    error
}

func (e *previewEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, e.snapsErr
}

func (e *previewEngine) ForgetPreview(_ context.Context, repo string, _ restic.RetentionPolicy, _ restic.Mode, tag string) ([]restic.ForgetGroup, error) {
	e.previewRepos = append(e.previewRepos, repo)
	e.previewTags = append(e.previewTags, tag)
	return e.previewGroups, e.previewErr
}

func snapWithTags(id string, tags ...string) restic.Snapshot {
	return restic.Snapshot{ID: id, Time: "2026-09-15T02:00:00.000000000+02:00", Tags: tags}
}

// TestPreviewRetentionPerIdentityMirrorsTheRealPass pins that the preview asks
// exactly the question the real retention pass answers: one tag-scoped preview
// per identity tag, in the same order applyRetentionPerIdentity would forget
// them. A repo-wide preview would report removals that never happen and hide
// ones that do, because that is not the pass BombVault actually runs.
func TestPreviewRetentionPerIdentityMirrorsTheRealPass(t *testing.T) {
	eng := &previewEngine{snaps: []restic.Snapshot{
		snapWithTags("a1", "container:plex", "p1"),
		snapWithTags("b2", "vm:win11"),
		snapWithTags("c3", "container:plex"),
		snapWithTags("d4", "flash"),
	}}
	s := &Service{engine: eng}

	if _, err := s.previewRetentionPerIdentity(context.Background(), "/repo",
		restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}); err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	want := []string{"container:plex", "vm:win11", "flash"}
	if len(eng.previewTags) != len(want) {
		t.Fatalf("want one preview per identity tag %v, got %v", want, eng.previewTags)
	}
	for i, w := range want {
		if eng.previewTags[i] != w {
			t.Fatalf("preview %d: got tag %q want %q (full: %v)", i, eng.previewTags[i], w, eng.previewTags)
		}
	}
}

// TestPreviewRetentionFallsBackToRepoWide pins the same fallback
// applyRetentionPerIdentity has: when the snapshot listing fails, or carries no
// identity tags at all, retention runs ONE repo-wide paths-grouped pass. The
// preview has to show that pass, or it would claim nothing would be removed
// while the real run removes plenty.
func TestPreviewRetentionFallsBackToRepoWide(t *testing.T) {
	t.Run("listing failed", func(t *testing.T) {
		eng := &previewEngine{snapsErr: errors.New("repository is unreachable")}
		s := &Service{engine: eng}

		if _, err := s.previewRetentionPerIdentity(context.Background(), "/repo",
			restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}); err != nil {
			t.Fatalf("want no error, got %v", err)
		}
		if len(eng.previewTags) != 1 || eng.previewTags[0] != "" {
			t.Fatalf("want a single repo-wide preview (empty tag), got %v", eng.previewTags)
		}
	})

	t.Run("no identity tags", func(t *testing.T) {
		eng := &previewEngine{snaps: []restic.Snapshot{snapWithTags("a1", "p1", "live")}}
		s := &Service{engine: eng}

		if _, err := s.previewRetentionPerIdentity(context.Background(), "/repo",
			restic.RetentionPolicy{KeepLast: 5}, restic.Mode{}); err != nil {
			t.Fatalf("want no error, got %v", err)
		}
		if len(eng.previewTags) != 1 || eng.previewTags[0] != "" {
			t.Fatalf("want a single repo-wide preview (empty tag), got %v", eng.previewTags)
		}
	})
}

// TestPreviewRetentionInertPolicyAsksNothing pins that retention which is
// switched off produces no engine traffic at all — not even a snapshot listing.
// Asking restic with an inert policy would report the entire repository as
// about to be removed (forget with no --keep-* flag keeps nothing).
func TestPreviewRetentionInertPolicyAsksNothing(t *testing.T) {
	eng := &previewEngine{snaps: []restic.Snapshot{snapWithTags("a1", "container:plex")}}
	s := &Service{engine: eng}

	groups, err := s.previewRetentionPerIdentity(context.Background(), "/repo",
		restic.RetentionPolicy{}, restic.Mode{})
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if groups != nil {
		t.Fatalf("an inert policy must preview nothing, got %+v", groups)
	}
	if len(eng.previewTags) != 0 {
		t.Fatalf("an inert policy must not reach the engine, got %v", eng.previewTags)
	}
}

// TestPreviewRetentionSurvivesOneFailingTag pins that a single unreadable
// identity does not blank the whole answer: the other identities are still
// reported, and the failure is returned alongside them. A preview that returns
// only an error would tell an operator nothing about the repositories that
// answered perfectly well.
func TestPreviewRetentionSurvivesOneFailingTag(t *testing.T) {
	eng := &previewEngine{
		snaps:      []restic.Snapshot{snapWithTags("a1", "container:plex"), snapWithTags("b2", "vm:win11")},
		previewErr: errors.New("no such snapshot"),
	}
	s := &Service{engine: eng}

	_, err := s.previewRetentionPerIdentity(context.Background(), "/repo",
		restic.RetentionPolicy{KeepLast: 5}, restic.Mode{})
	if err == nil {
		t.Fatal("want the per-tag failure surfaced, got nil")
	}
	if len(eng.previewTags) != 2 {
		t.Fatalf("a failing tag must not stop the remaining ones, got %v", eng.previewTags)
	}
}
