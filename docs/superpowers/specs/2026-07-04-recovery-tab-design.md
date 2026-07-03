# Guided Recovery tab — design

**Source:** GitHub issue #26 (manilx). On a fresh/rebuilt Unraid server, a dedicated tab that hand-holds
disaster recovery: attach the existing backup repo, discover the containers/VMs stored in it, and restore
them, without needing the originals present.

**Branch:** `feat/recovery-tab` (off `main` == v4.0.0). **Stack:** React/Vite SPA over the existing Go API.

**Goal:** one guided "Recovery" flow that sequences the attach → discover → restore steps BombVault already
supports, plus a fresh-install nudge — so recovering onto a new box is a walkthrough, not a scavenger hunt
across Settings, Containers and VMs.

**Key finding (recon):** every backend capability already exists and is API-exposed. This feature is **pure
frontend orchestration glue — no engine changes.** (If a clean "is the repo readable?" probe turns out
better than parsing the existing snapshot-list error, one tiny read-only GET may be added; default is none.)

---

## Existing capabilities this tab orchestrates (recon anchors)

- **Discover:** `POST /api/discover` + `POST /api/vms/discover` (`internal/api/handlers.go:354-374`,
  `Service.Discover` `service.go:2033`, `DiscoverVMs` `service.go:2146`) rebuild the target DB from the
  encrypted mirrored definitions in storage (`bombvault-defs` / `bombvault-vm-defs`, siblings of the repo),
  so containers/VMs become restorable with no live original. Each returns `{"discovered": N}`; the UI then
  re-fetches `listContainers` / `listVMs`. Frontend wrappers: `api.ts:644-650`.
- **Restore-without-original:** `Service.Restore` (`service.go:2233`) / `RestoreVM` (`service.go:3742`);
  `recreateOnly` from the stored definition when no live container exists (`service.go:2302-2347`), replayed
  against the Docker API (containers) / libvirt over SSH (VMs). Already exposed + used by Containers/VMs
  pages. The v4 restore-UX (live progress bar, cancel, busy hints) applies.
- **Attach an existing repo:** repo resolution is pure config (`repoFor` `service.go:4747`,
  `resolveRepo` `service.go:381`); BombVault lists snapshots of a repo it never created
  (`localRepoMissing` only checks a `config` marker; remotes list directly). Reading an encrypted repo
  requires the **original `APP_KEY`** (env-only, `config.go:36-42`; restic password derived via
  `restickey.Derive`); a mismatch surfaces the mapped error "backup repository can't be opened — the
  APP_KEY differs…" (`handlers.go:78-79`).
- **Recovery kit:** `GET /api/recovery-kit` (`service.go:5217`) — APP_KEY + derived restic password + repo
  locations + standalone restic restore commands; download link + Dashboard nag already exist.
- **Frontend structure:** flat routes under one Layout (`web/src/app/router.tsx:11-30`); sidebar NavItems
  (`web/src/components/Sidebar.tsx:262-285`); Advanced-gating via `useAdvanced()` (`web/src/lib/advanced.tsx`).

## What's missing (the glue this feature builds)

