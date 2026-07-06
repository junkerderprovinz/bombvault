# Per-container exclude patterns (#36) — implementation plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

Spec: `docs/superpowers/specs/2026-07-06-container-excludes-design.md`. Containers only. Reuse the existing
`excludes ...string` restic plumbing; feed it from a new per-container setting; translate user-typed container
paths to the anchored host-mount path restic stores; add a resolved-pattern preview with no-match warnings.

## Global Constraints
- Branch `feat/container-excludes` (off `main` == v4.5.4). Go gates: `go build ./... && go vet ./internal/...`,
  `gofmt -l internal/ cmd/` empty, `go test ./internal/... -count=1`, `golangci-lint run ./internal/...` 0.
  Frontend: `cd web && npx tsc --noEmit` + `npm run build` (then restore the gitignored `web/dist/index.html`
  if dirty). No AI attribution.
- Mirror the existing `StopContainers`/`SelectedPaths` per-target patterns exactly; `Excludes` is setter-owned
  and MUST stay out of `ON CONFLICT DO UPDATE SET`.

---

### Task 1 — Store: `Target.Excludes` + migration v53 + `SetExcludes`
**Files:** `internal/store/targets.go`, `internal/store/migrate.go`, `internal/store/targets_test.go` (or the
existing store test file).

- [ ] Add `Excludes []string` to `store.Target` (targets.go:31, after `StopContainers`) with a doc comment
  mirroring StopContainers ("Owned by SetExcludes (never reset by Upsert)").
- [ ] Migration: append to the migrations slice in `migrate.go` (highest is 52):
  `{version: 53, name: "target_excludes", sql: "ALTER TABLE targets ADD COLUMN excludes TEXT NOT NULL DEFAULT '[]';"}`
  (match the exact struct-literal shape of the entries at migrate.go:432-440).
- [ ] Wire the column through targets.go, mirroring `stop_containers` EXACTLY:
  - `UpsertTarget`: `exJSON, err := json.Marshal(t.Excludes)` (guard error); add `excludes` to the INSERT column
    list + a `?` + `string(exJSON)` to the args; **do NOT** add it to `ON CONFLICT DO UPDATE SET`.
  - `GetTargetByContainer` + `ListTargets`: add `excludes` to both SELECT column lists (end).
  - `scanTarget`: add `exJSON string`, append `&exJSON` to `s.Scan(...)` (end), and
    `json.Unmarshal([]byte(exJSON), &t.Excludes)` with an error guard.
- [ ] Add `func (r *Repo) SetExcludes(containerName string, excludes []string) error` mirroring
  `SetStopContainers` (targets.go:171-191): nil→`[]string{}`, marshal, `UPDATE targets SET excludes=? WHERE
  container_name=?`, and create-if-missing via `UpsertTarget(Target{ContainerName:..., Excludes: excludes})`.
- [ ] Test (mirror the StopContainers store test if present; else add): set excludes, read back; then call
  `UpsertTarget` again (a backup) and assert `Excludes` is NOT clobbered (the ON CONFLICT omission).
- [ ] Gates + commit `feat(store): per-container exclude patterns column + SetExcludes (#36)`.

### Task 2 — Orchestrator: `BackupDeps.Excludes` reaches `--exclude`
**Files:** `internal/backup/orchestrator.go`, `internal/backup/orchestrator_test.go`,
`internal/restic/restic_args_test.go` (reference).

- [ ] Add `Excludes []string` to `backup.BackupDeps` (orchestrator.go, near the other backup-input fields ~:166)
  with a doc comment ("restic --exclude patterns for this container's backup").
- [ ] Change the container backup call (orchestrator.go ~:364) from
  `d.Restic.Backup(ctx, d.RepoPath, d.AppdataPaths, tags)` to
  `d.Restic.Backup(ctx, d.RepoPath, d.AppdataPaths, tags, d.Excludes...)`.
- [ ] Test: mirror `TestBackupStopsAndRestartsDependencies`/the flash argv test — a `fakeRestic` (or the
  existing test double) capturing the excludes variadic; assert that `BackupDeps{Excludes: []string{"/host/user/x/Cache", ".git"}}`
  results in those reaching the restic `Backup` excludes. If the existing test double drops the variadic, extend
  it to record it. (restic_args_test.go:170-171 already proves `BackupArgs` emits `--exclude` per entry — no
  restic.go change needed.)
- [ ] Gates + commit `feat(backup): thread per-container excludes into the container backup (#36)`.

### Task 3 — Service: translation + preview + feed the backup
**Files:** `internal/api/service.go`, `internal/api/service_test.go`.

