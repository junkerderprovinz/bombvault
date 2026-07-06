# BombVault v5.0.0 Redesign — Implementation Spec

Synthesized 2026-07-07 from 7 area audits (dead-code, i18n-master, gating, dashboard, settings, restore, renames-routes). This is the single source of truth for the batched v5.0.0 UX redesign. An implementer who has never seen the code should be able to execute every task below without guessing.

---

## Global constraints

- **Branch:** all work lands on `feat/v5`. Bugfixes stay on `main` (do not mix). Preview image tag `:v5-preview`. **Tag `v5.0.0` only at the very end**, once all phases are done and green.
- **Do NOT rename backend/API/wire identifiers.** Frozen contract identifiers (verified in `web/src/lib/api.ts` + backend routing):
  - Settings field names: `configEnabled`, `configPath`, `configSchedule`, `configOffsite`, `configOffsiteSchedule`, `configOffsiteImmutable`, `containersEnabled`, `vmsEnabled`, `flashEnabled`, `drDrillTarget`, `*Schedule`, `*OffsiteSchedule` (api.ts Settings type ~lines 88–141).
  - API domain string `"config"` in `/api/config/backup`, `/api/config/snapshots`, `/api/config/restore`, `/api/prune/config`, `/api/snapshots/config`, `/api/check|verify|stats` (api.ts:916, 980, 1196–1218).
  - `progressKey:"config"`, `useProgress()["config"]`, `matchRun r.domain==="config"` (Config.tsx:38,41,297); `healthchecksByDomain` key `"config"` (Settings.tsx:647).
  - restore progress domain strings are **singular** `"container"`/`"vm"` (match `run.domain` + SSE key); `deleteSnapshot` uses **plural** `"containers"`/`"vms"`/`"flash"`/`"config"` (api.ts:979–984). Do not cross these.
  - The `jobs.*` i18n key namespace and `/jobs` route are internal identifiers — the user-facing word is "Plans" already; **do not rename the namespace or route** (see renames tasks).
- **i18n sync rule (26 edits per added/removed key):** `web/src/lib/i18n.ts` holds BOTH the English source object `const en` (starts line 39; `export type TranslationKey = keyof typeof en` at line 778) AND an inline German `de` object (starts ~line 785). There are **24** per-language files in `web/src/lib/locales/` (ar cs da el es fi fr he hu it ja ko nl no pl pt ro ru sv th tr uk vi zh) — **the brief's "23" is wrong; glob confirms 24**. German is NOT a file; it is the inline `de` block. So every key **add** = 26 edits (en + de + 24 files); every key **remove** = 26 deletions; a **reword** = change the value in en + de + all 24 (26 files) but touch no code reference. `Partial<Translations>` allows a locale to *omit* a key (English fallback) but NOT to carry an *unknown* key — so a removed key MUST be deleted from all 26, and an added key MUST be present in en (or `t()` fails typecheck).
- **Gates (all must pass before tag):** `tsc` (or `vue-tsc`/`tsc --noEmit` as configured) + `vite build` + `go build ./...` + `gofmt -l` clean + i18n key-count parity across all 26 locales.

---

## i18n master change-list

The single deduped source of truth for every key touched. `reword` = value-only (no code/key change, 26 files). `add` = new key in all 26. `rename-from:X` = change key string in all 26 AND update code references. `remove` = delete from all 26. EN line numbers are the current location in `web/src/lib/i18n.ts` (verified 2026-07-07); DE line in parentheses.

| Phase | Key | Action | EN string | Current EN line / DE line |
|---|---|---|---|---|
| 1 | `nav.config` | reword | `Self-Backup` | 51 / 795 |
| 1 | `config.title` | reword | `Self-Backup` | 665 / 1402 |
| 1 | `config.settingsTitle` | reword | `Self-Backup settings` | 667 / 1404 |
| 1 | `dashboard.domainConfig` | reword | `Self-Backup` | 487 / 1224 |
| 1 | `settings.configEnabled` | reword | `Self-Backup` | 306 / 1043 |
| 1 | `dashboard.statActiveJobs` | reword | `Active plans` | 471 / 1208 |
| 1 | `dashboard.statPausedJobs` | reword | `Paused plans` | 472 / 1209 |
| 1 | `flash.zipExport.keepHistory` | reword | `Keep exported zip files` | 655 / 1392 |
| 1 | `settings.retentionTitle` | reword | `Snapshot retention` | 315 / 1052 |
| 1 | `dashboard.subtitle` | add | `Your backup status at a glance.` | new (insert after `dashboard.title` ~L58) |
| 1 | `mode.advanced` | rename-from:`nav.advanced` | `Advanced` | 54 / 798 (keep value; new key string `mode.advanced`) |
| 1 | `mode.simple` | add | `Simple` | new (place beside `mode.advanced`) |
| 1 | `mode.hint` | add | `Advanced reveals expert controls` | new (place beside `mode.advanced`) |
| 1 | `nav.comingSoon` | remove | — (delete: EN `Coming soon`, DE `Demnächst`) | 55 / 799 |
| 2 | `settings.tab.general` | add | `General` | new |
| 2 | `settings.tab.storage` | add | `Paths & Storage` | new |
| 2 | `settings.tab.schedules` | add | `Schedules` | new |
| 2 | `settings.tab.offsite` | add | `Off-site` | new |
| 2 | `settings.tab.notifications` | add | `Notifications` | new |
| 2 | `settings.tab.integrity` | add | `Integrity` | new |
| 2 | `settings.tab.system` | add | `System` | new |
| 2 | `settings.schedulesBackup` | add | `Backup schedules` | new |
| 2 | `settings.schedulesOffsite` | add | `Off-site replication schedules` | new |
| 2 | `settings.schedulesSelfBackup` | add | `Self-backup schedule` | new |
| 2 | `settings.schedulesChecks` | add | `Restore-check schedule` | new |
| 2 | `settings.subtitle` | add | `BombVault configuration — changes take effect immediately.` | new |
| 2 | `filter.button` | add | `Filters` | new |
| 3 | `offsite.sectionTitle` | add | `Off-site protection` | new |
| 3 | `dashboard.summaryHealth` | add | `Overall health` | new |
| 3 | `dashboard.summaryNextBackup` | add | `Next backup` | new |
| 3 | `dashboard.summaryLastResult` | add | `Last result` | new |
| 3 | `restore.completeContainer` | add | `Restore complete — container is being recreated.` | new |
| 3 | `restore.completeVM` | add | `Restore complete — VM disks have been replaced.` | new |
| 3 | `restore.recreateComplete` | add | `Recreate complete — the container has been recreated.` | new |

**Total distinct keys touched: 34** (Phase 1: 14 · Phase 2: 13 · Phase 3: 7).

