# GlimStone Form-Engine Integration, Phase 2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (fresh subagent per task, spec-review then code-quality-review) to implement this plan task-by-task.

**Goal:** implement everything Phase 1 (`docs/superpowers/plans/2026-08-18-glimstone-form-engine.md`, PR #155, merged) deliberately deferred: the rainbow/reactive colour engine, the unified one-horizontal-selector, the hint→bubble content migration, section-headings-as-badges, the RTL logical-property sweep, resolving the fifth hue, and completing the focus system — plus the smaller items the Phase 1 whole-branch review found and tracked.

**Source spec:** `D:\nextcloud\it\github\glimstone\docs\design-language.md` (sections: "The colour engine", "Right-to-left languages", rules 11/13, "The user-owned axes") + `reference\tokens.css`/`tailwind-theme.css`/`appearance.ts`.
**Reference implementation:** `D:\nextcloud\it\github\knightloader\web\src\` — a real, shipped app using this exact mechanism. `lib/appearance.ts` (the framework-free engine, already essentially identical to `reference/appearance.ts`), `pages/settings/Look.tsx` (the real Settings UI for shape/accent/rainbow), `components/ui.tsx` (`Swatch`/`SwatchRow`/`Tabs`/`FieldGroup` — read these for the real, working implementations of two of this plan's own tasks, Task 3's selector and the swatch-row pattern Task 1 needs).
**Source of the deferred-item list:** `docs/superpowers/plans/2026-08-18-glimstone-form-engine.md`'s tail ("Consolidated deferred / follow-up list").

**Architecture:** CSS/token/component work in `web/src/`, PLUS new server-persisted settings fields for rainbow (accent/shape are already client-only in BombVault per Phase 1 — check whether they should also move server-side to match KnightLoader's pattern, or stay client-only; see Task 1). Work happens in an isolated clone (`C:\Users\JUNKER~1\AppData\Local\Temp\claude\d--nextcloud-it-github\f32a9de0-34f3-47a2-b2ee-f80d7aae709c\scratchpad\bombvault-glimstone-phase2`, branch `feature/glimstone-phase2`) since the real working directory is shared with other concurrent work. This clone's `origin` remote is the local `D:\nextcloud\it\github\bombvault` repo (not GitHub directly) — push there, then push from `D:\nextcloud\it\github\bombvault` to the real GitHub remote once ready to open the PR (mirrors Phase 1's exact process).

**Tech stack:** React + TypeScript + Tailwind v4, Go backend for anything server-persisted.

---

## Task 1: The colour engine — rainbow/reactive mode, foundation + Settings UI

**Files:** New `web/src/lib/appearance.ts` (or extend the existing `web/src/lib/accent.ts` from Phase 1 — read it first; Phase 1 already built shape-token *definitions* and accent-contrast computation, this task adds the *rainbow* half and decides whether to consolidate accent+shape+rainbow into one `appearance.ts` module matching the reference's architecture, or keep them split). `web/src/index.css` (port the `[data-rainbow]`/`.glim-hue`/`.glim-tint`/`.glim-hue-icon` CSS rules from `reference/tokens.css` and the `@theme`-resolution fix from `reference/tailwind-theme.css` — read the spec's own documented "three traps, all hit in production" section before starting, they are not hypothetical). A new Settings UI section (in `Settings.tsx` or wherever Phase 1's accent picker already lives — check) mirroring `knightloader/web/src/pages/settings/Look.tsx`'s structure: shape picker (if BombVault doesn't have one yet — check Phase 1's actual shipped state first), rainbow master switch + reactive/rotate sub-switches (using Phase 1's own `Toggle` component) + an 8-swatch palette editor.

**Decide, and document your decision:** does BombVault persist appearance settings server-side (like KnightLoader's `cfg.rainbow` etc., PATCHed to a settings endpoint) or client-only (localStorage, like Phase 1's accent picker apparently already does — verify)? Read Phase 1's actual `accent.ts`/theme handling to see which pattern BombVault already committed to, and stay consistent — don't introduce a second persistence mechanism for a sibling setting. If BombVault's accent is currently client-only, rainbow should very likely be client-only too (be consistent, don't invent a new backend requirement mid-task); if you find a reason server-side is actually needed (e.g. a shared/multi-viewer context where two people must see the same colour), justify it explicitly.

**Read the spec's "three traps, all hit in production" section for the colour engine and address every one:**
1. Rainbow position must be assigned by LIST POSITION, never a hash of an id/name — verify whatever you pick a position from.
2. Every element carrying `.glim-hue` needs the wash (`.glim-tint`) too, or a settled/completed item shows no colour at all (rule 4's green-on-completion swallows the hue).
3. Tailwind's `@theme` resolves `--color-accent: var(--accent)` ONCE at `:root` — a `.glim-hue` subtree redefining `--accent` never reaches `bg-accent`/`text-accent` utility classes. The hue rules must set `--color-accent`/`--color-accentContrast`/`--color-accentSoft` themselves (see `reference/tailwind-theme.css`'s own `[data-rainbow] .glim-hue` block for the exact mechanism) — copy this pattern exactly, this is the single easiest thing to get subtly wrong.

**Security requirement from the spec:** validate any user-supplied palette colour server-side (if persisted server-side) with `^#[0-9a-fA-F]{6}$`, and take a palette **all-or-nothing** — one invalid colour among 8 good ones is not "87% safe", reject the whole palette. If client-only, validate client-side with the same rigor before ever writing to `document.documentElement.style`.

**TDD:** cover `hueVars`/`rainbowAt`/`rainbowColor`'s pure logic (position→colour mapping, rotation/seed offset, palette validation/all-or-nothing rejection) using this branch's established no-jsdom pattern. Cover the Settings UI's wiring at whatever level Phase 1's own Settings-adjacent components were tested at.

**Verify:** `npm run build`, `npx tsc --noEmit`, `npx vitest run`, live check (real Go binary): turn rainbow on, confirm `data-rainbow` appears on `<html>`, confirm `--rb-0` through `--rb-7` are set; this task alone doesn't need any `.glim-hue` consumers yet (that's Task 2) so the live check here is "the engine works and the Settings UI reflects/controls it correctly", not yet "the app looks rainbow".

