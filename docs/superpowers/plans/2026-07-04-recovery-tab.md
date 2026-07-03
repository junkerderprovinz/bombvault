# Guided Recovery Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A guided "Recovery" tab that walks a fresh/rebuilt BombVault install through attaching an existing backup repo, discovering the containers/VMs stored in it, and restoring them, plus a fresh-install nudge on the Dashboard.

**Architecture:** Frontend-only orchestration glue over existing endpoints. A new `/recovery` page (built from small step-cards) + a Sidebar NavItem + a route, plus a Dashboard nudge. No backend changes.

**Tech Stack:** React + Vite + TypeScript SPA (`web/`), existing `web/src/lib/api.ts` client, `useT()` i18n, Carbon-style Tailwind classes, react-router.

## Global Constraints

- Branch `feat/recovery-tab` only; never touch `main`. Do NOT push (the orchestrator gates + pushes).
- No AI attribution in commits.
- **No backend changes.** Every capability already exists and is exposed in `web/src/lib/api.ts`.
- Frontend gate for EVERY task: `cd web && npx tsc --noEmit` (0 errors) && `npm run build` (green); then `git checkout -- web/dist/index.html` (the built `dist/index.html` is tracked; revert it after each build). `web/dist/assets/*` are gitignored.
- This repo has NO JS unit-test runner — the gate IS tsc + build. Keep pure logic (orchestration helpers, the fresh-install predicate) in small exported functions so they are trivially correct by inspection; each task states a manual verification.
- New i18n keys go into `web/src/lib/i18n.ts` en+de ONLY. The 24 locale files under `web/src/lib/locales/` are done in one batch at close-out (Task 8) — implementers must NOT edit them.
- Reuse existing components/calls; do NOT duplicate persistence. Attach/creds go through the existing `putSettings`/`setCloud`/`setRclone`; restore through the existing `restore`/`restoreVM` + the v4 restore-UX.

### Existing API (verified signatures — consume, don't recreate)

```ts
// web/src/lib/api.ts
listContainers(): Promise<ListContainersResponse>            // :311  -> { containers: Container[] }
listVMs(): Promise<ListVMsResponse>                          // :968  -> { vms: VM[] }
listSnapshots(name: string, source?: string): Promise<ListSnapshotsResponse>       // :346
listVMSnapshots(name: string, source?: string): Promise<ListSnapshotsResponse>     // :979
restore(...)                                                 // :358  (container restore; see Containers.tsx usage)
restoreVM(...)                                               // :984
discover(): Promise<OkEnvelope & { discovered?: number }>    // :644  POST /api/discover
discoverVMs(): Promise<OkEnvelope & { discovered?: number }> // :649  POST /api/vms/discover
getSettings(): Promise<GetSettingsResponse>                  // :700
putSettings(...)                                             // :709
setCloud(c: CloudCreds): Promise<OkEnvelope>                 // :613
setRclone(conf: string): Promise<OkEnvelope>                // :873
getStatus(): Promise<StatusResponse>                         // :895  -> { domains: DomainStatus[] }
recoveryKitUrl(): string                                     // :724
```
Types: `Container` (api.ts:12), `VM` (:950), `Settings` (:80), `DomainStatus` (:159). Bulk restore helper: `fireAndWaitRun` (`web/src/lib/backupWatch.ts:304`), used by `Containers.tsx` `restoreSelected` (:1098) — the fire/retry/wait cycle for the server's single-flight. Sidebar `NavItem({to,label,icon,disabled,comingSoon})` (`Sidebar.tsx:94`); routes flat in `web/src/app/router.tsx:17-24`.

---

### Task 1: Scaffold the Recovery tab (route + NavItem + page shell)

**Files:**
- Create: `web/src/pages/Recovery.tsx`
- Modify: `web/src/app/router.tsx` (add route), `web/src/components/Sidebar.tsx` (add NavItem + icon), `web/src/lib/i18n.ts` (`nav.recovery`, `recovery.title`, `recovery.intro`)

**Interfaces:**
- Produces: a default-exported `Recovery` page component; a `/recovery` route; a "Recovery" NavItem always visible.