**DE values (from convention "Selbst-Backup"):** `nav.config`/`config.title`/`dashboard.domainConfig`/`settings.configEnabled` = `Selbst-Backup`; `config.settingsTitle` = `Selbst-Backup-Einstellungen`; `dashboard.statActiveJobs` = `Aktive Pläne`; `dashboard.statPausedJobs` = `Pausierte Pläne`; `flash.zipExport.keepHistory` = `Exportierte ZIP-Dateien behalten`; `settings.retentionTitle` = `Snapshot-Aufbewahrung`; `mode.advanced` = `Erweitert` (existing value); `mode.simple` = `Einfach`; `mode.hint` = `Erweitert zeigt Expertenoptionen`. All other adds: translate all 24 locales in the same pass (translate-all-locales-immediately convention).

**Decisions locked (were conflicts across audits):**
- `dashboard.subtitle` — two candidates existed: `Your backup status at a glance.` (i18n-master, domain-neutral) vs `Backup status for your containers, VMs, and flash drive.` (dashboard agent, but omits self-backup). **Locked: the domain-neutral `Your backup status at a glance.`** because the enumerated version is incomplete (BombVault also covers self-backup).
- `mode.hint` — `Advanced reveals expert controls` (gating agent, fits one line) chosen over the longer `Advanced shows expert controls and extra settings; Simple hides them.` (i18n-master).
- `nav.advanced` → `mode.advanced` **rename adopted** (i18n-master's recommendation) rather than keeping `nav.advanced` (gating's minimal-churn option), because a v5 major is the right time to unify Simple/Advanced under one `mode.*` namespace — the split namespace is the exact inconsistency item (d) targets. Cost: update the sole code reference at Sidebar.tsx:340 + all 26 locales.
- Settings tab keys — used the **settings-area owner's** 7-tab set (`general/storage/schedules/offsite/notifications/integrity/system`); discarded i18n-master's speculative `backups/retention/security/maintenance` scaffold.
- Dashboard summary keys — used the **dashboard-area owner's** `summaryHealth/summaryNextBackup/summaryLastResult` (concrete 3-cell tier); i18n-master's `dashboard.showDetails/hideDetails` are NOT added (that design is a disclosure toggle; the chosen design is a persistent summary tier). Revisit only if the detail tier becomes a toggle.
- `restore.dialogTitle` (i18n-master anticipatory) **dropped** — the restore-area owner's component API needs no neutral dialog title; adding it would orphan a key.

---

## Phase 1 — tasks

Ordered so shared foundation (i18n batch) lands first, then per-file work. Every task that adds/rewords keys depends on Task 1.0 sync discipline.

