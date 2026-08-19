# GlimStone Form-Engine Integration (Phase 1: a+b)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (fresh subagent per task, spec-review then code-quality-review) to implement this plan task-by-task.

**Goal:** bring BombVault's `web/src/` frontend into compliance with the shared GlimStone design language's mechanical/contained rules (categories "a" and "b" from the 2026-08-18 audit), without touching the large/systemic items (rainbow/reactive colour mode, the unified one-horizontal-selector rebuild, the hint→bubble content migration, section-headings-as-badges, RTL logical properties) — those are an explicitly deferred Phase 2.

**Source spec:** `D:\nextcloud\it\github\glimstone\docs\design-language.md` + `reference\tokens.css`/`tailwind-theme.css`/`appearance.ts`.
**Source audit:** the full findings this plan is built from are in this session's transcript; every task below quotes the audit's exact file:line evidence so implementers don't need to re-derive it.

**Architecture:** this is CSS/token/component work in `web/src/`, no backend changes. Work happens in an isolated clone (`C:\Users\JUNKER~1\AppData\Local\Temp\claude\d--nextcloud-it-github\f32a9de0-34f3-47a2-b2ee-f80d7aae709c\scratchpad\bombvault-glimstone-audit`, branch `feature/glimstone-form-engine`) because the real working repo (`D:\nextcloud\it\github\bombvault`) has an unrelated background job actively committing to a different branch there — do not touch that directory for this plan's work. This clone's `origin` remote is the real GitHub repo, so `git push` from here works normally once ready to open a PR.

**Tech stack:** React + TypeScript + Tailwind v4 (`@theme` layer in `web/src/index.css`), no CSS framework beyond that.

---

## Task 1: Token/theme foundation

**Files:** Modify `web/src/index.css` (the `:root`/`[data-theme="dark"]` block at lines 82-147 and the `[data-theme="light"]` block at 149-211), `web/src/lib/theme.ts`, `web/index.html`, `web/src/lib/accent.ts`. Reference: `D:\nextcloud\it\github\glimstone\reference\tokens.css` and `reference\appearance.ts`.

This must land first — later tasks reference tokens this task adds.