- [ ] Add the per-line resolver (place near `toContainerPath`, service.go ~:585). It needs `path`,
  `strings`, `model`:
  ```go
  // ExcludePreview is one exclude line resolved against a container's live mounts:
  // Resolved is the restic --exclude pattern that will actually be used, Status is
  // how it was derived, Matches reports whether it would exclude anything in this
  // container's backup (so the UI can warn on a line that matches nothing).
  type ExcludePreview struct {
      Raw      string `json:"raw"`
      Resolved string `json:"resolved"`
      Status   string `json:"status"` // "basename" | "translated" | "passthrough"
      Matches  bool   `json:"matches"`
  }

  // resolveExcludeLine turns one raw user line into a restic --exclude pattern.
  // No slash → verbatim (restic matches a bare name at any depth). A line under a
  // container mount Destination → translated through that mount's Source +
  // toContainerPath into the exact anchored path restic stored. Anything else →
  // verbatim (advanced host/glob patterns), never silently dropped.
  func (s *Service) resolveExcludeLine(line string, in model.Inspect) (pattern, status string) {
      line = strings.TrimSpace(line)
      if line == "" {
          return "", ""
      }
      if !strings.Contains(line, "/") {
          return line, "basename"
      }
      clean := path.Clean(line)
      var bestSrc, bestDest string
      for _, m := range in.Mounts {
          d := path.Clean(m.Destination)
          if d == "" || d == "/" || m.Source == "" {
              continue
          }
          if clean == d || strings.HasPrefix(clean, d+"/") {
              if len(d) > len(bestDest) {
                  bestDest, bestSrc = d, m.Source
              }
          }
      }
      if bestDest != "" {
          host := path.Clean(bestSrc + strings.TrimPrefix(clean, bestDest))
          if cp, ok := s.toContainerPath(host); ok {
              return cp, "translated"
          }
      }
      return line, "passthrough"
  }
  ```
- [ ] Add `resolveExcludePatterns(raw []string, in model.Inspect) []string` — map each raw line through
  `resolveExcludeLine`, skip empty patterns, return the resolved `--exclude` strings.