### 1.0 — i18n foundation batch (do FIRST)
- **Files:** `web/src/lib/i18n.ts` + all 24 `web/src/lib/locales/*.ts`.
- **Change:** Apply every **Phase-1 row** of the master change-list in one pass: the 9 rewords (config→Self-Backup ×5, plans ×2, keep-history ×2), the `dashboard.subtitle` add, the `nav.advanced`→`mode.advanced` rename + `mode.simple`/`mode.hint` adds, and the `nav.comingSoon` removal. Keep en + de in i18n.ts and all 24 files in lockstep. After this, `tsc` must be clean (the rename's code reference at Sidebar.tsx:340 is updated in Task 1.4; sequence 1.0 → 1.4 in the same commit or expect one transient tsc error on the rename).
- **i18n keys:** all 14 Phase-1 rows.
- **Risk:** `nav.advanced`→`mode.advanced` and `nav.comingSoon` removal must hit all 26 files or `tsc` fails (unknown-key under `Partial`). Reword rows carry zero code risk. Do the rename + its Sidebar.tsx:340 edit atomically.

### 1.1 — Delete the orphaned TopBar.tsx component
- **Files:** `web/src/components/TopBar.tsx` (delete whole file, 158 lines).
- **Change:** Remove the entire file (`TopBar`, `IconSun`, `IconMoon`, `Flag`, `LanguageSwitcher`). Evidence dead: Grep `import.*TopBar|<TopBar` across `web/src` → zero matches; the only importer-shell `web/src/app/Layout.tsx` renders `<Sidebar settings={settings} />` (line 73) and imports only Sidebar. The language switcher + theme toggle it provided are duplicated live in `Sidebar.tsx` `SidebarControls` (lines 160–261). No barrel re-export.
- **i18n:** none (its `t()` keys `language.label`, `theme.toggle` stay in use by Sidebar).
- **Risk:** none — zero importers, content duplicated. Run `tsc`/`vite build` after to confirm no dangling reference.

### 1.2 — Remove dead NavItem `disabled`/`comingSoon` props in Sidebar.tsx
- **Files:** `web/src/components/Sidebar.tsx`.
- **Change:** No NavItem call site (lines 292–296, 297–301, 303–307, 308–312, 315, 318, 322, 342–346) passes `disabled` or `comingSoon`. (1) Delete interface members lines 16–17 (`disabled?: boolean;`, `comingSoon?: boolean;`) from `interface NavItem`; (2) delete the `navDisabled` const lines 115–116; (3) change signature line 118 from `function NavItem({ to, label, icon, disabled, comingSoon }: NavItem)` to `function NavItem({ to, label, icon }: NavItem)`; (4) delete the entire `if (disabled) { return ( … ); }` block lines 119–133 (this also removes the hard-coded literal `"Coming soon"` at line 128 — not a `t()` key). Do the `nav.comingSoon` key removal (Task 1.0) in the same commit.
- **i18n:** `nav.comingSoon` removed (handled in 1.0).
- **Risk:** self-contained. Remove the branch (119–133) and `navDisabled` const (115–116) together so no dangling reference. **Ordering:** this file is also touched by Task 1.4 (visible switch) and Task 1.0 (mode.advanced reference at line 340) — do all three Sidebar edits in one coordinated pass to avoid line-number drift.

### 1.3 — Replace hardcoded Dashboard subtitle with i18n
- **Files:** `web/src/pages/Dashboard.tsx`.
- **Change:** Lines 1382–1384 currently render the hardcoded literal `<p className="mt-1 text-sm text-carbon-textSub">BombVault — container backup overview</p>`. Replace the literal text with `{t("dashboard.subtitle")}` (keep the same `<p>` + classes).
- **i18n:** `dashboard.subtitle` = `Your backup status at a glance.` (added in 1.0).
- **Risk:** low. Same file is edited by Tasks 1.6 (banner cap) and 1.7 (stat-tile demotion) — coordinate to avoid edit collisions. Removes the only em dash in this JSX.

### 1.4 — Replace hidden Advanced checkbox with a visible Simple/Advanced switch + hint
- **Files:** `web/src/components/Sidebar.tsx`.
- **Change:** Keep `const { advanced, setAdvanced } = useAdvanced();` at line 265 (the new control MUST call the same `setAdvanced` so localStorage `bombvault.advanced` persistence is unchanged — Sidebar is the ONLY writer). Delete the hidden-checkbox block lines 329–341 (the `{/* Advanced mode … */}` comment + the `<label …><input type="checkbox" checked={advanced} onChange={() => setAdvanced(!advanced)} …/><span>{t("nav.advanced")}</span></label>`). In its place, inside the footer `<div className="flex flex-col gap-1 p-3">` (line 327), ABOVE the Settings `<NavItem to="/config" …/>`… actually ABOVE `<NavItem to="/settings" …/>` (line 342), render a 2-segment switch reusing the `SourceToggle.tsx` pattern (lines 25–29): container `<div className="inline-flex rounded-lg border border-carbon-border overflow-hidden w-full">` with two `<button type="button">` segments; active segment `bg-accent text-accentContrast`, inactive `text-carbon-textSub hover:text-carbon-text`. Left = `{t("mode.simple")}` → `onClick={() => setAdvanced(false)}`, active when `!advanced`. Right = `{t("mode.advanced")}` → `onClick={() => setAdvanced(true)}`, active when `advanced`. Directly below add the one-line hint `<span className="px-1 text-[11px] leading-snug text-carbon-textMuted">{t("mode.hint")}</span>`. Optionally extract as a tiny local `ModeSwitch` component (mirrors `SidebarControls`) for Phase-2 relocation; inline is acceptable for Phase 1.
- **i18n:** `mode.simple`, `mode.advanced`, `mode.hint` (from 1.0). Note the right label uses the **renamed** `mode.advanced`, not `nav.advanced`.
- **Risk:** Sidebar is the sole `setAdvanced` writer — a typo silently breaks ALL advanced gating app-wide. Do NOT place the switch in a header: `TopBar.tsx` is deleted (1.1) and `Layout.tsx` renders no header, so a header would mean reviving dead code. Coordinate with Tasks 1.2 + 1.0 (same file).

### 1.5 — Un-gate the notify-on-failure control into Simple mode
- **Files:** `web/src/pages/Settings.tsx`.
- **Change:** The notify-on policy select lives only inside `NotifyCard` (defined 503–740), which is entirely Advanced-gated at line 1894 `{advanced && <NotifyCard t={t} />}`. Surgical split: (1) line 1894 → render unconditionally `<NotifyCard t={t} />`; (2) inside NotifyCard add `const { advanced } = useAdvanced();` near line 504 (`useAdvanced` already imported at Settings.tsx:8); (3) keep ALWAYS visible: Card title + `notify.hint` (567–569), the `notify.on` policy select (571–578, options `notify.onNever`/`onFailure`/`onAlways`), the Unraid-notifications checkbox (580–593), and the Save/Test button row (725–737); (4) wrap in `{advanced && (<>…</>)}` the power-user channels — webhook+Matrix (595–630), Healthchecks + lifecycle hint (632–637), per-domain checks (639–667), SMTP (669–723). Result: Simple users get "notify me on failure via Unraid" (policy + Unraid channel + Save/Test); webhook/Matrix/Healthchecks/SMTP stay Advanced.
- **i18n:** none — all strings exist; `NotifyConfig.on` wire values (`never`/`failure`/`always`) unchanged.
- **Risk:** all fields bind one shared local `cfg` saved via `setNotify()`, so the split is safe (one Save persists everything). Keep the Save button in the Simple set or a policy change can't be saved.

### 1.6 — Cap simultaneously-shown Dashboard banners to at most 1 (Fresh > Recovery)
- **Files:** `web/src/pages/Dashboard.tsx`.
- **Change:** Two page-level banners can co-render today: `FreshInstallNudge` (rendered line 1394) and `RecoveryNag` (rendered line 1397); they overlap on a rebuilt install with encryption on, kit not re-acked, no successful backups. Make at most one show, Fresh winning:
  - (A) Lift Fresh's dismissal into the parent. In `Dashboard()` after the statusDomains state (~line 1351) add `const [freshDismissed, setFreshDismissed] = useState(() => { try { return localStorage.getItem(RECOVERY_NUDGE_DISMISSED) === "1"; } catch { return false; } });`, `const dismissFresh = () => { try { localStorage.setItem(RECOVERY_NUDGE_DISMISSED, "1"); } catch {} setFreshDismissed(true); };`, and `const freshShown = !statusLoading && !freshDismissed && isFreshInstall(statusDomains);`. (`RECOVERY_NUDGE_DISMISSED` is the module const at line 1283, precedes `Dashboard()`.)
  - (B) Make `FreshInstallNudge` (def 1285–1338) controlled: replace its internal `const [dismissed,setDismissed]=useState(...)` (1294–1300) and `dismiss` handler (1307–1314) with props `dismissed: boolean; onDismiss: () => void;`; keep guards `if (dismissed || loading) return null; if (!isFreshInstall(domains)) return null;` (1304–1305); change the ✕ onClick (1329) from `dismiss` to `onDismiss`.
  - (C) Add `suppressed?: boolean` to `RecoveryNag` (def 1215); at the top of its body add `if (suppressed) return null;` BEFORE its existing `if (!settings || !settings.encryptionEnabled || settings.recoveryKitAck) return null;` (1231–1233).
  - (D) Update render sites: line 1394 → `<FreshInstallNudge t={t} domains={statusDomains} loading={statusLoading} dismissed={freshDismissed} onDismiss={dismissFresh} />`; line 1397 → `<RecoveryNag t={t} suppressed={freshShown} />`.
- **i18n:** none.
- **Risk:** FreshInstallNudge def (1285–1338) and its call site (1394) MUST change together or dismissal breaks. RecoveryNag keeps its own `getSettings` fetch. All edits inside Dashboard.tsx — coordinate with 1.3 + 1.7. **Design note (flag to UX owner):** Fresh-over-Recovery is the one genuine choice — defensible because overlap only happens pre-first-backup where the kit protects nothing yet.

### 1.7 — Demote Active/Paused/Missing stat tiles to Advanced-only
- **Files:** `web/src/pages/Dashboard.tsx`.
- **Change:** `StatCardsRow` (def line 82, grid 129–139) shows 7 tiles unconditionally. Keep Containers/VMs/Errors in Simple; gate the four named tiles behind Advanced. (A) Add prop: signature line 82 → also accept `advanced: boolean`. (B) Keep StatCard for `statContainers` (131), `statVMs` (132), `statErrors` (135) always; wrap `statActiveJobs` (133), `statPausedJobs` (134), `statMissingContainers` (136), `statMissingVMs` (137) in `{advanced && ( … )}` (or one `{advanced && (<>…4 cards…</>)}`). (C) Responsive columns: change wrapper className (line 130) from `grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7` to `` grid grid-cols-2 gap-3 ${advanced ? "sm:grid-cols-4 lg:grid-cols-7" : "sm:grid-cols-3"} ``. (D) Call site line 1400 `<StatCardsRow t={t} />` → `<StatCardsRow t={t} advanced={advanced} />` (`advanced` already destructured from `useAdvanced()` at line 1346).
- **i18n:** `dashboard.statActiveJobs`/`statPausedJobs` reworded to "Active plans"/"Paused plans" in 1.0 — this task only gates them.
- **Risk:** low, local. `StatData` interface (49–57) + the compute effect stay unchanged (values just not rendered in Simple). Uses the same `useAdvanced` pattern as existing gates (1408, 1425); migrate to the unified helper in Phase 2 (Task 2.7).

### 1.8 — Config domain → "Self-Backup" (labels only; optional route/file rename DEFERRED)
- **Files (labels):** handled entirely by Task 1.0 (the 5 reword rows). No `.tsx` edits needed — `nav.config` (Sidebar.tsx:322 tab + Settings.tsx:647 healthchecks list), `dashboard.domainConfig` (Dashboard.tsx:984), `config.title` (Config.tsx:332), `config.settingsTitle`, `settings.configEnabled` all just render the reworded value.
- **Optional CODE rename (recommend DEFER — do NOT do in Phase 1 unless the team opts in):** route `/config`→`/self-backup` (router.tsx:24 + Sidebar.tsx:322), rename `Config.tsx`→`SelfBackup.tsx` + `export function Config`→`SelfBackup` (Config.tsx:289) + router.tsx:7 import. No internal deep-links to `/config` except Sidebar; unknown paths soft-redirect to `/dashboard` (router.tsx:28 catch-all).
- **Risk:** MUST NOT rename the KEYS (`nav.config`/`config.*`) or the `configEnabled` settings FIELD — only string VALUES. Do NOT reword the unrelated "Config" strings that mean the container `/config` folder or generic config: `containers.discoverHint`, `snapshots.recreate`/`configOnlyHint`, `rclone.*`, `backup.configOnly`, `excludes.hint`/`placeholder`, `jobs.flashRow` ("Unraid flash config"), `flash.backupHint`, `ransomware.configured`. `recovery.stepConfig`/`config*` values already say "settings" — leave.

### 1.9 — Plans/Jobs naming consistency
- **Files:** i18n (Task 1.0 rewords `dashboard.statActiveJobs`/`statPausedJobs` → "Active plans"/"Paused plans") + optional one-line comment in `web/src/pages/Jobs.tsx`.
- **Change:** User-facing text is ALREADY "Plans" (`nav.jobs`="Plans" L535, `jobs.title`="Plans" L536, `jobs.subtitle`="Backup plans by domain" L537; heading via `t("jobs.title")` Jobs.tsx:489; sidebar `t("nav.jobs")` Sidebar.tsx:299). The only leak was the two Dashboard tiles (fixed in 1.0). Optionally add a one-line comment atop `Jobs.tsx` documenting the intentional mapping "code identifier `jobs` == UI label Plans" so the split is discoverable.
- **i18n:** covered by 1.0.
- **Risk:** do NOT rename the `jobs.*`/`nav.jobs` keys or the `/jobs` route/`Jobs.tsx` file (high-churn, zero user benefit). `jobs.active`/`jobs.paused` ("Active"/"Paused" L542–543) are generic — leave.

### 1.10 — Give the two "keep history" controls distinct labels
- **Files:** i18n (Task 1.0). No `.tsx` edits.
- **Change:** Control A (exported-zip retention, `ToggleRow` at Settings.tsx:1514) uses `flash.zipExport.keepHistory` → reworded "Keep exported zip files". Control B (restic snapshot retention, Card title at Settings.tsx:1661) uses `settings.retentionTitle` → reworded "Snapshot retention". Keys already distinct → zero code/reference risk. `flash.zipExport.keepHistoryHint` (L656) already explains the difference; leave. Config.tsx has NO zip-export/retention control (verified) — unaffected.
- **i18n:** covered by 1.0.
- **Risk:** none beyond the standard 26-file reword.

### 1.11 — NON-DELETIONS (guard rails — do NOT delete these)
- **`web/src/components/SpikePanel.tsx` — KEEP.** **This overrides the settings audit's "Spike DEAD, remove."** Verified 2026-07-07: imported `Settings.tsx:9`, rendered `Settings.tsx:1901` (`<SpikePanel t={t} />`), calls `runSpike()`→`POST /api/spike` (api.ts:1001), also consumed by `Dashboard.tsx:209` (`getSpike()`), backend implements it, README documents the host-integration check. Deleting it breaks the Settings host-integration panel and leaves dangling imports. Its keys (`spike.checkNow`, `spike.allOk`, `spike.degraded`, `spike.colCheck`, `spike.colStatus`, `spike.colDetail`, `spike.bestEffort`) stay. **The Settings 7-tab plan (Task 2.1) must place SpikePanel on the System tab, NOT delete it.**
- **Containers "installed" filter — KEEP as functional; removal is Phase-2 UX, not dead code.** `FilterControl` (Containers.tsx def 155–187, rendered 1440) with `FilterKey='all'|'installed'|'notInstalled'` (L145) is fully wired: gates the live/orphans sections (`filterKey!=='notInstalled'` L1251/1531, `filterKey!=='installed'` L1252/1547), select-all (L1462), no-match compute (L1253), persisted to localStorage (`FILTER_STORAGE_KEY`, handleFilterChange 1217–1220). If the redesign still wants it gone, that is deliberate behavior change owned by the Phase-2 filter-popover work (Task 2.6), with its keys.
- **No dead Plans/flash editor exists.** Full reads of Jobs.tsx (556 lines) + Flash.tsx (292 lines) show no commented-out/unreachable editor. Jobs.tsx `FlashSection` (256–301) renders a live CadenceBuilder + intentional "planned" placeholder (`jobs.flashPlanned`/`flashRow`/`flashNotImplemented`) — a deliberate UI marker, not dead code.
- **Optional micro-cleanup (skip if minimizing churn):** Jobs.tsx `ScheduleStatus` union includes `'paused'` (L33) + `cls['paused']` (L49) but `scheduleStatus()` (35–38) only returns `'active'`/`'off'` — the `paused` path is unreachable. If tidying: narrow L33 to `type ScheduleStatus = "active" | "off";` and drop the `paused:` L49 entry. Self-contained, cosmetic.

---

## Phase 2 — tasks

### 2.1 — Settings: add 7-tab scaffolding, keep single state owner
- **Files:** `web/src/pages/Settings.tsx`.
- **Change:** In `SettingsPage` add `const [tab,setTab]=useState<TabKey>('general')` with `TabKey = 'general'|'storage'|'schedules'|'offsite'|'notifications'|'integrity'|'system'`. After the heading (1350–1357) render a segmented tab bar reusing the existing inline-flex/rounded-lg/border/overflow-hidden pill pattern (already used for the drill-kind toggle, Settings 1006–1031). Wrap each existing card group in `{tab==='X' && (…)}`. Do NOT move `settings`/`savedSettings`/`save`/`hostMountRoot`/save-state hooks — they stay in `SettingsPage` so all tabs share one instance and the baseline-merge `save()` (1258–1292, dispatches window `bv:settings-changed` at 1280) is preserved. **Tab→card mapping (function-oriented; lower risk than domain-oriented):**
  - **general** = Domains card (1362–1416) + Appearance (2005–2057) + language selector.
  - **storage** = Backup paths (1421–1479) + Flash zip export (1485–1567) + Encryption (1831–1875).
  - **schedules** = unified editor (Task 2.3).
  - **offsite** = Off-site repos+wizard (1574–1656, minus schedule inputs after 2.3) + Retention (1661–1734) + Bandwidth (1739–1780) + Rclone (1887) + Cloud (1889).
  - **notifications** = NotifyCard (always visible after Task 1.5).
  - **integrity** = IntegrityCard (1911).
  - **system** = Security (1916–2000) + VMSSHCard (1882) + Monitoring/Prometheus (1785–1826) + **SpikePanel (1901, KEEP per 1.11)** + AboutFooter (2060).
- **i18n:** `settings.tab.{general,storage,schedules,offsite,notifications,integrity,system}` (7 adds).
- **Risk:** ~900 lines of JSX to wrap; must not reparent `OffsiteWizard`/`IntegrityCard` props (`settings`,`setSettings`,`save`). Verify each `SaveBar` still binds its own save-state hook. **SpikePanel goes on System — do not drop it.**

### 2.2 — Settings: extract per-tab render helpers, keep exports live
- **Files:** `web/src/pages/Settings.tsx` (+ possibly new `web/src/components/settings-shared.tsx`).
- **Change:** Extract each tab's card block into a local component (`GeneralTab`, `StorageTab`, `SchedulesTab`, `OffsiteTab`, `IntegrityTab`, `SystemTab`) receiving `{settings,setSettings,save,t,advanced,hostMountRoot}` + relevant save-state props (or move each tab's independent save-state `useState` into its component). **Keep exported** `ToggleRow` (75–113), `RcloneCard` (322–388), `CloudCard` (393–478).
- **Risk:** `ToggleRow`/`RcloneCard`/`CloudCard` are imported by `Jobs.tsx` (:6), `Config.tsx` (:15), `Recovery.tsx` (:6). Moving them out breaks 3 files — either keep them exported from Settings.tsx OR move to `web/src/components/settings-shared.tsx` and update all 3 import sites in the SAME commit.

