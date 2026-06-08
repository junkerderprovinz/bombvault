# BombVault — Phase P1: Container Backup & Restore (TDD Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) tracking.

**Date:** 2026-06-08
**Spec:** `docs/superpowers/specs/2026-06-07-bombvault-design.md` (§5.1, §6.1, §11, §16)

## Goal

Ship the first **real** backup feature: a working vertical slice that backs up an Unraid Docker container's appdata to a **local restic repository** and restores it so the container reappears in the Unraid Docker tab — **manually triggered from the UI**. Backup = stop container → restic-backup appdata (tagged) → capture Unraid template → always restart container → record a Run. Restore = pick snapshot → confirm → stop+remove → restic-restore appdata → restore template → recreate+start via the Docker socket → record a Run.

## Architecture

Single Next.js (App Router) + custom-server process backed by SQLite (`better-sqlite3`). P1 adds three persisted tables (`destination`, `backup_target`, `run`) via the existing forward-only migration runner. Backup/restore are orchestrated **in-process** by an await orchestrator (no worker/queue yet — that is P2), invoked from server actions on a "Containers" UI surface. The orchestrator composes four typed adapters: restic (`lib/restic.ts`, extended), docker (`lib/docker.ts`, new, dockerode over the socket, DI for tests), appdata mapping (`lib/appdata.ts`, pure), template capture (`lib/unraid-template.ts`). The per-destination restic repo password is encrypted at rest with `lib/secrets.ts` (AES-256-GCM keyed by `APP_KEY`) and passed to restic only via `RESTIC_PASSWORD` env.

## Tech Stack

Node 22, TypeScript strict, Next.js 16 App Router + custom HTTPS server. `better-sqlite3` (^12) forward-only migrations. `restic` CLI (bundled in image; installed in CI). `dockerode` (^5) + `@types/dockerode`. `zod`. `node:crypto` via `lib/secrets.ts`. i18next (26 locales, parity enforced at compile time via `Translation` type + runtime via `test/locales.test.ts`). Tests: `node:test` + `tsx`; skip restic/`better-sqlite3` gracefully when absent.

---

## Honesty / test-reality (read before writing assertions)

| Capability | CI tests it for real? | How P1 tests it |
|---|---|---|
| restic backup→restore roundtrip | **YES** (restic installable in CI + image) | Real integration test: init temp repo, write known files, backup, wipe, restore, assert byte-identical + snapshot id listed. Suite `skip`s when `restic` absent locally. |
| restic arg building / `--json` parse | YES (pure) | Unit tests on exported pure fns with fixtures. |
| Docker socket (list/inspect/stop/start/create/pull) | **NO** (no socket/containers in CI) | Unit-tested against a **mocked dockerode** (DI). Real socket validated by the user on the Unraid box (like the P0 spike). |
| Unraid template read/write | Partial (fs real, no real flash) | Unit-tested against a temp dir; real `/boot/...` path validated on the box. |
| appdata path resolution | YES (pure) | Unit tests with inspect fixtures. |
| Backup/restore orchestration | **NO real Docker** | Unit-tested with all adapters mocked: asserts order (stop→backup→start), **always-restart on failure**, and Run recording. End-to-end on a real container = user release gate. |
| Schema/migrations | YES | Unit tests on `runMigrations` (tables/cols/idempotency/FK); skip if `better-sqlite3` unbuilt. |

**Non-negotiables:** restic repo password stored encrypted (`encryptSecret`/`decryptSecret`), passed only via `RESTIC_PASSWORD` env, never argv, never logged; keep error messages scoped (no SEC-006 regression). Do NOT claim container backup/restore works end-to-end from CI. No test depends on a network, a real socket, or a real flash dir.

---

## File Structure