- [ ] **Step 1: Page shell.** Create `web/src/pages/Recovery.tsx`:
```tsx
import { useT } from "../lib/i18n";

export default function Recovery() {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-5 p-1">
      <div>
        <h1 className="text-lg font-semibold text-carbon-text">{t("recovery.title")}</h1>
        <p className="text-sm text-carbon-textMuted mt-1 max-w-2xl">{t("recovery.intro")}</p>
      </div>
      {/* Step cards added in Tasks 2-6 */}
    </div>
  );
}
```

- [ ] **Step 2: Route.** In `web/src/app/router.tsx`, import `Recovery` and add after the `/flash` route:
```tsx
<Route path="/recovery" element={<Recovery />} />
```

- [ ] **Step 3: NavItem.** In `web/src/components/Sidebar.tsx`, add a small inline icon component near the other `Icon*` helpers, e.g.:
```tsx
function IconRecovery() {
  return (<svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.4"><path d="M8 2.5a5.5 5.5 0 1 0 5.2 3.7"/><path d="M13.5 2v3.2H10.3"/></svg>);
}
```
Then add a `<NavItem to="/recovery" label={t("nav.recovery")} icon={<IconRecovery />} />` in the top nav group (after Dashboard/Jobs, before Containers — a sensible place for a recovery entry). Always visible (no advanced/enabled gate).

- [ ] **Step 4: i18n.** Add en+de in `web/src/lib/i18n.ts`: `nav.recovery` ("Recovery"/"Wiederherstellung"), `recovery.title` ("Disaster recovery"/"Notfall-Wiederherstellung"), `recovery.intro` (en: "Recover your containers and VMs from an existing backup onto this install. Point BombVault at your backups, discover what's in them, and restore." / de: natural translation).

- [ ] **Step 5: Gate + commit.** `cd web && npx tsc --noEmit && npm run build`, revert `web/dist/index.html`. Commit: `feat: scaffold guided Recovery tab (route + nav + page shell)`. Manual check: the Recovery tab appears in the sidebar and routes to the page.

---

### Task 2: Step card — connection check (APP_KEY / repo readable)

**Files:** Modify `web/src/pages/Recovery.tsx` (add the card + a small `web/src/components/recovery/StepCard.tsx` wrapper), `web/src/lib/i18n.ts`.

**Interfaces:**
- Consumes: `listSnapshots` (api.ts:346), `getSettings`.
- Produces: `StepCard` (a titled card with a status pill: `ok|warn|bad|idle`) reused by later steps; a `readable` state on the Recovery page shared with later steps.

- [ ] **Step 1: StepCard wrapper.** Create `web/src/components/recovery/StepCard.tsx`:
```tsx
import type { ReactNode } from "react";

export type StepState = "idle" | "ok" | "warn" | "bad";

export function StepCard({ n, title, state, children }: { n: number; title: string; state: StepState; children: ReactNode }) {
  const dot = state === "ok" ? "bg-[#6fdc8c]" : state === "bad" ? "bg-[#ff8389]" : state === "warn" ? "bg-[#f1c21b]" : "bg-carbon-surface3";
  return (
    <div className="rounded-xl border border-carbon-border bg-carbon-surface p-4">
      <div className="flex items-center gap-2.5 mb-2">
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-carbon-surface2 text-xs font-semibold text-carbon-textSub">{n}</span>
        <h2 className="text-sm font-semibold text-carbon-text flex-1">{title}</h2>
        <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      </div>
      <div className="text-sm text-carbon-textMuted flex flex-col gap-2">{children}</div>
    </div>
  );
}
```

