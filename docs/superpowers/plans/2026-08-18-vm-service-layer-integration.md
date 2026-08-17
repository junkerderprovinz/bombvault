# VM Service-Layer Integration (post-Phase-B follow-up)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (fresh subagent per task, spec-review then code-quality-review) to implement this plan task-by-task.

**Goal:** close the two gaps Phase B's whole-branch review explicitly flagged as open (`docs/superpowers/plans/2026-08-16-bombvault-platform-expansion.md`'s Task 10/11 follow-ups): make zvol backup, TPM capture, and TrueNAS-26 friendly VM names actually reachable from the real UI/API, and make retention/restore correct once a single VM backup can legitimately produce more than one restic snapshot.

**Context (why this exists):** Phase B (PR #151) built the zvol-aware VM backup path (`internal/backup/vm_orchestrator.go`'s `BackupZvolDisk`/`RestoreZvolDisk`, wired into `runVMGraceful`/`runVMLive`/`runVMRestore` via `VMBackupDeps.BlockDisks`/`ZFSHost`/`ZvolRestic`), vTPM parsing (`internal/virshcli/tpm.go`, `DomainInfo.TPMPath`), and TrueNAS domain-name resolution (`internal/virshcli`'s `normalizeDomainName`, `VMInfo.FriendlyName`). None of these are reachable from a real backup click today: `internal/api/service.go`'s actual `BackupVM`/`RestoreVM`/`ListVMs` never populate or consume any of them. This plan closes that gap.

## Design decision: how multiple snapshots per VM backup are represented

A VM with any block-device (zvol) disks needs one restic invocation PER zvol disk (restic's `--stdin` mode is one stream per invocation) plus, if the VM also has file-backed disks, one more invocation for those — so a real TrueNAS VM backup can produce 2+ restic snapshots from one backup click. This is likely the COMMON case on TrueNAS (most VMs have more than one disk, and TrueNAS provisions zvols by default), not a rare edge case.

**Chosen design — tag-scheme correlation, no DB schema migration:**

- The file-backed-disk snapshot keeps today's `vm:<name>` tag exactly as-is (zero behavior change for every existing Unraid/file-only VM).
- Each additional zvol disk's snapshot gets its own tag: `vm:<name>:zvol:<dev>` (e.g. `vm:debian:zvol:vdb`).
- **Every** snapshot produced by one backup invocation (main + all zvol disks) additionally gets a shared correlation tag `vmrun:<runID>`, where `<runID>` is the string returned by `store.Repo.StartRun` **before** the backup begins (confirmed available up-front — `internal/store/runs.go:29`).
- `Runs.FinishRun`'s signature is **unchanged** — it still records one primary snapshot ID (the file-backed snapshot's ID if present, else the first zvol snapshot's ID) purely for the existing history-list display. No schema migration.
- **Retention** (`applyRetention`/`ForgetPolicy` in `internal/api/service.go`) is called once per identity tag actually present for the VM — i.e. once for `vm:<name>` and once per distinct `vm:<name>:zvol:<dev>` tag seen — reusing restic's existing native per-tag `--keep-*` policy engine completely unchanged. **No new retention algorithm.** This is the safest option given none of it can be tested against real TrueNAS hardware: it reuses a mechanism that already works correctly today, rather than inventing new batch-aware pruning logic.
- **Restore** resolves the target `Run` row (explicit ID, or the latest for this VM via the existing DB query), then asks restic for every snapshot tagged `vmrun:<that runID>`, and restores the file-backed snapshot to the VM's file disk paths (existing behavior, untouched) plus each zvol snapshot to its corresponding zvol dataset (Task 10's already-built, previously-unreachable `RestoreZvolDisk` path).

**Why not a DB migration / a `RunSnapshots` join table:** restic already durably records every snapshot's tags; querying it directly at restore time is exactly how "latest" resolution already works today (`internal/api/service.go` around line 4152, tag-prefix scan). Reusing that mechanism for `vmrun:` avoids a migration and a second source of truth that could drift from what restic actually has.

## TPM state: extend the existing NVRAM mechanism, don't wire the decorative field

`vm_orchestrator.go`'s `VMBackupDeps.TPMPath`/`VMRestoreDeps.TPMPath` (Phase B Task 11) were deliberately wired at the exact same weak layer NVRAM's own `NVRAMPath` field already sat at — **not** the real mechanism. The real, working NVRAM capture/restore is a separate, older, inline SSH read/write directly in `service.go`'s `BackupVM`/`prepareRestoreVMForTarget` (`s.ssh.ReadFile`/`WriteFile`), storing bytes in a DB-persisted `NVRAMBytes []byte` field on the VM's stored definition.

**Chosen approach:** extend that *existing, working* inline mechanism to also read/write TPM state the same way, using `DomainInfo.TPMPath` (already parsed by Task 11's `ParseDomain`) as the source path and a new parallel `TPMBytes []byte` DB field — **not** populating `vm_orchestrator.go`'s `TPMPath`/`NVRAMPath` fields, which stay exactly as unreachable-from-service.go as they already were (that's fine; they exist for direct-orchestrator callers, e.g. tests and any future non-service caller).

## Friendly VM names (Task 12 follow-through)

`internal/api/service.go`'s `ListVMs` currently builds `VMView{Name: vm.Name}` — the raw libvirt domain name, which is a bare UUID on TrueNAS 26. `virshcli.VMInfo.FriendlyName` already resolves this (Phase B Task 12) but is never consumed. Per the explicit gotcha already documented on that field (`internal/virshcli/types.go`): the classifier is shape-based, not platform-gated, so consuming it safely requires checking the platform first.

---

## Tasks

### Task 1: Thread a run-correlation tag through the VM backup orchestrator

**Files:**
- Modify: `internal/backup/vm_orchestrator.go` — `VMBackupDeps`/`VMRestoreDeps` (add `RunTag string`, the `vmrun:<runID>` value, empty-safe/optional), `runVMGraceful`/`runVMLive`/`runVMRestore`, `backupBlockDisksAndLog`/`restoreBlockDisksAndLog` (thread the extra tag into every restic call alongside the existing `vm:<name>`/`vm:<name>:zvol:<dev>` tags).
- Modify: `internal/backup/vm_orchestrator_test.go` — regression tests: when `RunTag` is empty, behavior/tags are byte-identical to before this task (critical — this must never break the existing file-only-VM path or any existing test); when set, every restic call (main + each zvol disk) carries both its own identity tag AND the shared `vmrun:` tag.

This task ONLY touches the orchestrator layer (already-tested, already-isolated from `service.go`) — get the tag plumbing right and regression-proven before touching the real caller in Task 2.

### Task 2: Wire `service.go`'s real `BackupVM`/`RestoreVM` to the zvol/TPM/run-tag mechanisms

**Files:**
- Modify: `internal/api/service.go` — `BackupVM` (populate `VMBackupDeps.BlockDisks = domain.BlockDisks`, a real `ZFSHost`/`ZvolRestic` implementation, `RunTag = "vmrun:" + runID`; extend the existing inline NVRAM SSH read/write to also read TPM bytes from `domain.TPMPath` into a new `TPMBytes` field alongside `NVRAMBytes`), `prepareRestoreVMForTarget`/restore path (same for `VMRestoreDeps`, plus writing `TPMBytes` back via SSH mirroring the existing NVRAM write-back), and `applyRetention`'s VM call site (call it once per identity tag actually present — main `vm:<name>` plus any `vm:<name>:zvol:<dev>` tags seen in this backup, not just once).
- Modify: the VM's stored-definition struct (wherever `NVRAMBytes` lives) — add `TPMBytes []byte`.
- Test: extend whatever test file already covers `BackupVM`/restore's NVRAM handling with the same shape for TPM, plus a fake multi-disk VM (1 file disk + 2 zvol disks) proving: 3 restic calls happen, each has the right identity tag, all three share the same `vmrun:` tag, and retention gets invoked 2 times (once for `vm:<name>`, once for `vm:<name>:zvol:vdb`) — not once, not three times.

This is the task that actually makes Phase B's zvol/TPM code reachable — the core of this plan.

### Task 3: Restore resolution via `vmrun:` tag grouping

**Files:**
- Modify: `internal/api/service.go` — wherever VM restore resolves "which snapshot(s) to use" (both explicit-run and "latest" cases): after resolving the target `Run` row, query restic for all snapshots tagged `vmrun:<that run's ID>` and pass the resulting set (file snapshot ID + zvol snapshot IDs, keyed by tag) into `RestoreVM`'s deps.
- Handle the case where `vmrun:` isn't present (a Run predating this plan, or a file-only VM backup that never got the multi-tag treatment) — must fall back to exactly today's single-`vm:<name>`-tag resolution, unchanged. This is a real, permanent case (existing history), not a transitional one to special-case away.
- Test: restoring a Run that has a `vmrun:` group with 3 snapshots restores all 3 correctly; restoring a pre-existing Run with no `vmrun:` tag behaves byte-identically to before this plan.

### Task 4: Friendly VM name display, platform-gated

**Files:**
- Modify: `internal/api/service.go` — `ListVMs`: when `s.platformFn().Kind() == platform.KindTrueNAS`, use `vm.FriendlyName` for `VMView.Name`'s *display* while continuing to use `vm.Name` (the raw libvirt name) for every internal lookup (`byName[vm.Name]`, `live[vm.Name]`, and all virsh-command call sites) — the friendly name is presentation-only, never an identifier. On any other platform, behavior is unchanged (`FriendlyName` happens to equal `Name` there anyway per Task 12's own classifier, but gate explicitly rather than relying on that).
- Test: a TrueNAS-platform fake VM list with a UUID-style name displays its `<title>`-resolved friendly name in `VMView.Name` while `store.VMTarget` matching still works via the raw name; an Unraid-platform fake VM list is unaffected.

### Task 5: i18n — translate the 3 undocumented env vars into all 25 non-English `docs/configuration.*.md` files

**Files:**
- Modify: `docs/configuration.ar.md`, `.cs.md`, `.da.md`, `.de.md`, `.el.md`, `.es.md`, `.fi.md`, `.fr.md`, `.he.md`, `.hu.md`, `.it.md`, `.ja.md`, `.ko.md`, and every other locale listed in `mkdocs.yml`'s `languages:` block (26 total incl. English).

`PLATFORM`, `DATA_ROOT_SEGMENTS` (Phase A) and `LIBVIRT_URI` (Phase B) exist in the English `docs/configuration.md`'s env-var table but not in any translated copy. One agent per language (per the project's own established mkdocs-i18n pattern — reference `web/src/lib/locales/<code>.ts` for terminology consistency with the app itself, no em dashes, translate only the 3 new table rows' prose — do not touch anything else in each file).

---

## Live verification

None of this is testable against real TrueNAS hardware (unchanged from Phase B's own caveat — no test instance available). Unit/integration-tested against fakes only. This must be stated plainly in whatever ships this, exactly as Phase B's PR did.