No `/recovery` page, no first-run/empty-state flow, no unified "discover everything + show the reconstructed
list + restore all". The pieces are scattered across Settings / Containers / VMs and must be run manually in
the right order. `APP_KEY` isn't surfaced/explained in the UI, and Cloud/Rclone credential fields are
Advanced-gated (a recovering user can't see them without discovering the Advanced toggle first).

---

## The Recovery page (`web/src/pages/Recovery.tsx`)

A single always-visible "Recovery" tab (DR is a core, non-expert flow), laid out as a top-to-bottom guided
sequence of cards. Each card shows a done/incomplete state so the user always sees where they are. Steps:

### Step 1 — Connection check (APP_KEY)
Explains: to read existing backups this container needs the SAME `APP_KEY` as before (it's in the recovery
kit); set it in the Unraid template if it isn't. A live check lists snapshots of the configured repo:
- readable → green "your backups are readable" (shows snapshot count / newest date);
- the "APP_KEY differs" error → red with the exact remedy (set the original APP_KEY in the template, then
  recheck); other/unreachable → connection error + retry.
Implementation: call the existing snapshot list for a configured domain; classify the result. No new endpoint.

### Step 2 — Attach your backups
One consolidated panel aggregating the otherwise-scattered settings needed to point at existing backups:
local backup path(s) (FolderBrowser), OR off-site URL + credentials (rest/s3/b2/sftp/rclone), plus the
encryption on/off toggle. Cloud/Rclone credential fields are shown **un-gated here** (recovery context),
reusing the existing cred components/`setCloud`/`setRclone` without duplicating persistence. A "Connect &
preview" action lists snapshots as proof the attach worked. (Persistence goes through the existing
`putSettings`/`setCloud`/`setRclone`; this panel is a recovery-focused re-surfacing, not new storage.)

### Step 3 — Discover everything
A single button calls `discover` + `discoverVMs`, then re-fetches `listContainers` + `listVMs`, and reports
"Found N containers, M VMs in your backups." Handles the "0 found" case (repo empty / not attached yet /
wrong key) with a pointer back to Step 1/2. (A small `discoverAll()` helper in `api.ts` wraps the two calls.)

### Step 4 — Review & restore
The reconstructed containers + VMs, each with its latest snapshot (from `listContainers`/`listVMs` +
snapshot info). Per-item **Restore**, plus **Restore all** — sequential (the backend is single-flight;
reuse the existing bulk fire-and-wait pattern), **left stopped** by default so nothing races ahead of its
dependencies (the user starts them afterward), with the existing v4 restore progress + cancel surfaced.
Flash is a zip download (no defs to rebuild), noted separately. If VMs are present but the libvirt SSH link
isn't configured, a clear note points to Settings → VM Backup over SSH.

### Step 5 — Recovery kit
Prominently surfaces the recovery-kit download ("your safety net for next time" — the standalone restic
runbook), reusing `recoveryKitUrl()`.

## Fresh-install nudge (Dashboard)
When the install looks fresh — no discovered targets (`listContainers`/`listVMs` empty) AND no successful
backups (`getStatus` all "never"/"off") — show a dismissible Dashboard card: "New or rebuilt install?
Recover your existing backups →" linking to `/recovery`. Client-side detection; dismiss persists per browser.

## Error handling
- APP_KEY mismatch → the mapped guidance (set original key in template, recheck).
- Repo unreachable / wrong path → connection error + retry, without wedging the flow.
- Restore failures → per-item, via the existing restore-UX (cancelled ≠ failed carries over).
- VM restore needs the SSH link → surfaced as a note, not a hard block (container recovery still proceeds).

## Testing
Frontend gate (`tsc --noEmit` + `npm run build`). Testable helpers to unit-ish cover: the fresh-install
detection predicate, and the `discoverAll` orchestration (both endpoints called, lists re-fetched). No
backend tests (no backend change), unless the optional probe endpoint is added (then a Go arg/handler test).

## Out of scope (YAGNI)
- General first-run onboarding for NEW users with no backups (separate theme).
- Making `APP_KEY` UI-editable (it is an env var by security design; the tab guides, it does not set it).
- Any new restore/discover engine behavior (all exists).

## File map
- Create: `web/src/pages/Recovery.tsx` (the guided page; may split step cards into small components under
  `web/src/components/recovery/` if the page grows unwieldy).
- Modify: `web/src/app/router.tsx` (+`/recovery` route), `web/src/components/Sidebar.tsx` (+NavItem, inline
  SVG icon, always-visible), `web/src/pages/Dashboard.tsx` (fresh-install nudge card),
  `web/src/lib/api.ts` (`discoverAll()` wrapper; reuse existing calls otherwise),
  `web/src/lib/i18n.ts` (en+de keys; 24 locales at close-out).
- Backend: none by default.