- [ ] **Step 2: Connection-check logic.** On the Recovery page, add a `checkReadable()` that reads `getSettings()` to find the first configured/enabled domain, calls `listSnapshots(<domainProbeName?>...)` — NOTE: `listSnapshots(name)` lists a container's snapshots; for a repo-level readability probe, call it with the first discovered/known name if any, else attempt a domain snapshot list. Simplest robust probe: call `getStatus()` (which reads the repos) and/or `listContainers()`; if the backend returns the "APP_KEY differs" error string it surfaces as a rejected promise / error field. Classify:
  - success (any snapshots or a clean status) → `ok` "Your backups are readable".
  - error text contains "APP_KEY differs" → `bad` with the remedy (Step 3 render below).
  - other error → `warn` "Couldn't reach the backups yet — configure the location below, then re-check."
Store `readableState: StepState` + a `lastError` string.

- [ ] **Step 3: Render the card.** Add `<StepCard n={1} title={t("recovery.step1")} state={readableState}>` with: the explanation (needs the same APP_KEY, from the recovery kit, set in the Unraid template), a "Re-check" button calling `checkReadable()`, and — when `bad` — the exact remedy text `t("recovery.appKeyRemedy")`.

- [ ] **Step 4: i18n.** en+de: `recovery.step1` ("Can BombVault read your backups?"), `recovery.appKeyExplain` (needs the original APP_KEY from the recovery kit, set in the Unraid container template), `recovery.appKeyRemedy` ("The encryption key doesn't match these backups. Set the original APP_KEY (from your recovery kit) in the container template, then re-check."), `recovery.readable` / `recovery.notReachable`, `recovery.recheck`.

- [ ] **Step 5: Gate + commit.** Gate; commit `feat: recovery step 1 — repo-readable / APP_KEY check`. Manual check: on a readable repo the pill is green; simulate a bad key (or unreachable path in Settings) → the remedy shows.

---

### Task 3: Step card — attach your backups (consolidated, cloud creds un-gated)

**Files:** Modify `web/src/pages/Recovery.tsx`; reuse existing cred inputs. `web/src/lib/i18n.ts`.

**Interfaces:**
- Consumes: `getSettings`, `putSettings`, `setCloud`, `setRclone`, existing FolderBrowser + cloud/rclone input patterns (see `Settings.tsx` Paths card ~1356, CloudCard ~441, RcloneCard ~359).
- Produces: a "Connect & preview" that re-runs the Task-2 `checkReadable()` after saving.