- [ ] Add `previewExcludes(raw []string, in model.Inspect, effective []string) []ExcludePreview`: for each raw
  line, resolve; set `Matches` = `status=="basename"` OR (`status=="translated"` AND the resolved path is a
  prefix of / under one of `effective`); `passthrough` → `Matches=false`. Return one entry per non-empty raw
  line (preserve the user's text in `Raw`). Use a small `isUnderAny(p, roots)` helper (equal, or `HasPrefix(p,
  root+"/")`).
- [ ] Add `func (s *Service) SetExcludes(_ context.Context, name string, excludes []string) error`: trim each
  line, drop blanks + exact dupes (preserve order), `s.store.SetExcludes(name, cleaned)`. Mirror
  `SetStopContainers` (service.go:4807-4819) minus the self-name check.
- [ ] Add `func (s *Service) PreviewExcludes(ctx, name string, candidate []string) ([]ExcludePreview, error)`:
  `in, err := s.docker.Inspect(ctx, name)` (return err), `effective := s.effectiveBackupPaths(name, in)`,
  return `s.previewExcludes(candidate, in, effective)`.
- [ ] Feed the backup: in `Backup` (service.go ~:2055-2072) set `Excludes: s.resolveExcludePatterns(tg.Excludes, in)`
  on the `backup.BackupDeps` literal (parallel to `StopContainers: deps`). `tg` already carries `Excludes` (Task 1)
  and `in` is the live inspect at :2004.
- [ ] Tests (service_test.go): (a) `resolveExcludeLine` for a `/config/...` line with a mount
  `{Source:"/mnt/user/appdata/plex", Destination:"/config"}` and `HostSourceRoot="/mnt"`,`HostMountRoot="/host/user"`
  resolves to `/host/user/user/appdata/plex/...` status `translated`; (b) a bare `.git` → `basename`; (c) a
  `/data/x` with no matching mount → `passthrough`; (d) `previewExcludes` sets `Matches=false` for a translated
  line whose volume is not in `effective`, `true` when it is.
- [ ] Gates + commit `feat(api): resolve + preview per-container exclude patterns, feed the backup (#36)`.

### Task 4 — API: DTO + preview endpoint
**Files:** `internal/api/handlers.go`, `internal/api/api.go`, `internal/api/handlers_test.go` (if present).

- [ ] `handlePatchContainer` (handlers.go:701-735): add `Excludes *[]string \`json:"excludes"\`` to the body
  struct; add `if body.Excludes != nil { if err := h.svc.SetExcludes(r.Context(), name, *body.Excludes); err !=
  nil { ...writeJSON 500... ; return } }` mirroring the StopContainers block at ~:730.
- [ ] `containerView` (handlers.go:173): add `Excludes []string \`json:"excludes"\``; populate
  `v.Excludes = t.Excludes` where the view is built from the target (handlers.go ~:216, next to StopContainers).
- [ ] Preview endpoint: add `handleExcludesPreview(w, r)` — `name := r.PathValue("name")`, decode body
  `{ "patterns": []string }`, call `h.svc.PreviewExcludes(r.Context(), name, body.Patterns)`, `writeJSON` the
  `[]ExcludePreview` (envelope `{ok:true, preview:[...]}` matching the codebase's envelope style — check a
  neighbouring handler). Register `mux.HandleFunc("POST /api/containers/{name}/excludes/preview", h.handleExcludesPreview)`
  inside the authGate group in api.go (next to the other container routes).
- [ ] Gates + commit `feat(api): container excludes PATCH field + preview endpoint (#36)`.

### Task 5 — Frontend: ExcludesEditor + preview
**Files:** `web/src/pages/Containers.tsx`, `web/src/lib/api.ts`.

- [ ] `api.ts`: add `excludes: string[]` to the `Container` interface (~:12-33, next to `stopContainers`);
  add `setContainerExcludes(name: string, excludes: string[]): Promise<OkEnvelope>` mirroring `setStopContainers`
  (~:692-698; PATCH `/api/containers/{name}` body `{excludes}`); add
  `previewContainerExcludes(name: string, patterns: string[]): Promise<{ok:boolean; preview:{raw:string;resolved:string;status:string;matches:boolean}[]}>`
  (POST `/api/containers/{name}/excludes/preview`).
- [ ] `Containers.tsx`: add an `ExcludesEditor` (clone `StopContainersEditor` ~:522-591) — a one-pattern-per-line
  `<textarea>` seeded `initial={container.excludes ?? []}`, a Save button calling `setContainerExcludes`, the
  same saved/error state pattern. Render it in the `installed && advanced` block (~:695-706) right after the
  existing folders/stop-containers editors.
- [ ] Preview UX: on mount + debounced on textarea change, call `previewContainerExcludes(name, currentLines)`;
  render, under the textarea, per non-empty line: the resolved `--exclude` pattern (muted) and, when
  `matches===false`, a warning (`excludes.noMatch` for passthrough / `excludes.volumeNotBackedUp` for a
  translated-but-unselected volume — pick the message by `status`). Match Containers.tsx styling conventions.
- [ ] Gates (`tsc --noEmit` + `npm run build`; restore `web/dist/index.html` if dirty). Commit
  `feat(web): per-container exclude-patterns editor with resolved-pattern preview (#36)`.

### Task 6 — i18n ×24 + inline en/de
**Files:** `web/src/lib/i18n.ts`, all 24 `web/src/lib/locales/*.ts`.

- [ ] Add these keys to inline `en` and `de` in i18n.ts (near the container-settings keys) and to ALL 24
  `locales/*.ts` files in the SAME pass ([[translate-all-locales-immediately]], one writer per file, disjoint):
  - `excludes.title` = "Exclude patterns"
  - `excludes.hint` = "One pattern per line. A container path (e.g. /config/…/Cache) is matched against the
    backed-up volume; a bare name like .git matches at any depth. Brace lists like {a,b} are not supported —
    one line each."
  - `excludes.placeholder` (multi-line example)
  - `excludes.save` = "Save excludes" / `excludes.saved` = "Excludes saved" / `excludes.error` = "Could not save excludes"
  - `excludes.resolvedTo` = "matches:" (label before the resolved pattern)
  - `excludes.noMatch` = "Passed to restic as-is (not a recognized container path)."
  - `excludes.volumeNotBackedUp` = "This folder's volume is not in the backup, so this line excludes nothing."
- [ ] Verify each locale has exactly the new keys (count) and `tsc --noEmit` passes (en/de are full typed maps
  → a missing key fails the build). Commit `i18n: exclude-patterns keys across all locales (#36)`.

## Self-review
Spec coverage: store (T1) → orchestrator (T2) → service translate/preview/feed (T3) → api DTO+preview (T4) →
frontend editor+preview (T5) → i18n (T6). Path-semantics hybrid + no-match preview + containers-only all
covered. Upsert-no-clobber and the doubled-`user` translation are explicitly tested.

## Handoff
Subagent-driven, sequential (T3 depends on T1+T2; T4 on T3; T5 on T4; T6 supports T5). After T6: adversarial
review of the full diff, then release **v4.6.0** (Minor) + close #36 (ONE reply, per [[one-issue-reply-after-release]]).