### Created
- `lib/docker.ts` — dockerode wrapper + `DockerLike` interface (DI). `createDockerClient()`, `listContainers`, `inspectContainer`, `stopContainer(timeoutSec)`, `startContainer`, `removeContainer`, `pullImage`, `createAndStartFromDefinition`, pure `buildCreateOptions(inspect)`.
- `lib/appdata.ts` — pure `resolveAppdataPaths(inspect, appdataDir)`, `appdataPathForName(name, appdataDir)`.
- `lib/unraid-template.ts` — `templateFileName(name)`, `readTemplate(dir,name)`, `writeTemplate(dir,name,xml)`.
- `lib/backup-repo.ts` — typed CRUD for `destination`/`backup_target`/`run`; decrypts repo password via `secrets`.
- `server/orchestrator.ts` — `backupContainer(deps)`, `restoreContainer(deps)` with injected adapters; sequencing + always-restart + Run recording.
- `app/containers/page.tsx`, `app/containers/view.ts`, `app/containers/actions.ts` — containers UI + discovery + Back-up-now.
- `app/containers/snapshots/page.tsx`, `.../view.ts`, `.../actions.ts` — snapshot list + confirm-gated restore.
- `app/destinations/page.tsx`, `.../actions.ts`, `.../validate.ts` — local repo setup + init-on-save.
- Tests: `test/docker.test.ts`, `test/appdata.test.ts`, `test/unraid-template.test.ts`, `test/backup-repo.test.ts`, `test/orchestrator.test.ts`, `test/restic-args.test.ts`, `test/restic-roundtrip.test.ts`, `test/destinations-action.test.ts`, `test/containers-view.test.ts`, `test/snapshots-view.test.ts`.
- Fixtures: `test/fixtures/inspect-bind-appdata.json`, `inspect-no-appdata.json`, `restic-backup-summary.json`, `my-Plex.xml`.

### Modified
- `lib/restic.ts` — add pure `buildBackupArgs`, `parseBackupSummary`, `buildRestoreArgs` (+ `parseForgetJson`/`parseStatsJson` if cheap); async `backup`, `restore`, `stats`. Reuse `run()`/`subcommandOf()`.
- `lib/config.ts` — add `FLASH_TEMPLATES_DIR` (default `/boot/config/plugins/dockerMan/templates-user`).
- `server/schema.ts` — append migration **v2** (`init_p1_backup`): `destination`, `backup_target`, `run`. Never edit migration 1.
- `server/db.ts` — `PRAGMA foreign_keys = ON`.
- `lib/i18n/locales/en.ts` + `de.ts` (real) + 24 others (English placeholder, `// P1 — TODO translate`) — new UI keys (parity stays green).
- `app/dashboard/page.tsx` — links to `/containers` + `/destinations`.
- `.env.example` — document `FLASH_TEMPLATES_DIR`.
- `test/config.test.ts`, `test/schema.test.ts` — new cases.

> **i18n honesty:** `test/locales.test.ts` requires every locale to have exactly the en key set, and `Translation` is `Record<TranslationKey,string>` — so new keys go into all 26 locales now (en+de real, 24 placeholder English), with a follow-up to translate the 24.

---

## Parallelization map (for subagent execution)

```
Wave A (parallel, pure leaves): T1 config · T2 appdata · T3 template IO · T4 restic helpers · T5 i18n keys
Wave B (after A): T6 restic roundtrip (needs T4) · T7 docker adapter · T8 schema v2
Wave C: T9 backup-repo (needs T8 + secrets)
Wave D: T10 orchestrator (needs T4,T7,T9,T2,T3)
Wave E (parallel UI, after D): T11 destinations · T12 containers · T13 snapshots/restore
Wave F: T14 dashboard links + .env + full sweep
```

---

## Tasks

Each task: write failing test → run (expect FAIL, exact cmd) → minimal real code → run (expect PASS) → commit (conventional commit, NO AI attribution). Paths relative to `d:\nextcloud\it\github\bombvault`. Single file: `node --import tsx --test test/<file>`.