### 2.3 — Build the unified Schedules tab (every cadence via CadenceBuilder)
- **Files:** `web/src/pages/Settings.tsx`, `web/src/components/CadenceBuilder.tsx`.
- **Change:** Create the Schedules tab with grouped `CadenceBuilder` editors, replacing raw text inputs: (a) Backup schedules `containersSchedule`/`vmsSchedule`/`flashSchedule` (reuse Jobs sync-checkbox); (b) Off-site replication `containersOffsiteSchedule`/`vmsOffsiteSchedule`/`flashOffsiteSchedule` (raw input Settings 1622–1630) + `configOffsiteSchedule` (Config 183–188); (c) Self-backup `configSchedule` (Config 167–173); (d) Restore-check `drillsSchedule` + `drillsSubsetPct` + `drillsEnabled`/`offsiteDrillsEnabled` (Jobs `RestoreChecksSection` 309–361). Each writes via `setSettings`; one `SaveBar` persists all schedule fields via `save()`. Remove the raw schedule input at Settings 1622–1630 from the off-site card. Surface `tamperTestSchedule` here too (currently has NO editor; server default weekly Sun 04:30).
- **i18n:** `settings.schedulesBackup`, `settings.schedulesOffsite`, `settings.schedulesSelfBackup`, `settings.schedulesChecks`.
- **Risk:** `CadenceBuilder.parseCadenceString` falls back to `'off'` for unrecognized strings — off-site/config schedules accept free text; confirm stored values match the grammar or the first edit silently resets to `off`. **Jobs.tsx edits the same containers/vms/flash fields — pick ONE owner (see 2.4).**

