# Dependency run-state (#33) + config-backup delete (#34) — plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

Two GitHub bugs (ptmorris1). **#33:** a backup restarts a container that was stopped — the bug is in the
`StopContainers` dependency restart, which ignores run-state. **#34:** config backups can't be deleted (the
delete engine supports config, but the HTTP handler rejects it and there's no UI).

## Global Constraints
- Branch `fix/dep-runstate-and-config-delete` (off `main` == v4.5.2). Sequential implementers.
- Go gates: `go build ./... && go vet ./...`, `gofmt -l internal/ cmd/` empty, `go test ./... -count=1`,
  `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- No AI attribution (controller commits).

---

### Task 1 (#33): dependency containers follow detect-state (don't start a stopped dependency)
**Files:** `internal/backup/orchestrator.go`, `internal/api/service.go`, `internal/backup/orchestrator_test.go`.

Root cause (recon): the target's stop/restart is correctly gated on `WasRunning`, but `BackupDeps.StopContainers`
(the "stop these other containers during the backup", e.g. a DB) is a plain `[]string` with no run-state, and
the restart loop is unconditional. Because `Docker.Stop` on an already-stopped container returns `nil` (daemon
304), an already-off dependency is appended to `stoppedDeps` and then `Start`ed. In a backup-all it cascades.

- [ ] **Give dependencies run-state.** In `internal/backup/orchestrator.go`, change `BackupDeps.StopContainers`
  from `[]string` to a named slice `[]StopContainer` where `type StopContainer struct { Name string; WasRunning bool }`
  (define the type in the backup package). Update the field doc (drop "Independent of WasRunning").
- [ ] **Only stop+restart running dependencies.** In the dependency stop loop, skip a dep that wasn't running,
  so it is neither stopped nor restarted:
  ```go
  for _, dep := range d.StopContainers {
      if !dep.WasRunning {
          continue // already stopped: leave it exactly as it was
      }
      if stopErr := d.Docker.Stop(ctx, dep.Name, stopTimeout); stopErr != nil {
          log.Printf("backup: stop dependency %q: %v", dep.Name, stopErr) //nolint:gosec // G706: %q-quoted
          continue
      }
      stoppedDeps = append(stoppedDeps, dep.Name)
  }
  ```
  The existing restart loop (`for _, dep := range stoppedDeps { d.Docker.Start(ctx, dep) }`) is now correct —
  `stoppedDeps` holds only deps that were running. Keep it.
- [ ] **Service layer: compute each dependency's run-state.** In `internal/api/service.go` `Backup`
  (~:2050, where `StopContainers: tg.StopContainers` builds the deps), replace it with a loop that inspects each
  dependency name and builds `[]backup.StopContainer{Name, WasRunning: dep.Running}`:
  ```go
  var deps []backup.StopContainer
  for _, name := range tg.StopContainers {
      di, dErr := s.docker.Inspect(ctx, name)
      if dErr != nil {
          log.Printf("backup: inspect dependency %q: %v (leaving as-is)", name, dErr) //nolint:gosec // G706
          continue // can't inspect (e.g. removed) → don't touch it
      }
      deps = append(deps, backup.StopContainer{Name: name, WasRunning: di.Running})
  }
  ```
  and pass `StopContainers: deps`. (Use the real Inspect return field for running — mirror the target's
  `WasRunning: in.Running` at ~:2050 / `mapInspect` `out.Running`.)
- [ ] **Tests** (`internal/backup/orchestrator_test.go`): the two tests that encode the OLD behavior must be
  updated:
  - `TestBackupRestartsDepsEvenWhenTargetWasStopped` → rename/rework to assert a **running** dep is restarted
    and a **stopped** dep (`WasRunning:false`) is NOT started (nor stopped).
  - `TestBackupStopsAndRestartsDependencies` → deps are now `StopContainer{Name, WasRunning:true}`; assert the
    running dep is stopped then restarted. Add a case with a `WasRunning:false` dep asserting no Stop/Start on it.
  Update the `fakeDocker` if needed. Also verify `TestBackupStoppedContainerStaysStopped` (target path) still
  passes unchanged.
- [ ] Gates + commit: `fix(backup): don't restart a dependency container that was stopped (detect-state, #33)`.

---

### Task 2 (#34): let users delete config backups
**Files:** `internal/api/handlers.go`, `web/src/lib/api.ts`, `web/src/pages/Config.tsx`.

Root cause (recon): `Service.DeleteSnapshot`/`PruneDomain` are domain-generic and already handle `"config"`
(`repoFor`/`offsiteImmutableFor` have config arms). The blocker is `handleDeleteSnapshot` (handlers.go ~:1280)
whitelisting only `containers/vms/flash`; and Config.tsx has a read-only snapshot list.

- [ ] **Backend whitelist:** in `handleDeleteSnapshot` (handlers.go ~:1277-1290) add `"config"` to the
  `case "containers", "vms", "flash":` domain check. Do the same in `handlePrune` (~:1260-1273) so a config
  prune also works (PruneDomain already supports it). Read both handlers first to place it exactly.
- [ ] **Frontend api client:** widen the `deleteSnapshot(domain, ...)` `domain` union in `web/src/lib/api.ts`
  (~:938) to include `"config"` (`"containers" | "vms" | "flash" | "config"`).
- [ ] **Config.tsx delete button:** import `deleteSnapshot`. In the snapshots list (Config.tsx ~:326-336), give
  each row a delete button mirroring `Flash.tsx`'s `FlashSnapshotRow` (Flash.tsx ~:83-194): a `handleDelete`
  that `window.confirm(t("snapshots.deleteConfirm"))` → `await deleteSnapshot("config", snap.id, source)` → on
  success `void load()`, tracking local `deleting`/`deleteErr` state, rendering a `t("snapshots.delete")` button.
  Reuse the existing `source` state (local/offsite) so delete targets the viewed repo. The i18n keys
  `snapshots.delete` / `snapshots.deleteConfirm` already exist in all 24 locales — NO new i18n.
- [ ] Gates (Go + `tsc`/`build`) + commit: `feat(config): delete individual config backups from the Config page (#34)`.

## Self-review
- #33: dep run-state type (T1) built in the backup pkg + consumed in the service layer; tests updated. #34:
  handler whitelist + api union + Config UI, reusing existing delete engine + i18n. Covered.

## Handoff
Subagent-driven, sequential (both touch service.go/handlers.go). After T2: `/code-review` on the diff, then
release **v4.5.3** (patch) + answer/close #33 (pending ptmorris1's setup confirm) + #34.