### Task 1 — Config: `FLASH_TEMPLATES_DIR`
- Test (`test/config.test.ts`): default `/boot/config/plugins/dockerMan/templates-user`; overridable.
- Code (`lib/config.ts`): add `FLASH_TEMPLATES_DIR: z.string().min(1).default("/boot/config/plugins/dockerMan/templates-user")` to schema + `AppConfig` + frozen return.
- Commit: `feat(config): add FLASH_TEMPLATES_DIR for Unraid template capture`

### Task 2 — appdata resolution (`lib/appdata.ts`)
- Fixtures: `inspect-bind-appdata.json` (`.Mounts:[{Type:"bind",Source:"/mnt/user/appdata/plex",Destination:"/config"},{...Media...}]`, `.Name:"/plex"`), `inspect-no-appdata.json` (`.Mounts:[]`, `.Name:"/whoami"`).
- Test (`test/appdata.test.ts`): `resolveAppdataPaths` returns only bind sources under appdataDir; falls back to `/mnt/user/appdata/<name>` when none; `appdataPathForName("/plex",APPDATA)==="/mnt/user/appdata/plex"`.
- Code: pure fns using `path.posix`; narrow inspect type (no `any`); filter `Type==="bind"` & `Source` under `appdataDir`, de-dupe, else name convention.
- Commit: `feat(appdata): resolve container appdata paths from inspect + Unraid convention`

### Task 3 — Unraid template IO (`lib/unraid-template.ts`)
- Fixture `test/fixtures/my-Plex.xml`.
- Test (`test/unraid-template.test.ts`): `templateFileName("Plex")==="my-Plex.xml"`; `readTemplate` null when absent; write→read roundtrip.
- Code: `templateFileName`, `readTemplate` (existsSync guard), `writeTemplate` (mkdirSync recursive). No content logging.
- Commit: `feat(unraid-template): read/write my-<Name>.xml on the flash templates dir`

### Task 4 — restic backup/restore/stats (`lib/restic.ts`)
- Test (`test/restic-args.test.ts`, pure): `buildBackupArgs("/repo",["/data/a","/data/b"],["container:plex","p1"])` === `["-r","/repo","backup","--json","--tag","container:plex","--tag","p1","/data/a","/data/b"]`; `buildRestoreArgs("/repo","abc123","/restore/here")` === `["-r","/repo","restore","abc123","--target","/restore/here"]`; `parseBackupSummary(fixture)` → `{snapshotId(/^[0-9a-f]{8,}$/), bytesAdded:number}`.
- Fixture `restic-backup-summary.json`: real multi-line `--json` stream with `status` lines + a final `{"message_type":"summary","snapshot_id":...,"total_bytes_processed":...,"data_added":...}`.
- Code: pure `buildBackupArgs`/`buildRestoreArgs`; `BackupSummary` interface; `parseBackupSummary` (parse each line in try/catch, pick `message_type==="summary"`, map `snapshot_id`/`data_added`/`total_bytes_processed`, throw if none); async `backup`/`restore`/`stats` over existing `run()`. `forget` arg-only + unit test if time-boxed.
- Commit: `feat(restic): add backup/restore (+stats) with --json parsing and pure arg builders`

### Task 5 — i18n keys (en/de real, 24 placeholder)
- Keys: `nav.containers`, `nav.destinations`, `containers.{title,discover,backupNow,lastBackup,never,colName,colImage,colStatus,colAppdata,colActions,backupStarted,noDestination}`, `destinations.{title,localPath,password,save,initOnSave,saved,testOk}`, `snapshots.{title,colId,colTime,colTags,colSize,restore,none}`, `restore.{confirmTitle,confirmBody,confirm,cancel,preview,started}`, `run.{kindBackup,kindRestore,statusRunning,statusSuccess,statusFailed,historyTitle,colKind,colStatus,colStarted,colFinished}`.
- Add to en.ts (extends `TranslationKey`) → `test/locales.test.ts` goes RED for 25 locales → add same keys to de (German) + 24 others (English placeholder in a `// P1 — TODO translate` block) → GREEN + `npm run typecheck`.
- Commit: `feat(i18n): add P1 container backup/restore UI keys (en+de; 24 locales placeholder)`