### 2.4 — Retire/redirect the Plans page (Jobs.tsx) into the Schedules tab
- **Files:** `web/src/pages/Jobs.tsx`, `web/src/app/router.tsx`, `web/src/components/Sidebar.tsx`, `web/src/pages/Config.tsx`.
- **Change:** Make the Settings Schedules tab the canonical schedule home, then either (A) delete Jobs.tsx + its `/jobs` route (router.tsx:9,26) + Sidebar NavItem (297–301) and redirect `/jobs`→`/settings#schedules`, or (B) keep Plans as a read-only overview linking to the tab. Remove `configSchedule` (Config 167–173) and `configOffsiteSchedule` (Config 183–188) from Config.tsx `ConfigSettingsCard` once they live in the Schedules tab (leaving Config with path/offsite-repo/immutable only).
- **Risk:** **CROSS-AREA** — Jobs is likely its own audit area (not among the 7 provided); coordinate so schedules are not owned in two places. Jobs.tsx `save`/`savedSettings`/`syncSchedules` logic (367–482) incl. the sync-mirror effect (422–434) must be reproduced faithfully in the Schedules tab or backups regress.

### 2.5 — Replace #offsite scroll deep-link with tab selection
- **Files:** `web/src/pages/Settings.tsx`, `web/src/pages/Dashboard.tsx`.
- **Change:** The deep-link effect (Settings 1235–1248) scrolls to `div id=offsite`. Replace: on mount, if `window.location.hash` matches a known tab (`#offsite`, `#schedules`, …), `setTab` to it. Keep `Dashboard.tsx:705` (`Link to /settings#offsite`) working by mapping `#offsite`→offsite tab. Remove the now-unused `id=offsite` wrapper (1574) or keep for back-compat.
- **Risk:** hash is read once on mount; an in-app `#offsite` link while already on `/settings` won't remount — add a `hashchange`/location listener if live deep-link tab switching is required.

### 2.6 — Move Containers/VMs top controls into a filter popover
- **Files:** `web/src/pages/Containers.tsx`, `web/src/pages/VMs.tsx`.
- **Change:** Move the top controls (including the search/status filters and, per the redesign, the Containers "installed" filter from Task 1.11) into a popover triggered by a `Filters` button. Reuse the existing `filter.*` value keys (`filter.all`/`scheduled`/`notScheduled`/`backedUp`/`neverBackedUp`, i18n L525–532) inside the popover. If the redesign drops the redundant Containers installed-filter here, remove `containers.filter`/`filterAll`/`filterInstalled` keys (L105–107) in the same pass and delete the two-section split logic (Containers 1251–1253/1531–1547).
- **i18n:** `filter.button` = "Filters". Coordinate the possible removal of `containers.filter*` with the Containers area before locking.
- **Risk:** the installed-filter drives a real two-section render; deleting it changes what's shown — confirm the UX decision before removing.

### 2.7 — Add the unified `<Advanced>` gating helper and adopt it everywhere
- **Files:** `web/src/lib/advanced.tsx` (define) + `web/src/pages/{VMs,Containers,Settings,Dashboard}.tsx` + `web/src/components/RestorePanel.tsx` (adopt).
- **Change (define):** Keep `AdvancedProvider` + `useAdvanced` + `KEY="bombvault.advanced"` exactly as-is (main.tsx depends on the Provider). Append:
  ```tsx
  export function Advanced({ when = true, children }: { when?: boolean; children: ReactNode }) {
    const { advanced } = useAdvanced();
    return advanced && when ? <>{children}</> : null;
  }
  ```
  `ReactNode` is already imported; the fragment needs no new import.