- [ ] **Step 1: Attach panel.** Add `<StepCard n={2} title={t("recovery.step2")}>` containing, in one place (NOT advanced-gated here): the local backup path field(s) (reuse FolderBrowser bound to `containersPath`/`vmsPath`/`flashPath` via the existing settings save), the off-site repo URL field, the cloud-credential fields (reuse the CloudCard field set) and the rclone-config field (reuse RcloneCard's textarea), and the encryption on/off toggle. Persist via the SAME `putSettings`/`setCloud`/`setRclone` the Settings page uses (import and reuse; do not add new endpoints or duplicate state — read the existing save/merge pattern and follow it).

- [ ] **Step 2: Connect & preview.** A button that saves the panel's values then calls the Task-2 `checkReadable()` and shows the result inline ("Found snapshots — your backups are attached" or the error). This ties Step 2 back to Step 1's readable state.

- [ ] **Step 3: i18n.** en+de: `recovery.step2` ("Attach your backups"), `recovery.attachHint` (point at the local path, or an off-site repo — rest/S3/B2/sftp/rclone — with its credentials), `recovery.connectPreview` ("Connect & preview"), plus any field labels not already keyed (reuse existing `paths.*`/`cloud.*`/`offsite.*` keys where they exist).

- [ ] **Step 4: Gate + commit.** Gate; commit `feat: recovery step 2 — consolidated attach-backups panel`. Manual check: setting a path/off-site here persists (visible in Settings too) and Connect updates Step 1's pill.

---

### Task 4: Step card — discover everything

**Files:** Modify `web/src/lib/api.ts` (`discoverAll` helper), `web/src/pages/Recovery.tsx`, `web/src/lib/i18n.ts`.

**Interfaces:**
- Consumes: `discover`, `discoverVMs`, `listContainers`, `listVMs`.
- Produces: `discoverAll(): Promise<{ containers: number; vms: number }>` in api.ts; a `discovered` result on the page used by Task 5.

- [ ] **Step 1: `discoverAll` helper.** In `web/src/lib/api.ts`:
```ts
// Runs both domain discovers (rebuild targets from the backup defs) and returns the counts.
export async function discoverAll(): Promise<{ containers: number; vms: number }> {
  const [c, v] = await Promise.all([discover(), discoverVMs()]);
  return { containers: c.discovered ?? 0, vms: v.discovered ?? 0 };
}
```

- [ ] **Step 2: Discover card.** Add `<StepCard n={3} title={t("recovery.step3")}>` with a "Discover backups" button that calls `discoverAll()`, then re-fetches `listContainers()` + `listVMs()` (store the lists for Task 5), and reports `t("recovery.foundCounts")` ("Found {c} containers and {v} VMs in your backups"). Handle `0/0`: show `t("recovery.foundNone")` pointing back to Step 1/2 (repo not attached / wrong key / empty).

- [ ] **Step 3: i18n.** en+de: `recovery.step3` ("Discover what's in your backups"), `recovery.discover` ("Discover backups"), `recovery.foundCounts` ("Found {c} containers and {v} VMs."), `recovery.foundNone` ("Nothing found yet — check the connection and attachment above.").

- [ ] **Step 4: Gate + commit.** Gate; commit `feat: recovery step 3 — discover all (containers + VMs)`. Manual check: against a real repo, Discover reports non-zero counts and populates the lists.

---

### Task 5: Step card — review & restore all (left-stopped)

**Files:** Modify `web/src/pages/Recovery.tsx`, `web/src/lib/i18n.ts`.

**Interfaces:**
- Consumes: the discovered `Container[]` / `VM[]` from Task 4; `restore`/`restoreVM`; `fireAndWaitRun` (backupWatch.ts:304) for the sequential bulk; the v4 restore-UX progress via `useProgress()`.
- Produces: per-item + "Restore all" actions.

- [ ] **Step 1: Review list.** Add `<StepCard n={4} title={t("recovery.step4")}>` listing the discovered containers then VMs, each row: name + latest snapshot (from the list objects) + a per-item **Restore** button. Mirror how `Containers.tsx` triggers a single restore (read `restoreSelected`/the single-restore call + `RestorePanel` usage) so the same async fire-and-watch + progress bar applies. For VMs, if none of them have the libvirt SSH configured, show `t("recovery.vmSshNote")` (a note, not a block) pointing to Settings → VM Backup over SSH.

- [ ] **Step 2: Restore all.** A **Restore all** button that restores every discovered container then VM **sequentially** and **left stopped** (pass the leave-stopped flag the restore call already supports — see `Containers.tsx` restore usage), reusing `fireAndWaitRun` exactly as `restoreSelected` does (fire/retry on the single-flight busy, wait per item), accumulating an ok/fail count shown as `t("recovery.restoreAllResult")`. Disable it while `anyActive(progress)` (reuse the v4 helper) so it can't collide with a running op.

- [ ] **Step 3: i18n.** en+de: `recovery.step4` ("Review and restore"), `recovery.restoreAll` ("Restore all (left stopped)"), `recovery.restoreAllResult` ("Restored {ok}, failed {fail}. Start them from the Containers/VMs tabs when ready."), `recovery.vmSshNote` ("VM restore needs the libvirt SSH link — set it up under Settings → VM Backup over SSH."), `recovery.noneDiscovered` ("Run Discover above first.").

- [ ] **Step 4: Gate + commit.** Gate; commit `feat: recovery step 4 — review + restore-all (left stopped)`. Manual check: restore-all runs items one at a time with progress, leaves them stopped, and reports the count.

---

### Task 6: Step card — recovery kit

**Files:** Modify `web/src/pages/Recovery.tsx`, `web/src/lib/i18n.ts`.

**Interfaces:** Consumes `recoveryKitUrl()` (api.ts:724).

- [ ] **Step 1: Kit card.** Add `<StepCard n={5} title={t("recovery.step5")}>` with a short "your safety net for next time" explanation and a download link/button to `recoveryKitUrl()` (an `<a href={recoveryKitUrl()} download>` styled as a button, mirroring the Settings → Encryption card link ~`Settings.tsx:1693`).

- [ ] **Step 2: i18n.** en+de: `recovery.step5` ("Your recovery kit"), `recovery.kitHint` ("Download and store your recovery kit somewhere safe — it holds the encryption key and the exact restic commands to restore even without BombVault."), `recovery.kitDownload` ("Download recovery kit").

- [ ] **Step 3: Gate + commit.** Gate; commit `feat: recovery step 5 — surface the recovery kit`. Manual check: the download link fetches the kit.

---

### Task 7: Fresh-install nudge on the Dashboard

**Files:** Create `web/src/lib/freshInstall.ts` (pure predicate), modify `web/src/pages/Dashboard.tsx`, `web/src/lib/i18n.ts`.

**Interfaces:**
- Consumes: `Container[]`/`VM[]` (via existing dashboard fetches or `listContainers`/`listVMs`), `DomainStatus[]` (`getStatus`).
- Produces: `isFreshInstall(containers, vms, domains): boolean`.

- [ ] **Step 1: Predicate.** Create `web/src/lib/freshInstall.ts`:
```ts
import type { Container, VM, DomainStatus } from "./api";
// Fresh = nothing to show yet: no known targets AND no domain has ever backed up successfully.
export function isFreshInstall(containers: Container[], vms: VM[], domains: DomainStatus[]): boolean {
  const noTargets = containers.length === 0 && vms.length === 0;
  const neverBacked = domains.every((d) => d.status === "off" || d.status === "never");
  return noTargets && neverBacked;
}
```

- [ ] **Step 2: Nudge card.** In `Dashboard.tsx`, when `isFreshInstall(...)` is true AND the nudge isn't dismissed (`localStorage["bombvault.recoveryNudgeDismissed"]`), render a dismissible card near the top: text `t("recovery.freshNudge")` + a Link to `/recovery` (`t("recovery.freshNudgeCta")`) + a dismiss "x" that sets the localStorage flag. Reuse the existing dashboard card styling.

- [ ] **Step 3: i18n.** en+de: `recovery.freshNudge` ("New or rebuilt install? Recover your existing backups."), `recovery.freshNudgeCta` ("Go to Recovery").

- [ ] **Step 4: Gate + commit.** Gate; commit `feat: fresh-install nudge linking to Recovery`. Manual check: on an empty install the nudge shows and links to /recovery; dismiss persists; a configured install doesn't show it.

---

### Task 8: Close-out — i18n ×24 + final gate + docs

**Files:** `web/src/lib/locales/*.ts` (24), `README.md`.

- [ ] **Step 1:** Dispatch the 24-locale translation for every new `recovery.*` / `nav.recovery` key added in Tasks 1-7 (single batch).
- [ ] **Step 2:** Full re-gate: `cd web && npx tsc --noEmit && npm run build`, revert `web/dist/index.html`; confirm `go build ./...` still passes (no Go changed).
- [ ] **Step 3:** README: add a short "Guided recovery" bullet under Restore (a Recovery tab that walks a fresh install through attach → discover → restore). Commit `i18n+docs: recovery-tab strings in all locales + feature note`.

---

## Self-review notes

Spec coverage: Step 1 connection/APP_KEY (Task 2), Step 2 attach (Task 3), Step 3 discover-all (Task 4), Step 4 review+restore-all left-stopped (Task 5), Step 5 kit (Task 6), fresh-install nudge (Task 7), tab scaffold/route/nav (Task 1), i18n×24 + docs (Task 8). No backend change (honored). Type consistency: `StepCard`/`StepState` defined in Task 2 and reused in 3-6; `discoverAll` defined Task 4 used Task 4/5; `isFreshInstall` defined + used Task 7. Placeholder note: Task 2 Step 2 and Task 5 Step 1 tell the implementer to READ the existing single-restore + repo-probe usage and mirror it (the exact `restore(...)` argument shape lives in `Containers.tsx`); this is deliberate reuse, not a placeholder — the implementer has the file refs.