### Task 6 — restic roundtrip integration test (real)
- `test/restic-roundtrip.test.ts` with `{ skip }` via `execFileSync("restic",["version"])` probe (mirror `test/restic.test.ts`): init temp repo, write `hello.txt`, `backup` (tags), assert snapshot listed, wipe src, `restore` to target, read back from the nested absolute path under target, assert identical.
- Validates Task 4; adjust the restored-path join to restic's actual nesting. PASS in CI / SKIP if no binary.
- Commit: `test(restic): real backup→wipe→restore roundtrip integration test`

### Task 7 — docker adapter, mocked (`lib/docker.ts`)
- Test (`test/docker.test.ts`): inject a fake `DockerLike`; assert `buildCreateOptions(inspect)` maps Image/name(strip `/`)/Env/HostConfig.Binds; `stopContainer(d,"cid",30)` → `["get:cid","stop:{\"t\":30}"]`; `createAndStartFromDefinition` order `pull→create→start`.
- Code: `DockerLike` interface (subset: listContainers/getContainer/createContainer/pull); `createDockerClient(socketPath)` wrapping `new Docker({socketPath})` with `pull` draining `modem.followProgress`; narrow `ContainerInspect`/`CreateOptions` (no `any`); `buildCreateOptions` minimal map (Image, name, Env, Cmd, Binds, PortBindings, RestartPolicy — YAGNI); the list/inspect/stop/start/remove/pull/createAndStart fns.
- Commit: `feat(docker): dockerode wrapper (list/inspect/stop/start/create/pull) with DI for tests`

### Task 8 — schema migration v2 (`server/schema.ts`)
- Test (`test/schema.test.ts`, `{skip}` if no addon): v2 creates `destination`/`backup_target`/`run`; `run` has cols `id,target_id,kind,status,started_at,finished_at,snapshot_id,bytes,error,log_ref`; FK from `backup_target.destination_id` → `destination(id)` throws on bad ref (with `foreign_keys=ON`).
- Code: append `{version:2, name:"init_p1_backup", sql: <the three CREATE TABLEs + idx_run_target>}` (schema as in the plan body). Do not edit v1.
- Commit: `feat(schema): add destination, backup_target, run tables (migration v2)`

### Task 9 — backup-repo (`lib/backup-repo.ts`)
- First add `db.pragma("foreign_keys = ON")` to `server/db.ts`.
- Test (`test/backup-repo.test.ts`, `{skip}` if no addon): `createDestination` stores `password_ref` encrypted (`/^v1:/`, not plaintext) and `getDestinationPassword` decrypts; `createRun`→`finishRun` records status/snapshot; `lastBackupRun` returns latest successful backup.
- Code: `createRepo(db, appKey)` → typed CRUD; `randomUUID` ids, injectable `now`; `encryptSecret`/`decryptSecret` for the password; JSON-encode `appdata_paths`/`options`; no orchestration.
- Commit: `feat(backup-repo): typed CRUD for destinations/targets/runs with encrypted repo passwords`

### Task 10 — orchestrator (`server/orchestrator.ts`)
- Test (`test/orchestrator.test.ts`): inject mocked deps; happy path order `runStart→stop→backup→readTemplate→start→runFinish:success`; **on backup failure the container is still started AND run recorded `failed`** (rejects but `start` called); restore requires `confirmed:true` (throws otherwise) and orders `pull→stop→remove→resticRestore→writeTemplate→createAndStart→runFinish:success`.
- Code: `OrchestratorDeps` interface; `backupContainer(deps)` with always-restart via `finally`, tags `["container:"+ref,"p1"]`, persist template to `<DATA_DIR>/templates/<snapshotId>-my-<Name>.xml`, re-throw after recording failure; `restoreContainer(deps)` with `confirmed` guard; thin `makeOrchestratorDeps(...)` wiring real adapters (kept out of the DI seam).
- Commit: `feat(orchestrator): container backup/restore sequencing with guaranteed restart and run recording`