- **Change (adopt):** In each file add `Advanced` to the existing `import { useAdvanced } from "../lib/advanced"`. Convert the **14 pure** `{advanced && <X/>}` sites to `<Advanced><X/></Advanced>`: VMs 379, 627, 1146; Containers 875, 1496; Settings 1739, 1785, 1887, 1889, 1894*, 1899; Dashboard 1408, 1425 (+ the new StatCardsRow tile gate from 1.7). *Settings 1894 is un-gated entirely by Task 1.5, not migrated. Convert the **2 compound-AND** sites via `when`: Containers 888 `{installed && advanced && (…)}` → `<Advanced when={installed}>…</Advanced>`; RestorePanel 1139 `{advanced && !loading && !error && snapshots.length >= 2 && (…)}` → `<Advanced when={!loading && !error && snapshots.length >= 2}>…</Advanced>`. **LEAVE on the hook** (cannot use the component): Settings 976 `...(advanced ? [prune] : [])` (array-spread), Settings 1882 `(advanced || settings.vmsEnabled) && …` (OR), RestorePanel 819 `const effectiveMode = advanced ? mode : "inPlace"` (value logic). After migrating, delete any now-unused `const { advanced } = useAdvanced();` (VMs 346/560/876; RestorePanel 1040) to avoid no-unused-locals; KEEP the reads still feeding logic cases (Settings 862→976, Settings 1159→1882, RestorePanel 793→819). RestorePanel.tsx and Settings.tsx each hold TWO independent hook reads — track separately.
- **i18n:** none.
- **Risk:** **Ordering — define `<Advanced>` BEFORE the migration.** Additive/non-breaking: hook + component coexist, so migrate incrementally, `tsc`/build after each file. `<Advanced>` cannot express OR or value-logic — do not force those cases into it.

### 2.8 — Remove OffsiteWizard inline schedule step (single schedule owner)
- **Files:** `web/src/components/OffsiteWizard.tsx`.
- **Change:** Wizard Step 3 edits the off-site schedule via a raw input (404–411) bound to `SCHED_KEY`. Once the Schedules tab owns all cadences (2.3), remove that input (+ `patchSched` 184–186 + `schedKey` from the Step-3 save at 417) or replace with a pointer to the Schedules tab. Keep repo URL + credentials + immutable + retention steps intact.
- **Risk:** wizard save (416–421) persists `{repoKey,schedKey}` together; `save()` merges onto `savedSettings` so removing `schedKey` won't clobber the Schedules-tab value — but verify.

### 2.9 — Fix hardcoded English strings exposed by the Settings tab split
- **Files:** `web/src/pages/Settings.tsx`, `web/src/lib/i18n.ts`.
- **Change:** Several Settings strings are raw literals: page subtitle (1354–1356), Domains hint + 4 row descriptions (1364–1365, 1369, 1377, 1385, 1393), Backup-paths hint (1422–1426), metrics `GET /metrics` (1790). Convert to `t()` keys; at minimum add `settings.subtitle` for the heading subtitle. (Row-description keys can be added incrementally; only `settings.subtitle` is in the master list — add the rest as `settings.domainsHint` etc. if surfaced.)
- **i18n:** `settings.subtitle` = "BombVault configuration — changes take effect immediately."
- **Risk:** adds keys → all 24 locales; batch with the tab-key additions (2.1) to avoid two translation passes.

---

## Phase 3 — tasks