---

## Task 2: Apply rainbow positions to real list rows

**Files:** Depends on Task 1. Candidates identified by the original audit: the 8 sidebar `NavItem`s (`Sidebar.tsx`), Settings tabs (once Task 3's unified selector exists — sequence this after Task 3, or coordinate: apply hue vars to whatever selector component ends up rendering tabs), container/VM/file-set list rows (`Containers.tsx`, `VMs.tsx`, `Files.tsx`), the 5 heatmap domain toggles (`Dashboard.tsx`).

**Your job:** decide which of these candidates genuinely benefit from a rainbow position (a list where "which row is which" benefits from a stable, glanceable colour — e.g. a container list where a user tracks several containers at once) versus which don't (e.g. a single always-visible heatmap toggle row probably doesn't need 5 different hues competing with rule 4's fixed status hues). Use judgement; don't rainbow everything just because you can. For whichever you pick: assign position by LIST INDEX (never a hash), wire `hueVars()`'s output as inline styles + `.glim-hue` class on the owning element, add `.glim-tint` to the row wrapper so a settled/completed row still shows colour (trap #2 above), add `.glim-hue-icon` to any icon that should also tint.

**Verify:** live, both `rainbow: on` (non-reactive) and `rainbow: reactive` states, both themes — confirm colours are stable across re-renders/re-fetches (position-based, not flickering), confirm a COMPLETED/settled row still shows its hue via the wash (not swallowed by the green completion colour), confirm reactive mode genuinely rests neutral and only shows colour on hover/active (not on rest).

---

## Task 3: The unified one-horizontal-selector

**Files:** Create `web/src/components/Selector.tsx` (or match whatever KnightLoader's `Tabs.tsx` calls its own version — read it for a real working reference: roving tabindex, arrow-key/Home/End navigation, RTL-aware, `select="one"|"many"`, filled/unfilled segment style, no wrapping container per rule "no wrapping bar"). Migrate every hand-rolled selector found in the original Phase 1 audit (re-verify current line numbers, a lot has changed since): the Settings tab strip, `Containers.tsx`'s `SortControl`/`FilterControl`/`ChipFilter`, `VMs.tsx`'s duplicate `SortControl`/`ChipFilter`, `Files.tsx`'s `destChip`, `Dashboard.tsx`'s heatmap domain toggle, `CadenceBuilder.tsx`'s cadence-mode pills + weekday pills, `SourceToggle.tsx`. Also check for the `<label>`-wraps-multiple-tabs trap the spec calls out explicitly (a `Field` wrapping 3 tabs hands its click to the first one) — grep for it, the Phase 1 audit found zero instances but re-verify.

**Verify:** live, keyboard-only navigation (arrow keys, Home/End, roving tabindex) on at least 3 migrated selectors, RTL direction-awareness (Arabic/Hebrew), both themes, confirm no wrapping wrapper/box around any selector, confirm the Settings tab strip gets icons per the spec's "top, with an icon" rule (the audit found it currently has none).

---

## Task 4: Hint→bubble content migration

**Files:** `web/src/components/InfoBubble.tsx` (already spec-compliant from Phase 1 Task 3 — this task is content migration, not component work). Migrate the highest-value subset of the ~106 permanent grey `<p>` hint paragraphs found by the original audit (Settings.tsx alone holds ~85) into `InfoBubble`'s `tip` prop attached to the relevant field's label, per the spec's "explanations live in a bubble, not on the page" rule.

**Scope discipline:** 106 sites is a lot of judgment-heavy content work, not mechanical. Do NOT attempt all 106 in one task. Migrate a meaningful, coherent subset (e.g. one full Settings tab's worth, or all of one page) as real, complete, verified work — and leave a clear, explicit follow-up list (code comment + your final report) of what's left, exactly matching how Phase 1's Task 9 handled its own "~35 remaining sites, not all of them" scope. Prioritize hints that are genuinely disposable-after-first-read (the spec's own test: "if something cannot be explained in a bubble, the control is wrong, not the label" — a hint that's actually load-bearing reference text a user re-reads often is a signal the CONTROL needs redesigning, not forcing into a bubble; skip those, don't force them).

**Verify:** live, hover AND focus reveal the bubble (already tested at the component level in Phase 1 — this task just needs to confirm real call sites wire it correctly), Escape closes, both themes.

---

## Task 5: Section headings as filled badges

**Files:** The ~21 section headings found by the original audit (`Settings.tsx`, `Config.tsx`, `Flash.tsx`, `Dashboard.tsx`, `ActivityLog.tsx`, `Containers.tsx`, `VMs.tsx`, `OffsiteWizard.tsx` — re-verify current locations) currently render as `.glim-eyebrow`-style uppercase caption text (correct per a DIFFERENT rule, but rule 11 wants headings specifically to be filled, coloured badges, not eyebrows). Also rule 13: "everything clickable is a badge, including links" — the original audit found 8 plain text links (`WhatsNewDialog.tsx`, `Settings.tsx`, `OffsiteWizard.tsx`, `Dashboard.tsx`, `ItemScheduleOverride.tsx`) that should become badge-styled.

**Judgment call, make it explicitly:** the Phase 1 whole-branch review explicitly flagged this as worth "deciding deliberately whether BombVault keeps the eyebrow treatment as a documented app-specific exception... rather than converting" (the spec's own last section allows this). Read both the eyebrow convention (currently used everywhere) and rule 11's badge requirement, decide which BombVault should actually do, and implement that decision consistently — don't do a partial conversion that leaves the app visually inconsistent between eyebrows and badges for the same kind of element.

**Verify:** live, both themes, visual consistency across every migrated heading/link.

---

## Task 6: RTL logical-property sweep

**Files:** Repo-wide. The original audit found 41 physical (`pl-`/`pr-`/`ml-`/`mr-`/`left-`/`right-`) vs. 2 logical (`ps-`/`pe-`) properties, and 36 `text-left`/`text-right` vs. 0 `text-start`/`text-end`, despite Arabic/Hebrew already being real, shipped, `dir="rtl"`-correct locales (`lib/i18n.ts` already sets `dir="rtl"` for them). Phase 1's own new components (`RevealInput`, `ToastViewport`) already correctly use logical properties — use those as your in-repo reference for the pattern, not just the external spec.

**The spec's explicit exceptions — do NOT logical-ize these:**
1. **Technical content stays pinned LTR** (`dir="ltr"` + `text-align: start`) — paths, URLs, filenames, API keys, log lines, cron expressions. Converting `text-left`→`text-start` on a field displaying a repo path would make it flip under RTL, which is exactly the bug the spec warns about. Identify every technical-content display field first (grep for monospace/`font-mono` fields, snapshot IDs, paths) and confirm they're `dir="ltr"` explicitly, not swept into logical properties.
2. **Directional icons mirror, symmetric ones don't.** An arrow/chevron/"forward" glyph gets `transform: scaleX(-1)` under `dir="rtl"`; a gear/trash-can/the reveal eye never does. Audit every icon touched by this sweep for which category it's in before touching its positioning.

**Verify:** per the spec's own explicit instruction — "verify RTL on the rendered page, not by reading the CSS: a logical property used correctly and one used incorrectly look identical in the source." Live-test in Arabic AND Hebrew, both themes, on every page. Confirm technical-content fields (paths, repo URLs, cron expressions, snapshot IDs, the APP_KEY-equivalent fields) stay pinned LTR and readable. Confirm directional icons flip and symmetric ones don't.

---

## Task 7: Resolve the fifth hue

**Files:** `web/src/index.css`'s `--status-info-*` tokens (currently blue, both themes) — Phase 1 deliberately left this alone because 63+ call sites keyed focus rings off it; Phase 1 Task 8 already converted all form-field focus to the brightness-step + dedicated `--field-focus-ring` token, and this Phase's Task 1/2 work will have converted list-row focus to hue-aware rainbow rings where applicable — so the dependency that blocked this in Phase 1 should now be resolved. Verify that's actually true (grep for remaining `--status-info-solid`/`statusInfoSolid` usages) before proceeding.

**What to do:** per rule 4 ("four state hues... never a fifth"), `--status-info-*` should not exist as a separate fixed hue — resolve every current "info" semantic use to either the accent (if it represents "this is active/notable, not a fixed state") or fold it into one of the four real state hues (settled/fault/warning/neutral) if it more honestly means one of those. Read every current `status-info`/`statusInfo` call site (there were ~8 info banners plus the "running"/"checking" chips per the original audit) and make a real semantic judgment call per site, not a blind find-replace to one new colour.

**Verify:** live, both themes, confirm no visual regression where "info" meant something real (e.g. a checking/in-progress state might genuinely want the accent, since accent = activity per rule 3), confirm nothing still points at a dangling `--status-info-*` token.

---

## Task 8: Complete the focus system

**Files:** `web/src/index.css`. Adopt the reference's base rule from `reference/tokens.css`: `:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px }` — Phase 1 took the `--focus-ring` token but never adopted this base rule, so ~215 buttons/links across the app currently fall through to the browser's native default focus outline instead of the app's own token. Also fix the 8 specific sites the Phase 1 whole-branch review found still painting the OLD blue `outline-statusInfoSolid` ring directly (`Containers.tsx`, `Dashboard.tsx`, `Settings.tsx` — re-verify current lines).

**Sequencing note:** if Task 7 (resolve the fifth hue) already ran, `--focus-ring`'s own value might need re-deriving depending on what happened to `--status-info-solid` — check for any accidental dependency before finalizing this task's values. If Task 2 (rainbow rows) already ran, some of these 215 buttons/links might now be inside a `.glim-hue` subtree and should pick up `--item-hue-ring` instead of the flat `--focus-ring` per the reference's own documented mechanism (`hueVars()`'s `--item-hue-ring` field exists specifically for this) — verify the base rule and the hue-subtree rule compose correctly (the hue rule should win inside a `.glim-hue` element, the base rule everywhere else).

**Verify:** live, Tab through a real page with a mix of plain buttons, links, and (if Task 2 ran) rainbow-hued rows — confirm every focusable element gets a real, visible, appropriately-coloured ring, confirm nothing regresses Phase 1's own field-focus brightness-step treatment (which is separate and should stay untouched).

---

## Task 9: Smaller tracked items (bundle what's cheap and coherent; defer what isn't)

Everything the Phase 1 whole-branch review found and tracked but didn't fix, plus the per-task deferrals that are still real:

1. **Type-scale sweep**: 47 hard-coded `text-[11px]` → `text-caption` (the equivalent token), matching the radius sweep's own precedent from Phase 1 Task 2. Check current count first, it may have shifted.
2. **Dashboard's raw untranslated status strings** (`Dashboard.tsx` — `"failed"`/`"ok"`/`"neutral"` rendered as literal badge text) — apply the exact same i18n fix Phase 1 Task 5 already did for `SpikePanel`'s identical bug class.
3. **Document the "Quiet toasts" setting** (Phase 1 Task 9 added it, appears in no README/docs/configuration.md/DOCKERHUB.md).
4. **5 unmigrated switch copies** (Phase 1 Task 4's deferral — `OffsiteWizard.tsx`, `Containers.tsx`, `Files.tsx`, `Settings.tsx`, `VMs.tsx`) — migrate to the real `Toggle` component now.
5. **i18n key de-duplication**: `common.confirm`/`common.cancel` vs. the older `restore.confirm`/`files.cancel`/`settingsIO.cancel`/`offsite.targets.cancel` — consolidate onto the canonical pair where the strings are genuinely identical, verify no locale has them meaningfully diverged first.
6. `docs/superpowers/specs/2026-07-07-v5-redesign-spec.md`'s stale `rounded-lg`/`StatusChip` references — add a one-line "superseded, historical" note at the top rather than rewriting it.

**Explicitly out of scope even for this task** (real content/asset work needing separate handling, not a quick sweep): regenerating README screenshots (needs actual new screenshots taken against the finished Phase 2 UI — do this as part of Final Ship below, not mid-plan), richer confirmation-dialog stake-copy (Phase 1 Task 7's own deferral, needs new interpolated i18n keys, real scope of its own), the ~35 remaining toast-adoption sites (Phase 1 Task 9's own deferral, per-site judgment work).

**Verify:** `npm run build`, `npx tsc --noEmit`, `npx vitest run`, `npm run lint` after each bundled fix.

---

## Final Live Verification & Ship

- [ ] Whole-branch review across all 9 tasks: token/CSS consistency, no orphaned pre-Task-2/3/5 classes, rainbow/reactive mode doesn't visually fight the RTL sweep or the resolved fifth hue, full fresh top-to-bottom live pass across every page in dark/light/system theme AND with rainbow on/off/reactive, AND in an RTL locale.
- [ ] `npm run build`, `npx tsc --noEmit`, `npm run lint`, `npx vitest run` clean. Go side clean if Task 1 ended up needing backend changes.
- [ ] Regenerate README screenshots against the finished UI (`.github/assets/dashboard.png`, `recovery.png`, `containers.png`, `settings.png`, `restore-demo.gif`) — flagged by Phase 1's review as predating that whole branch; now doubly true after Phase 2.
- [ ] Open PR, watch CI, ask before merge (per this project's established pattern), merge, sync local main, delete branch.
- [ ] Update the BombVault vault note marking the GlimStone integration fully complete (Phase 1 + Phase 2), noting any items THIS phase still had to defer.