### Task 11 — destinations page + action (`app/destinations/`)
- Test (`test/destinations-action.test.ts`): `parseDestinationForm` (zod) rejects empty repoPath; accepts valid.
- Code: `validate.ts` (zod), `actions.ts` (`"use server"` saveDestinationAction → createDestination + initRepo tolerating "already initialized", revalidatePath), `page.tsx` (server component, `getTranslator()`, form, dark theme, never re-render password).
- Commit: `feat(ui): destinations page with local restic repo setup and init-on-save`

### Task 12 — containers page + Back-up-now (`app/containers/`)
- Test (`test/containers-view.test.ts`): `toContainerRows(list, appdataDir, lastBackupByName)` merges discovered containers with last-backup; marks no-run as never.
- Code: `view.ts` (pure), `actions.ts` (`backupNowAction`: ensure target exists, attach destination or throw "no destination", run `backupContainer`, revalidate), `page.tsx` (server component: `createDockerClient().listContainers` with graceful catch like the spike page; render table + per-row Back-up-now; i18n; dark).
- Commit: `feat(ui): containers page with discovery, last-backup status and Back up now`

### Task 13 — snapshots + restore (`app/containers/snapshots/`)
- Test (`test/snapshots-view.test.ts`): `toSnapshotRows` formats id/short_id/time/tags.
- Code: `view.ts` (pure); `actions.ts` (`restoreAction(targetId,snapshotId,confirmed)` requires `confirmed===true`, runs `restoreContainer`, revalidate); `page.tsx` (load destination, `snapshots(...)`, render rows, two-step confirm form; i18n; dark). File-level `restic ls` preview deferred to P2.
- Commit: `feat(ui): per-target snapshot list with confirm-gated restore`

### Task 14 — dashboard links, env docs, full sweep
- Add `/containers` + `/destinations` links to `app/dashboard/page.tsx`; document `FLASH_TEMPLATES_DIR` in `.env.example`.
- Full verification: `npm run lint`, `npm run typecheck`, `npm test` (restic + addon tests run for real in CI; SKIP locally if absent).
- Commit: `feat(ui): link Containers and Destinations from the dashboard; document FLASH_TEMPLATES_DIR`

---

## Risks / watch-items (carry into review)
- **restic restore path layout:** restic nests the absolute source path under `--target`. Task 6 must assert the real layout (verify on first CI run; don't assume `<target>/<basename>`).
- **dockerode `pull` stream** must be drained (`followProgress`) before the image is usable — in the adapter, not CI; user validates on the box.
- **FK enforcement** needs `PRAGMA foreign_keys = ON` per connection (Task 9).
- **i18n parity:** keep the 24 placeholder locales in a commented `// P1 — TODO translate` block; file a follow-up to translate them.
- **No worker yet:** "Back up now" runs synchronously in the server action (acceptable for P1 manual trigger; P2 adds the queue/worker — keep the orchestrator request-agnostic so P2 reuses it unchanged).

## Self-review (against spec §5.1/§6.1/§11/§16)
- §5.1 container backup (stop/appdata/template/start) → Tasks 7,10,3,4. §6.1 restore (appdata + template + recreate → Docker tab) → Task 10,13. §11 data model (destination/target/run) → Tasks 8,9. §16 error handling (always-restart, no overwrite without confirm, scoped errors) → Task 10 + the confirm gates. Secrets-at-rest → Task 9 (`lib/secrets.ts`). No spec requirement for P1 left without a task.