### 3.1 — Consolidate the ~5 off-site cards into one section
- **Files:** `web/src/pages/Settings.tsx`, `web/src/components/OffsiteWizard.tsx`.
- **Change:** Merge the scattered off-site cards (`settings.offsiteTitle` L244, `rclone.title` L338, `cloud.title` L342, `offsite.wizard.*`, `offsite.retention.*`, `ransomware.*` dashboard card) under one section header. Existing per-card titles remain as sub-headings within the consolidated section.
- **i18n:** `offsite.sectionTitle` = "Off-site protection". (Only ADD the header — do NOT reword the safety-critical append-only/ransomware strings without the off-site owner's sign-off; no dedicated off-site audit was provided, so treat this as anticipatory.)
- **Risk:** off-site strings are the largest/most safety-critical cluster. Confirm which titles collapse with the off-site area before finalizing.

### 3.2 — Two-tier Dashboard (summary tier above detail tier)
- **Files:** `web/src/pages/Dashboard.tsx`, `web/src/lib/api.ts` (+ backend), `web/src/lib/i18n.ts`.
- **Change:** Insert a compact SUMMARY tier after the heading block (after line 1390, before the banners); everything from `StatCardsRow` (1400) down becomes the DETAIL tier (unchanged except Phase-1 gating). Summary = a 3-cell row reusing StatCard/Card visual language:
  - **Cell 1 Overall health** — worst RPO status across enabled non-off domains (any `{overdue,never}`→red, else any `warn`→amber, else any `ok`→green, else all off→neutral); render via existing `StatusChip` + `chipForRpo` (146–166 / 278–290), reuse `rpo*` labels (`rpoOverdue/rpoWarn/rpoOk/rpoOff`).
  - **Cell 2 Next backup** — soonest upcoming scheduled run across enabled domains. **HARD BACKEND DEPENDENCY:** no next-run timestamp exists (`DomainStatus`, api.ts 178–213, has `schedule`/`periodSeconds` but no next-run). Add backend field `nextRun: number` (unix secs; 0=none) to `DomainStatus`, populate from the scheduler; cell shows `min(nextRun>0)` via `relativeTime(t, …)` with `formatTs` title, falling back to `formatCadence(soonest)` if all zero.
  - **Cell 3 Last result** — newest run from `listRuns()` (`runs[0]`, newest-first): `StatusChip(run.status)` + target + `relativeTime(run.startedAt)`; empty state reuses `dashboard.noRuns`.
  Reuse the existing `statusDomains` fetch (1352–1373) for cells 1–2; for cell 3 either lift `RunsCard`'s `listRuns` fetch to the parent or fetch once in the parent and share.
- **i18n:** `dashboard.summaryHealth`, `dashboard.summaryNextBackup`, `dashboard.summaryLastResult`.
- **Risk:** **Cell 2 blocked on the backend `nextRun` addition** — cannot be built client-only (no cron next-run calculator exists; `formatCadence` renders text, not a timestamp). Lifting `RunsCard`'s fetch couples it to the parent; alternatively fetch `listRuns` twice (one extra request). `max-w-5xl` container (1376) + the 6-gap flex column accommodate a new row without structural change.

### 3.3 — Shared Restore component family (RestoreProgress → RestoreAction → SnapshotListShell)
Restore UI is hand-copied per page. Three-tier extraction; do them in order (3.3a is a prerequisite for 3.3b). **Preserve the three hardcoded English success strings verbatim by moving them into i18n** (repo English, em dash allowed).

#### 3.3a — Extract `RestoreProgress` banner primitive (highest-count dedup)
- **Files:** new `web/src/components/restore/RestoreProgress.tsx`; edit `web/src/components/RestorePanel.tsx`, `web/src/pages/VMs.tsx`, `web/src/pages/Recovery.tsx`.
- **Change:** Create `RestoreProgress.tsx` rendering the copy-pasted `if(isPending){ started + bgHint + ProgressBar + RestoreCancelButton }` block + success/cancelled/error `<p>` banners. Props: `{ state: BackupWatchState; isPending: boolean; prog: ProgressState | undefined; cancelKey: string; inPlace: boolean; name: string; cancelledRef?: MutableRefObject<boolean>; successMessage: string; showStartedHint?: boolean; t: T }`. Body reproduces RestorePanel.tsx 975–998 exactly but: lift the `restoreProgressCaption` helper here (currently RestorePanel.tsx 17–23); `showStartedHint` (default true) gates the two `restore.started`/`restore.bgHint` `<p>`s (Recovery passes false); success text = `successMessage` prop; cancelled = `t('restore.cancelled')`; error = `state.message`. Replace all **6** duplicate blocks: RestorePanel 975–998, 363–393, 444–463, 541–562; VMs 525–554; Recovery 114–131.
- **i18n:** `restore.completeContainer`, `restore.completeVM`, `restore.recreateComplete`.
- **Risk:** RestorePanel.tsx houses 4 of 6 call sites with different start fns + `inPlace` flags (SnapshotFileBrowser `inPlace={dest==='inPlace'}`, RestoreToFolder `inPlace={false}`, others inPlace). **`cancelledRef` is load-bearing:** RestoreCancelButton sets `cancelledRef.current=true` and `useBackupWatch`'s no-run fallback reads it (backupWatch.ts:257) to report `cancelled` vs green `success` — RestoreProgress MUST forward the SAME ref instance `useBackupWatch` received.

#### 3.3b — Create `RestoreAction` (THE mandated shared in-place restore control)
- **Files:** new `web/src/components/restore/RestoreAction.tsx`.
- **Change:** Owns the `useBackupWatch(kind:'restore')` cycle, confirm gate, leaveStopped option, restore button (spinner + busy hint), and renders `<RestoreProgress>`. Props:
  ```ts
  interface RestoreActionProps {
    domain: 'container' | 'vm';   // progressKey `${domain}:${name}`, matchRun, restore() vs restoreVM()
    name: string;
    snapshotId: string;           // snap.id or 'latest'
    source?: RepoSource;          // undefined => backend default (Recovery)
    otherActive: { active: boolean; phase?: string };
    successMessage: string;
    requireConfirm?: boolean;     // default true; Recovery false
    showLeaveStopped?: boolean;   // default true; Recovery false
    forceLeaveStopped?: boolean;  // default false; Recovery true
    showBusyHint?: boolean;       // default true; Recovery false
    showStartedHint?: boolean;    // default true; Recovery false
    label?: string;               // default t('snapshots.restore')
    t: T;
  }
  ```
  Internals: `const cancelledRef = useRef(false)`; `const progressKey = `${domain}:${name}``; `useBackupWatch({ progressKey, kind:'restore', matchRun:(r)=>r.domain===domain && r.target===name, cancelledRef, start: () => domain==='container' ? restore(name, snapshotId, true, source, forceLeaveStopped||leaveStopped) : restoreVM(name, snapshotId, true, source, forceLeaveStopped||leaveStopped) })`. `const prog = useProgress()[progressKey]`; `const blockedByOther = otherActive.active && !isPending`. Confirm checkbox only when `requireConfirm`; button `disabled = (requireConfirm && !confirmed) || isPending || blockedByOther || state.phase==='success'`. Reuse the exact button spinner markup from RestorePanel 947–958. Pass `inPlace={true}` to RestoreProgress (all three consumers are in-place/recreate). Delete-agnostic and list-agnostic — callers own row chrome, snapshot id/time, and delete button.
- **i18n:** none (reuses existing `snapshots.restore` etc.).
- **Risk:** do NOT let RestoreAction fetch the snapshot list or own delete (delete uses plural-domain `deleteSnapshot`). Keep `matchRun` domain **singular** (`'container'`/`'vm'`); a plural typo makes the watch never resolve. `source` must stay optional (Recovery passes undefined for the backend default repo).

#### 3.3c — Migrate Containers/VMs/Recovery onto RestoreAction
- **Files:** `web/src/components/RestorePanel.tsx`, `web/src/pages/VMs.tsx`, `web/src/pages/Recovery.tsx`.
- **Change:** **Containers** (RestorePanel): replace SnapshotRow inPlace block (927–999) with `<RestoreAction domain="container" name={containerName} snapshotId={snap.id} source={source} otherActive={running} successMessage={t('restore.completeContainer')} t={t} />`, deleting local `useBackupWatch` (802–813), `confirmed`/`leaveStopped` state (794,797), `handleRestore` (838–841). Keep the files/toFolder modes — they only adopt `<RestoreProgress>` (3.3a). Convert RecreateButton (415–466) to `<RestoreProgress>` with `successMessage={t('restore.recreateComplete')}` (or reuse `<RestoreAction snapshotId='latest' requireConfirm={false} showLeaveStopped={false} label={t('snapshots.recreate')} successMessage={t('restore.recreateComplete')}>`). **VMs**: replace VMSnapshotRow restore mechanics (watch 424–435, confirm+button 473–503, leaveStopped 514–524, pending+banners 525–554) with `<RestoreAction domain="vm" name={vmName} snapshotId={snap.id} source={source} otherActive={running} successMessage={t('restore.completeVM')} t={t} />`; keep the id/time/tags line + delete button (504–511) + `handleDelete` (439–452, plural `deleteSnapshot('vms',…)`). **Recovery**: replace RestoreRow mechanics (watch 67–79, button 96–112, pending+banners 114–131) with `<RestoreAction domain={domain} name={name} snapshotId="latest" otherActive={{active: otherActive}} successMessage={t('common.done')} requireConfirm={false} showLeaveStopped={false} forceLeaveStopped showBusyHint={false} showStartedHint={false} label={t('snapshots.restore')} t={t} />`; keep its name + lastBackup line (91–95).
- **i18n:** none new (keys added in 3.3a).
- **Risk:** VMs holds a full ~270-line duplicate (VMRestorePanel + VMSnapshotRow) that never imported RestorePanel — biggest single diff. Recovery deliberately omits started/bgHint + busy-phrase — the `*Hint` prop defaults must reproduce that when false. RestoreRow currently passes `otherActive: boolean` — wrap as `{active: otherActive}`.

#### 3.3d — Extract `SnapshotListShell` (collapsible Backups panel; 4th consumer = Flash)
- **Files:** new `web/src/components/restore/SnapshotListShell.tsx`; edit `web/src/components/RestorePanel.tsx`, `web/src/pages/VMs.tsx`, `web/src/pages/Flash.tsx`, optionally `web/src/pages/Config.tsx`.
- **Change:** Shell = chevron + `t('snapshots.title')` disclosure + optional Source toggle + loading/error/empty states, mapping a snapshot array to caller rows. Props: `{ snapshots: Snapshot[]; loading: boolean; error: string|null; source: RepoSource; onSourceChange: (s:RepoSource)=>void; showSource?: boolean; headerSlot?: ReactNode; emptyText?: string; renderRow: (snap: Snapshot) => ReactNode; t: T }` (keep the `getSettings`/`listSnapshots` fetch in the caller; pass results in). Migrate: RestorePanel shell 1084–1158 (`renderRow=SnapshotRow`, `showSource=advanced`, Recreate/Compare kept as headerSlot/children); VMs VMRestorePanel 598–668 (`renderRow=VMSnapshotRow`, `headerSlot=`delete-all button, `showSource=advanced`); Flash restore card 258–288 (`renderRow=FlashSnapshotRow` — a DOWNLOAD row, no RestoreAction — `showSource` always true, `flash.restoreNote` as children); optionally Config.tsx card (`renderRow=ConfigSnapshotRow`, no restore).
- **i18n:** none.
- **Risk:** the shells diverge (Containers lazy disclosure that also fetches restoreFolder/hostMountRoot 1059–1069 + Compare/Recreate; VMs delete-all header + always-on source-hint; Flash always-open, always source, prepends `flash.restoreNote`). Model via `showSource`/`headerSlot`/`children`, not one rigid layout. **If the shell can't cleanly absorb Containers' extras, keep Containers on its own shell and share only VMs+Flash+Config — do not distort the component to force a 5-way fit.** Lower priority than 3.3a–c (the real logic duplication is the action block).

---

## Dead-code deletion list

| File | Symbol | Action | Evidence |
|---|---|---|---|
| `web/src/components/TopBar.tsx` | entire file (`TopBar`, `IconSun`, `IconMoon`, `Flag`, `LanguageSwitcher`, 158 lines) | DELETE | Grep `import.*TopBar|<TopBar` = 0 matches; Layout.tsx renders only `<Sidebar>`; content duplicated in Sidebar `SidebarControls` 160–261. |
| `web/src/components/Sidebar.tsx` | `NavItem.disabled?` / `NavItem.comingSoon?` props (L16–17), `navDisabled` const (L115–116), `if(disabled){…}` branch (L119–133), signature (L118) | DELETE | No call site passes them (all NavItem usages omit); Grep `comingSoon` = only the definition. Includes hard-coded literal "Coming soon" L128. |
| `web/src/lib/i18n.ts` + 24 locales | `nav.comingSoon` (EN L55 "Coming soon", DE L799 "Demnächst") | DELETE (all 26) | Referenced nowhere; only "coming soon" UI was the dead literal at Sidebar.tsx:128. |
| `web/src/pages/Jobs.tsx` | `ScheduleStatus` `'paused'` (L33) + `cls['paused']` (L49) | OPTIONAL DELETE | `scheduleStatus()` (35–38) only returns `'active'`/`'off'`; `paused` path unreachable. Skip if minimizing churn. |

**Explicitly NOT dead — must be kept (guard rails):**
- `web/src/components/SpikePanel.tsx` — LIVE (imported Settings.tsx:9, rendered :1901, `POST /api/spike`, consumed Dashboard.tsx:209). **Overrides the settings audit's "remove."** Place on the System tab (2.1), do not delete.
- Containers "installed" filter (`FilterControl`, FilterKey, FILTER_STORAGE_KEY) — functional; any removal is Phase-2 filter-popover UX (2.6), not a dead-code sweep.
- Jobs.tsx `FlashSection` "planned" placeholder — intentional UI marker.

---

## Risks & ordering notes

1. **SpikePanel conflict (CRITICAL):** the settings audit says "Spike DEAD, remove" (Settings.tsx:1899 + import :9); the dead-code audit + my 2026-07-07 re-verification say it is LIVE and wired to `/api/spike` and Dashboard. **Resolution: KEEP SpikePanel.** Any implementer must treat "remove Spike" as a defect in the settings audit.
2. **Locale count:** the brief says "23 locales"; the repo has **24** files in `web/src/lib/locales/` + inline `de` + `en` source = **26 sync targets per key**. Use 26 for every add/remove, or the i18n-parity gate / `tsc` fails.
3. **Phase-1 ordering:** do Task 1.0 (i18n batch) first. The `nav.advanced`→`mode.advanced` rename + its Sidebar.tsx:340 reference (Task 1.4) must land atomically. Sidebar.tsx is touched by 1.0 (reference), 1.2 (dead props), and 1.4 (switch) — do all three in one coordinated pass to avoid line-number drift. Dashboard.tsx is touched by 1.3, 1.6, 1.7 — likewise.
4. **Phase-2 ordering:** define `<Advanced>` (2.7 first half) BEFORE migrating gate sites (2.7 second half). Settings tab scaffolding (2.1) before per-tab extraction (2.2). Schedules tab (2.3) before Jobs retirement (2.4) and OffsiteWizard schedule removal (2.8). Keep `ToggleRow`/`RcloneCard`/`CloudCard` exports alive throughout (3 importers).
5. **Phase-3 ordering:** RestoreProgress (3.3a) before RestoreAction (3.3b) before the migrations (3.3c). SnapshotListShell (3.3d) last and lowest priority. Dashboard Cell-2 (3.2) is blocked on a backend `DomainStatus.nextRun` addition — cannot be built frontend-only.
6. **Shared files / cross-file coupling:** `web/src/pages/Settings.tsx` is the single owner of `settings`/`savedSettings`/`save()` (merge-onto-baseline contract, dispatches `bv:settings-changed`) — the tab split must keep it as the state owner and make tabs pure render children. `web/src/lib/advanced.tsx` is a shared primitive (Provider consumed by main.tsx) — extend, never replace. `web/src/lib/i18n.ts` derives `TranslationKey` from `en` — any add/remove propagates type-wide.
7. **Contract freeze reminder:** never rename settings field names, the `"config"` API domain string, progress/run domain strings (singular container/vm/config), the plural `deleteSnapshot` domains, or the `jobs.*` key namespace / `/jobs` route. All user-facing renames are i18n VALUE rewords only.
8. **Cross-area gaps (no dedicated audit provided):** the "Jobs/Plans schedule owner" area (needed for 2.4), the "off-site consolidation" area (3.1 safety strings), and the "Containers/VMs" area (2.6 filter popover, installed-filter removal) were not among the 7 audits. Confirm those areas' owners before locking the anticipatory strings (`offsite.sectionTitle`, `filter.button`) and before deleting `containers.filter*`.