1. Correct the 7 light-theme values that drifted from the reference (`--carbon-bg`, `--carbon-sidebar`, `--carbon-surface`, `--carbon-surface2`, `--carbon-surface3`, `--carbon-hover`, `--carbon-border`, `--carbon-text-sub`, `--sidebar-text` — audit section 1's table has the exact current-vs-spec values for each).
2. Add the 11 missing tokens as real custom properties under both theme blocks: `--accent-soft`, `--elevation`, `--hairline`, `--focus-ring`, `--radius-control`, `--radius-pill` (keep `--radius-card` but move it out of the `@theme`-only literal at line 54 into a real themeable custom property too), `--text-heading`, `--text-body`, `--text-dense`, `--text-caption`. Copy exact values from `reference/tokens.css`.
3. Add `@media (prefers-color-scheme: dark)` / `(prefers-color-scheme: light)` fallback blocks per the reference's pattern (currently zero `prefers-color-scheme` hits in the whole tree). `web/src/lib/theme.ts:8`'s `DEFAULT` is currently a fixed `"dark"` with only a two-value toggle — add a third "system" state that reads the media query and updates live if it changes. Remove the hard-coded `data-theme="dark"` from `web/index.html:2` so the system default actually applies before JS hydrates (use a tiny inline pre-hydration script if a flash-of-wrong-theme would otherwise occur — check how KnightLoader or another sibling app handles this FOUC concern if one exists locally, per this project's own documented FOUC gotcha).
4. Copy `reference/appearance.ts` in (as a new file, e.g. `web/src/lib/appearance.ts`, or merge its logic into the existing `lib/accent.ts` if that reads more naturally given BombVault's current structure) and route accent-setting through it so `--accent-contrast` is actually COMPUTED (sRGB luminance) instead of hard-coded to `#161616` in both themes (`lib/accent.ts:19-22` today never sets it at all), and `--accent-soft` gets set alongside `--accent`.
5. Do NOT touch `--status-info-*`'s blue value in this task — that's audit item 19 (resolving the fifth hue), sequenced deliberately AFTER the focus-ring rework (Task 8) since 63 call sites currently key focus rings off `--status-info-solid`; changing the underlying colour now vs. after Task 8 rewires focus entirely changes the visual outcome. Leave it as-is here, just don't add MORE dependencies on it.

**Do NOT do in this task:** the `data-shape`/radius-token *sweep* across the 367 call sites (that's Task 2) — this task only adds the token *definitions*.

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run`, visually confirm dark/light/system all still render correctly (a quick local `npm run dev` + browser check is enough for this task alone; the full live-verification pass happens once all tasks land).

---

## Task 2: Shape-token sweep (`data-shape` + radius tokens)

**Files:** `web/src/index.css` (add a `[data-shape="round"]` / `[data-shape="square"]` axis if BombVault wants a user-facing shape toggle — check whether Settings already exposes a "corner style" preference anywhere before adding a NEW one; if not, a fixed `--radius-control`/`--radius-pill`/`--radius-card` set with no user toggle is fine, don't invent a settings UI that wasn't asked for), then a scripted sweep of `rounded-lg` (268 sites), `rounded-sm` (74), `rounded-full` (62), `rounded-md` (23), `rounded-xl` (2) across all of `web/src/pages/` and `web/src/components/` to the appropriate token-backed class (map: card-level rounding → `--radius-card`, control-level (buttons, inputs, small chips) → `--radius-control`, pill/circle badges → `--radius-pill`).

Given the large count, this is mechanical but must be done carefully: write a script (or use careful multi-file find/replace) rather than hand-editing 367 sites individually, but REVIEW every substitution — some `rounded-full` uses are legitimately avatar/dot-style circles that should map to `--radius-pill` capped with `min(var(--radius-pill), 50%)` per the badge rule, not a blind replace.

Verify: `npm run build`, `npx tsc --noEmit`, `npm run lint`, then a full visual pass (dev server + browser) across every page — this is the highest visual-regression-risk task in the whole plan since it touches nearly every component. Screenshot a few representative pages before/after if that helps catch regressions.

---

## Task 3: Cheap polish (typography, hardcoded values, InfoBubble, empty states)

**Files:** `web/src/components/RestorePanel.tsx` (lines 534, 556, 562, 567 — `text-[10px]` → `text-caption`/11px), `web/src/pages/Dashboard.tsx:1056` (hard-coded untranslated `"No containers found."` string — needs a real i18n key, check `web/src/lib/i18n.ts` for the nearest existing similar key's naming convention and add it to English + propagate to all 25 locale files per this project's own established i18n discipline — or if time-boxing this task, at minimum fix the English string and flag the other 24 locales as a fast-follow, but check first whether a bulk-insertion script pattern already exists in this repo's history for this kind of single-key propagation), `web/src/pages/Settings.tsx:310`/`:313` (`text-green-500`/`text-red-400` → the proper `--status-ok`/`--status-fail` Tailwind utilities), `web/src/components/InfoBubble.tsx` (rename prop `text` → `tip` and update all 5 call sites in `Settings.tsx`; align bubble chrome to `.glim-bubble` in `reference/tokens.css` — surface colour, radius, elevation+hairline instead of `shadow-lg`, `text-caption` size, `max-width: 280px` instead of `w-64`; add the `glim-fade` entrance; remove `focus:outline-none` at `InfoBubble.tsx:59` which currently kills the keyboard focus indicator).

For empty states: add a muted icon at reduced opacity to the 5 card-shaped empty states that currently lack one (`Receiver.tsx:588-598`, `Fleet.tsx:741-751`, `Files.tsx:1157-1178`, `Containers.tsx:1914-1920`, `VMs.tsx:1253-1257`), and upgrade `Containers.tsx:1914-1920`/`VMs.tsx:1253-1257` to also have an action button matching the other three's pattern where a sensible action exists (e.g. nothing to add if it's a live Docker/libvirt enumeration rather than a BombVault-managed list — use judgement, don't force a button where there's nothing useful to click).

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run` (this repo has an i18n locale-parity test — if the Dashboard string gets a real key, this test will catch any locale left behind), visual check of the InfoBubble and a couple of empty states.

---

## Task 4: Unified `Toggle` component

**Files:** Create `web/src/components/Toggle.tsx` (or similar name matching this repo's existing naming convention — check whether components are typically PascalCase-named after their role, e.g. `IncludeToggle.tsx`). Consolidate `web/src/components/IncludeToggle.tsx:42-58`, the `ToggleRow` in `web/src/pages/Settings.tsx:82-120` (exported and reused by `Config.tsx:15`, `Recovery.tsx:6`), and the inline fourth copy in `web/src/components/OffsiteTargetsSection.tsx:367-382` into ONE shared component. Add a `hideLabel` prop (text survives as `aria-label`, visually hidden) per the spec's exact mechanism.

Fix the found violations while migrating: remove the redundant duplicate-caption pattern at `Settings.tsx:4435`/`:4440` (Card "Weekly digest" + ToggleRow "Weekly digest" — use `hideLabel` on the ToggleRow since the Card title already says it; same at `Settings.tsx:4350-4361` "Encryption"/"Enabled..." and `Settings.tsx:4266-4275` "Monitoring (Prometheus)"/"Expose /metrics" — check `lib/i18n.ts:1139`/`:1141` and the German `:2461`/`:2463` for the exact duplicate strings), un-indent the sub-control at `ItemScheduleOverride.tsx:65` (remove `pl-5`, rely on already-being-dimmed-when-parent-off instead), and replace `opacity-50 pointer-events-none` on CONTAINERS (not the switch itself) at `CadenceBuilder.tsx:216` with a non-opacity darkening approach per rule 15 ("opacity is off-limits on a container — it composites the whole subtree").

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run`, visual check of every page that uses a toggle (Settings, Config, Recovery, OffsiteTargetsSection call sites).

---

## Task 5: Unified `Badge` component with named size stages

**Files:** Create `web/src/components/Badge.tsx`. Consolidate `Dashboard.tsx:276-300` (`StatusChip`), `SpikePanel.tsx:8-28` (duplicate `StatusChip`, hard-coded English "OK"/"FAIL"/"INFO" — fix to use i18n keys while migrating), `Receiver.tsx:54-68` (`Badge`), `Fleet.tsx:73-87` (byte-identical duplicate of Receiver's), `Containers.tsx:36-50`/`VMs.tsx:31-45` (`StateChip` duplicate pair) into ONE component with a small number of named size stages (small/medium/large is enough per the spec — pick concrete px values that cover the app's actual current range: the audit found `px-2 py-0.5 text-xs`, `px-1.5 py-0.5 text-xs`, `px-1.5 py-0.5` with no size class, and `px-1.5 py-0.5 text-[10px]` all coexisting for what should be the same visual weight).

Enforce pixel-identical height between a `<button>`-rendered badge and a `<span>`-rendered badge at the same stage (explicit `height`, `min-height: 0`, consistent `box-sizing`, `appearance: none` on any native element) — the audit's concrete failing example is `ErrorDetailPanel.tsx:247-258` (a `<span>` count badge next to a `<button>` "Resolve" at different padding/radius/height in one flex row) and `OffsiteTargetsSection.tsx:262-271` vs `:275-299`. Fix both after the component exists. Cap the pill radius with `min(var(--radius-pill), 50%)` rather than the raw token on anything short.

Migrate every call site listed in the audit's section 7 table to the new component. Fix the two adjacent-badge radius inconsistencies at `Dashboard.tsx:601`/`:622`/`:645`/`:659`/`:668` (three different radius classes for what should be the same badge type in one component) as part of the migration.

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run`, visual check of Dashboard, Receiver, Fleet, Containers, VMs, ErrorDetailPanel, OffsiteTargetsSection.

---

## Task 6: The reveal eye (secret/token fields)

**Files:** Create a small reveal-eye affordance (a component or a shared hook, whichever fits this codebase's existing patterns better — check how the other small UI primitives here are structured before choosing). Wire it into the fields listed in audit section 6's table — prioritize `Receiver.tsx:446-454` (the APP_KEY-equivalent field, highest value per the audit) first, then the rest: `Login.tsx:56-64`, `Settings.tsx:822-828`/`:1005-1011` (the two show-once token fields — these already have the correct `flex-1 min-w-0` full-width wrapper per the audit, so the eye slots in without a width regression), `Settings.tsx:1200`/`:1391` (AWS secret key), `Settings.tsx:1221`/`:1410` + `OffsiteWizard.tsx:464-466` (restic REST password), `Settings.tsx:1648` (Matrix token), `Settings.tsx:1734-1735` (SMTP password), `Settings.tsx:4281-4291` (Prometheus token), `Settings.tsx:4549-4556`/`:4562-4569` (set/confirm login password), `Fleet.tsx:621-629` (peer token).

Per spec: a bare icon inside the field's trailing padding (not a chrome button beside it — on a foreign-host-UI page this would need to be a `<span role="button" tabindex="0">` instead of a real `<button>` to avoid inherited button theming, but BombVault owns its own CSS everywhere it renders, so a real `<button>` is fine here), neutral colour never the accent, doesn't change the field's width. While wiring `Settings.tsx:822-828`/`:1005-1011`, move the existing "Regenerate"/"Disable" action buttons (`:829-844`, `:1012-1027`) to their own line below the field instead of beside it, per the spec's explicit "verify/check action goes on its own line below, not beside it" rule.

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run`, visual + interaction check (actually click each eye and confirm reveal/hide + width-stability) on at least the APP_KEY field and one AWS-secret field.

---

## Task 7: Confirmation dialog (replace `window.confirm()`)

**Files:** Extract a reusable confirmation-dialog shell from the existing modal chrome pattern already used 5 times (`WhatsNewDialog.tsx:219-299` is the cleanest reference — `rounded-card bg-carbon-surface`, anchored header, scrolling middle, anchored footer). Create `web/src/components/ConfirmDialog.tsx` (or similar). Replace all 21 `window.confirm()` call sites: `VMs.tsx:436, 528, 737, 1209`, `Settings.tsx:2035, 2061`, `Flash.tsx:99`, `Containers.tsx:269, 1286, 1837`, `Receiver.tsx:188`, `Config.tsx:258`, `Files.tsx:383, 506, 597, 874`, `Fleet.tsx:404`, `RestorePanel.tsx:135, 250, 615`, `RestoreCancelButton.tsx:47`.

Scope for THIS task: swap the mechanism (native `confirm()` → styled, keyboard-accessible, non-blocking dialog) using the SAME existing copy/strings at each call site — do not add new "N snapshots, X GB" stake-detail copy in this pass (the audit notes that would need new i18n keys with interpolated values across all 25 locales, which is real additional scope; leave a code comment at 2-3 of the highest-value sites — e.g. `Files.tsx:874` delete-a-file-set — noting where richer stake copy would go, as a flagged follow-up, not silently forgotten). Keep `RestoreCancelButton.tsx:43-47`'s existing hard/light-warning branching and `{name}` interpolation exactly as-is, just through the new dialog shell instead of `confirm()`.

Also fix the two "reversible action asks anyway" cases while here: `Receiver.tsx:188` and `Fleet.tsx:404` (removing a re-addable monitoring entry) — per the spec's "reversible actions don't ask" rule, these should use the lighter two-click inline-confirm pattern already correctly used at `OffsiteTargetsSection.tsx:282-299`/`Settings.tsx:1360-1377`, not a full dialog. Everything else in the 21-site list is a genuinely destructive/irreversible action and keeps a real dialog.

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run`, interaction check (trigger at least 3 different confirmation dialogs and both the confirm and cancel paths) plus the two downgraded-to-inline-confirm cases.

---

## Task 8: Focus ring → brightness step

**Files:** Sweep all 75 occurrences of `focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid` (63 sites) and its `focus-visible:` variant (12 sites) across every page/component with a form field. Replace with a shared field class (add it once, e.g. in `index.css` or as a Tailwind `@layer components` utility) that does `surface2 → surface3` on focus instead of an outline ring, using the `--focus-ring` token Task 1 added (keep it available for anywhere an actual outline is still appropriate — e.g. non-field focusable elements like buttons/links, which are NOT in scope for rule 14's specific "form fields" wording; only convert genuine text input/select/textarea fields, not every focusable element in the app).

This depends on Task 1 (needs `--focus-ring` to exist) and should land after Task 2 (the radius sweep) so the two large mechanical sweeps don't produce overlapping diffs on the same lines.

Verify: `npm run build`, `npx tsc --noEmit`, `npm run lint`, then Tab through several forms in the browser (Login, Settings, Receiver) confirming focus is visibly clear via the brightness step alone, keyboard accessibility isn't regressed.

---

## Task 9: Toast system

**Files:** Create `web/src/components/Toast.tsx` (or a `ToastProvider`/`useToast` hook pair, whichever fits this codebase's existing state-management patterns — check if there's a context/provider convention already used for something similar before inventing a new one) plus the CSS (`.glim-toast`/`glim-toast-in` already exist in `reference/tokens.css:337-340` — copy them in). Implement: fixed 4-second duration, stacked in a fixed corner (not replacing), hover-or-focus pauses the countdown AND preserves remaining time (not a reset to full duration), a severity-based quiet mode that always surfaces failures/blocking items but can suppress routine completions.

Scope for THIS task: build the system and wire it to 3-4 clear candidates from the ~40 inline-status sites the audit found, as a proof of real adoption — do NOT attempt a full migration of all ~40 sites in this pass (the audit explicitly flags that triage as separate judgement work). Good initial candidates: the copy-feedback pattern (`Settings.tsx:237, 245, 802, 967`, `Fleet.tsx:45` — currently a 2000ms inline text swap) and the `SaveBar` success/error pattern (`Settings.tsx:128-168`, `Settings.tsx:3020`, `Config.tsx:157`, `Settings.tsx:1098`/`:1176` — currently a 3000ms inline text swap) are both good fits since they're already ad-hoc timed-feedback patterns being formalized, not being changed in kind. Leave a note (code comment + this plan's own follow-up list) that the remaining ~35 inline-status sites are an explicit, deliberate follow-up requiring per-site judgement, not something this task silently left incomplete.

Verify: `npm run build`, `npx tsc --noEmit`, `npx vitest run`, interaction check (trigger a toast, hover over it and confirm the countdown pauses and resumes from where it was, trigger two toasts and confirm stacking).

---

## Final Live Verification & Ship

- [x] Whole-branch review (mirroring this project's own established pattern) after all 9 tasks land, looking specifically for: token-name drift between tasks, any place Task 2's radius sweep and Task 8's focus sweep produced conflicting edits on the same line, and overall visual coherence. **No token-name drift and no sweep collisions found.** Three cross-task issues found and fixed — see "Whole-branch review findings" below.
- [x] `npm run build`, `npx tsc --noEmit`, `npm run lint`, `npx vitest run` — full clean run. tsc clean; 297 tests in 16 files pass; lint 0 errors (2 pre-existing `react-hooks/exhaustive-deps` warnings in ActivityLog.tsx and Sidebar.tsx, both present on `main`); build clean. Go side: `TestThemeBootScriptCSPHashMatches` still passes, so the rebuilt bundle didn't invalidate the CSP hash.
- [x] Live verification: real `cmd/bombvault` binary built from the branch tip and driven in a real browser. Walked Dashboard, Containers, VMs, Files, Flash, Config, Fleet, Receiver, Recovery, Login and all 7 Settings tabs in BOTH themes plus the new "system" state (no stored preference → follows the OS, no flash of wrong theme). Verified end-to-end: unified Toggle (`aria-label` present on every switch incl. `hideLabel` ones), Badge, reveal eye (show/hide, width-stable, correct in both themes), ConfirmDialog (opens, Tab-trap holds, Escape closes AND returns focus to the trigger), toast (fires, hover-pause survives past the 4s duration, `pointer-events:none` viewport at z-70). Zero console errors from the app. Rule 15 spot-check: every `<fieldset>` in the Schedules tab computes `opacity: 1` — no container-level opacity anywhere.
- [ ] Open a PR from this branch, flag Phase 2 (rainbow/reactive colour mode, unified selector, hint→bubble migration, headings-as-badges, RTL, remaining fifth-hue resolution, remaining toast-site migration) as explicit, deliberately deferred follow-up work, not silently dropped scope. **Use the consolidated list below verbatim.**

---

## Whole-branch review findings

Fixed in this review (see the review commit):

1. **A raw NUL byte in `web/src/pages/Recovery.tsx` made the whole file invisible to every grep-driven sweep.** A React `key` used a literal `0x00` as its separator instead of the `\u0000` escape, so ripgrep/grep/git classified the file as binary and returned zero content lines for it. This is not theoretical: Task 7's own commit (`45fdcbf`) had to hand-find this file's two "grep-invisible" call sites after the sweep missed them. Replaced with the escape (identical runtime value) and left a comment explaining why it must stay an escape. Pre-existing on `main`, but fixed here because Phase 2's RTL logical-property sweep and the remaining toast-adoption sweep are both grep-driven and would have hit it again.
2. **The new shared `Toggle` depended on the deliberately-unresolved `--status-info-solid` hue.** Task 1's plan note says of that token "just don't add MORE dependencies on it", yet the new unified switch was authored with `focus-visible:outline-statusInfoSolid` while its three sibling shared controls (RevealInput's eye, InfoBubble's icon, Toast's dismiss X) all use `--focus-ring`. Switched to `--focus-ring`, and aligned the 5 not-yet-migrated switch copies (`OffsiteWizard`, `Containers`, `Files`, `Settings`, `VMs`) to match so Settings doesn't show amber-focus and blue-focus switches side by side. Contrast re-derived before the change: ≥3:1 on both `--carbon-surface` and `--carbon-surface2` in both themes.
3. **`index.css`'s Task-8 comment over-claimed `--focus-ring`'s reach.** It stated the token was "still used by every non-field element", which was never true. Corrected to state the measured reality and to explain why closing the gap is Phase 2 work (see follow-up 1).

Verified clean, no action needed: no duplicate or contradictory CSS rules and no orphaned pre-unification classes in `index.css`; no leftover hard-coded `rounded-lg/md/sm/xl` anywhere (the 40 remaining `rounded-full` are all genuine circles — spinners, status dots, switch thumbs, the step-number badge — correctly left alone); `window.confirm()` fully eliminated; z-index layering is ordered and intentional (overlays 50 → bubble 60 → toast 70); the shared component set follows one consistent convention (pure hookless component + companion hook for state/DOM, `describe("Component — aspect")` tests callable as plain functions in the node-environment suite).

---

## Consolidated deferred / follow-up list (for the PR description)

Everything below is deliberate, disclosed scope — not dropped work. Gathered from the 28 commit messages and in-code `TODO(#follow-up)` markers so a PR reader doesn't have to reconstruct it.

**Resolved before merge:**

0. **Docs said BombVault opens dark, and it no longer does — RESOLVED.** Task 1 made "system" the default (`theme.ts` `DEFAULT = "system"`, hard-coded `data-theme="dark"` removed from `web/index.html`), so a first-time visitor on a light-mode OS now gets a light UI. This was flagged as a positioning call rather than fixed unilaterally; jdp decided (2026-08-19): keep the system-default behavior (matches the GlimStone spec — no app should open against the OS preference), rewrite the copy instead of reverting the theme default. Fixed in `52146cb`: `README.md`, `.github/DOCKERHUB.md`, `.github/SUPPORT_THREAD.html`, `mkdocs.yml`, `docs/stylesheets/extra.css`, and all 26 `docs/index*.md` files (English + 25 real per-language translations, not machine-literal) — 31 files total. Note: this only updates the repo's own doc sources; the already-live Unraid forum support-thread post itself is unchanged and would need a separate manual edit if jdp wants that too.

**Named Phase 2 scope (from the plan's own goal statement):**

1. **Rainbow / reactive colour mode** (`[data-rainbow]`, `.glim-hue`) — the largest deferred item.
2. **The unified one-horizontal-selector rebuild.**
3. **The hint→bubble content migration** — moving existing grey `<p>` hint prose behind `InfoBubble`. The component and its chrome are done; the content move is not.
4. **Section-headings-as-badges.**
5. **RTL logical-property sweep** — `RevealInput` already uses logical `pe-*`/`end-*`, and `ToastViewport`'s corner uses `end-0`; the rest of the app is still physical.
6. **Audit item 19 — resolve the fifth hue** (`--status-info-solid`). Sequenced deliberately after Task 8, which is now done, so this is unblocked.

**Focus system, completing Task 8:**

7. **Adopt the reference's base `:focus-visible` rule.** `glimstone/reference/tokens.css` ends with `:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px }` and its adoption checklist says to copy it; BombVault took the token but not the rule. Measured live: ~215 buttons/links declare no focus utility and fall through to Chromium's native ring, 8 pre-existing sites still paint the old `outline-statusInfoSolid` blue (`Containers.tsx:909/923/959/983`, `Dashboard.tsx:144/1260`, `Settings.tsx:4786`), and only the 4 shared controls opt into `--focus-ring` by hand. Pair this with item 6 — the same base rule is what rainbow mode rebinds per item, so items 1, 6 and 7 want to land together.

**Per-task disclosed deferrals:**

8. **Task 4 — 5 unmigrated `role="switch"` copies** (`OffsiteWizard.tsx:518`, `Containers.tsx:456`, `Files.tsx:92`, `Settings.tsx:2737`, `VMs.tsx:264`). Their focus ring was harmonised to `--focus-ring` in this review, but they are still hand-rolled switches rather than `<Toggle>`. Also: `Toggle.test.ts` does not exercise `ToggleRow` directly.
9. **Task 7 — richer "stake detail" confirmation copy** ("N snapshots, X GB"). Needs new interpolated i18n keys across all 25 non-English locales. Marked in code at the three highest-value sites: `Containers.tsx:266`, `Files.tsx:605` (flagged as the highest-value one), `VMs.tsx:528`.
10. **Task 9 — ~35 remaining inline-status sites.** Four proof-of-adoption sites are live (Fleet's `CopyBlock`, Settings' `VMSSHCard` and `handleSetPassword`, Config's `ConfigSettingsCard`). The bulk is `SaveBar`'s ~30 call sites in `Settings.tsx`, which all share one generic `save()` helper — converting the helper would silently migrate all of them at once, so per-site triage is separate work. See the `SaveBar` header comment in `Settings.tsx` and `lib/toast.tsx`'s header.
11. **Task 1 — no UI path back to "system".** `theme.ts:53-63` documents it: once a user's first toggle click moves them off the default, only clearing `localStorage` returns them. Needs a third icon, a new i18n key across every locale, and a cycle-order decision. Note there is also no `theme.system` i18n key yet.

**Found by this review, not previously tracked:**

12. **Finish the type scale.** Task 1 added `--text-heading`/`--text-body`/`--text-dense`/`--text-caption`, but there was never a type sweep to match Task 2's radius sweep. Current adoption: `text-caption` 6 uses, `text-dense` 4, `text-body` 0, `text-heading` 0 — against **47 hard-coded `text-[11px]`**, which is exactly `--text-caption`'s value. Mechanical and visually identical today (and a small a11y win, since the token is `rem` and scales with user font size), but it is a 47-site sweep across ~20 files and deserves its own commit rather than riding along on a review.
13. **Dashboard badges render raw, untranslated status strings.** `Dashboard.tsx:379/392/580/872/987/1720/1741` pass values like `"failed"`, `"ok"`, `"neutral"` straight through as badge text (visible live as a red `failed` chip on a German UI). Pre-existing on `main` and not a regression — but note Task 5 fixed exactly this defect class for `SpikePanel`'s hard-coded `OK`/`FAIL`/`INFO` while migrating it, so the same fix was applied to one migrated call site and not the other.
14. **Document the new "Quiet toasts" setting.** `Settings.tsx:4833` adds the only new user-visible option on the branch (strings at `i18n.ts:654-655`, persisted as `bombvault.quietToasts`). It appears in no README, `docs/configuration.md`, `docs/features.md`, or `DOCKERHUB.md`.
15. **Regenerate the README screenshots.** No asset changed on this branch, so `.github/assets/dashboard.png`, `recovery.png`, `containers.png`, `settings.png` and `restore-demo.gif` (`README.md:112-139`) all predate the radius sweep, the unified Badge/Toggle, the focus change and the corrected light-theme tokens.
16. **Minor i18n tidy-ups.** `common.confirm`/`common.cancel` were added as the canonical generic pair, but the older `restore.confirm`, `files.cancel`, `settingsIO.cancel` and `offsite.targets.cancel` still duplicate them. `confirmDialog.title` and `common.confirm` are also identical in both shipped locales ("Confirm" / "Bestätigen"), kept separate on purpose so a locale can diverge. Three different namespaces were invented for the three new components' generic chrome labels (`common.*` for the eye, `confirmDialog.*` for the dialog, `toast.*` for the toast).
17. **`docs/superpowers/specs/2026-07-07-v5-redesign-spec.md` is archivally stale.** It still prescribes `rounded-lg` chrome (`:161`), `StatusChip` (`:237`, `:239`) and cites Settings line ranges that have moved. It is a completed v5.0.0 spec in a repo now at v7.11.x, so this is history rather than live guidance — flagged only so nobody follows it as current.
