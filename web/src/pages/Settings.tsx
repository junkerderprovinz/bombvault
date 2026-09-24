import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { getAuth, getSettings, importSettingsApply, listContainers, listFileSets, listOffsiteTargets, listVMs, putSettings, setAuthPassword, type OffsiteTarget } from "../lib/api";
import { subscribeOffsiteTargets, type OffsiteDomain } from "../lib/useOffsiteTargets";
import { useNamedRepos } from "../lib/useNamedRepos";
import { useConfirm } from "../lib/useConfirm";
import { pushSaveWarnings } from "../lib/placementCodes";
import { directAsk, primaryDirects, retentionLowered } from "../lib/directRepo";
import { getLabelMode, type ControlAxis, type LabelMode } from "../lib/controls";
import { PAGE_SHELL_TABBED } from "../lib/pageShell";
import { Button } from "../components/Button";
import { useReveal } from "../lib/useReveal";
import type { Settings, Container, VM, FileSetView, RegistryAuthEntry } from "../lib/api";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { randomId } from "../lib/uuid";
import { useAdvanced } from "../lib/advanced";
import { getRainbow, setRainbow, type RainbowState } from "../lib/appearance";
import { getShape, type Shape } from "../lib/shape";
import { getMotionIntensity, type MotionIntensity } from "../lib/motion";
import { applyStoredDisco, discoTap, getDisco } from "../lib/disco";
import { HUE_OFFSET, Selector } from "../components/Selector";
// The integrity row's own two verbs ([324]). They live in the ACTION set
// rather than the nav one, same split IconUpload already crosses.
// The two tab glyphs that are generated rather than drawn below. Aliased so the
// wrappers further down keep their own names and TAB_ICON reads the same for
// all seven, generated and hand-drawn alike.
import {
  IconTabOffsite as IconTabOffsiteGlyph,
  IconTabSystem as IconTabSystemGlyph,
  IconTabIntegrity as IconTabIntegrityGlyph,
  IconTabStorage as IconTabStorageGlyph,
} from "../components/navGlyphs";
// IconUpload lives in the ACTION set while its twin IconDownload sits in the
// nav set, so the export/import pair ([293]) has to reach across both. Worth a
// line because the split is by generator file, not by meaning: `upload-box-1`
// and `download-box-1` are one Streamline drawing with the arrow reversed.
import type { SaveState } from "./settings/shared";
import { GeneralTab } from "./settings/tabs/GeneralTab";
import { StorageTab } from "./settings/tabs/StorageTab";
import { SchedulesTab } from "./settings/tabs/SchedulesTab";
import { OffsiteTab } from "./settings/tabs/OffsiteTab";
import { NotificationsTab } from "./settings/tabs/NotificationsTab";
import { IntegrityTab } from "./settings/tabs/IntegrityTab";
import { SystemTab } from "./settings/tabs/SystemTab";
import type {
  DomainToggleKey,
  MergedAutoSaveKey,
  OffsiteRetentionKey,
  ScheduleBoolKey,
  SettingsTabProps,
} from "./settings/tabs/types";



export function SaveBar({
  state,
  onSave,
  t,
  disabled = false,
}: {
  state: SaveState;
  /** Always null post-migration — see this component's header comment. */
  error?: string | null;
  onSave: () => void;
  t: ReturnType<typeof useT>["t"];
  disabled?: boolean;
}) {
  return (
    <div className="flex items-center gap-3 pt-1">
      <Button
        label={t("settings.save")}
        labelKey="settings.save"
        tone="accent"
        onClick={onSave}
        disabled={disabled || state === "saving"}
        busy={state === "saving"}
        title={state === "saving" ? t("common.saving") : undefined}
      />
    </div>
  );
}

// Accent preset swatches: DEFAULT_ACCENT_PRESETS (lib/accent.ts) now owns
// both the hex values AND the persistence/reset shape — see that module's
// own header comment. AccentCard/AccentPresetSwatch further down are the UI
// half.


// TabKey enumerates the 7 Settings tabs. The active tab is the single source of
// truth for which card group renders; SettingsPage owns all shared state so every
// tab shares one `settings`/`save()` instance regardless of which tab is visible.
type TabKey =
  | "general"
  | "storage"
  | "schedules"
  | "offsite"
  | "notifications"
  | "integrity"
  | "system";

/** The tab strip's own left-to-right order — the single source of truth for
 *  "later" vs "earlier" that both the deep-link hashchange effect (below)
 *  and the tab-slide direction (GlimStone motion-engine animation 7, its own
 *  call site further down) read, instead of each keeping its own duplicate
 *  literal list of the same seven keys. */
const TAB_ORDER: TabKey[] = [
  "general",
  "storage",
  "schedules",
  "offsite",
  "notifications",
  "integrity",
  "system",
];

// ---------------------------------------------------------------------------
// Settings tab icons (GlimStone form-engine Phase 2, Task 3 — design-language
// "top, with an icon": "Settings pages line their tabs up horizontally at the
// top, each with a glyph. A tab with no label is a gap; a tab with the wrong
// glyph is a lie — no icon beats the wrong one."). 16×16, stroke-based,
// matching Sidebar.tsx's own icon weight/style but at the tab strip's smaller
// scale. Local to Settings.tsx, not Sidebar.tsx's exported icon set: these
// name Settings' own SECTIONS (domain toggles, storage paths, cadences,
// off-site targets, alerts, integrity checks, system/SSH), which is a
// different taxonomy than the sidebar's page destinations, and none of the
// seven map onto an existing sidebar glyph without lying about what it is.
// ---------------------------------------------------------------------------
// FILLED (design-language.md "Icon glyphs" — every icon glyph is a solid
// shape, `fill="currentColor"`, never a stroked outline): all seven tab
// glyphs below were the last stroke-only holdouts in the app (GlimStone
// follow-up round, full-area sweep after IconFolder/IconCloud/the off-site
// action badges were fixed) — each redrawn using this section's own
// established techniques: a closed silhouette flips directly (rule 218,
// IconTabOffsite's cloud), a line glyph becomes a filled polygon (rule 219,
// the shield's checkmark), and a structural detail that has to stay thin
// (a switch track, a clock's hand, a slider's track) becomes a thin filled
// shape instead of a stroke (rule 220).
//
// REGRESSION FIX (jdp, live review — "die Icons der Einstellungstabs sind,
// wenn sie ausgewählt sind, bei manchen nicht mehr erkennbar"): that first
// redraw pass gave the "knob"/"hand"/"checkmark" detail on four of these
// seven (General, Schedules, Integrity, System) a SECOND colour —
// `fill="var(--carbon-surface, transparent)"` painted on top of the
// silhouette, standing in for what used to be a stroke's own natural gap.
// Verified live (Playwright, both themes, idle+selected, real running
// container build): that second colour is a fixed, THEME-scoped token,
// while the badge's own ink when SELECTED is `text-accentContrast` — a
// value derived from the accent colour alone, constant across both themes.
// In light theme the pairing (near-black ink, white surface) contrasts
// fine; in dark theme `--carbon-surface` is `#262626`, which sits right
// next to that same near-black `#161616` ink (measured contrast ratio
// ≈1.16:1 — nowhere near WCAG's 3:1 floor for a graphical detail) — the
// knob/hand/checkmark all but disappear into the icon's own fill the
// instant one of these four tabs is SELECTED in dark theme. Confirmed this
// never regressed Offsite/Notifications: neither ever used a second fill at
// all (a closed silhouette and a small solid tab, respectively).
//
// FIXED at the geometry level, not by picking a new hardcoded colour (a
// different literal would just move the same coincidence to some other
// accent/theme pairing later): each detail is now cut as REAL negative
// space — one compound `<path fill-rule="evenodd">` per icon, silhouette
// subpath plus detail subpath, so the "hole" is true transparency showing
// whatever the badge's own live background actually is. That background is
// by construction already the one thing this icon's `currentColor` ink is
// chosen to contrast against (bg-accent + text-accentContrast when
// selected, bg-carbon-surface2 + text-carbon-textSub when idle), so the cut
// reads clearly in every theme/state/hue this control can ever carry —
// including every rainbow-mode accent, not just the yellow default — with
// no second token to fall out of sync again.
// Each of these carries a viewBox cropped to its own INK, not the 0 0 16 16 it
// was drawn on ([285], jdp: "auf den settingstabs ist das offsite icon zu
// klein. da wirken die glyphen kleiner als auf den sidebar tabs").
//
// He was right and the box was not the reason: every one of these already
// rendered into the same 20px square as a rail glyph. What differed was how
// much of that square the drawing used. Measured live: the rail's Streamline
// glyphs fill 98-100% of their box, while these hand-drawn ones filled 69% to
// 88%, because each was drawn with a comfortable margin inside its 16-unit
// grid. Two glyphs of the same nominal size, one visibly smaller.
//
// A cropped viewBox fixes that without touching a single path coordinate: the
// numbers below are each glyph's measured ink box, squared off (side = the
// larger of width and height) and centred on the ink, so the default
// preserveAspectRatio="xMidYMid meet" scales it up to fill the box in its
// dominant dimension and leaves the aspect ratio alone. Cropping rather than
// redrawing also means the paths stay exactly the shapes that survived the
// earlier legibility rounds.
//
// The explicit width/height="15" is gone with it: `.glim-seg > svg` has set the
// real size since [241], so those attributes only documented a size that had
// not been true for a while.
function IconTabGeneral() {
  // Two stacked switches — the domain on/off toggles this tab actually holds.
  // Each pill + its knob is one evenodd path: the knob is a real cut-out,
  // not a second painted colour (see the fix note above this section).
  return (
    <svg viewBox="1 1 14 14" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M3,3 H9 A2,2 0 0 1 9,7 H3 A2,2 0 0 1 3,3 Z M9.15,5 A1.15,1.15 0 1 0 6.85,5 A1.15,1.15 0 1 0 9.15,5 Z M7,9 H13 A2,2 0 0 1 13,13 H7 A2,2 0 0 1 7,9 Z M9.15,11 A1.15,1.15 0 1 0 6.85,11 A1.15,1.15 0 1 0 9.15,11 Z"
      />
    </svg>
  );
}

function IconTabStorage() {
  // jdp's own file now ([320]), cropped to its measured ink like every
  // other imported glyph. The hand-drawn disk stack it replaces went
  // through three redraws chasing legibility; an icon needing that many
  // attempts is a better candidate for replacing than for a fourth
  // redraw. See scripts/gen_glyphs.py.
  return <IconTabStorageGlyph />;
}

function IconTabSchedules() {
  // A clock — cadence/timing. Dial + both hands as one evenodd path — the
  // hands are a real cut-out, not a second painted colour (see the fix note
  // above this section). Sharp-edged (not rounded-cap) hands: a deliberate
  // simplification over the old cutout's rounded rects, made so the
  // diagonal hour hand's four corners are exact rotated points instead of
  // needing rotated arc math — verified live, reads identically at this
  // icon's actual 15px size.
  return (
    <svg viewBox="1.8 1.8 12.4 12.4" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M14.2,8 A6.2,6.2 0 1 0 1.8,8 A6.2,6.2 0 1 0 14.2,8 Z M7.35,4.5 H8.65 V8.1 H7.35 Z M8.163,7.339 L11.506,9.347 L10.837,10.462 L7.494,8.453 Z"
      />
    </svg>
  );
}

function IconTabOffsite() {
  // Was a hand-drawn cloud silhouette; jdp asked for a nicer one. Now the
  // generated `cloud-data-transfer`, which is both a better-shaped cloud AND
  // says what this tab is: a cloud something is copied TO and FROM, not just
  // weather. See scripts/gen_glyphs.py.
  return <IconTabOffsiteGlyph />;
}

function IconTabNotifications() {
  // A bell — alerts. The bell body was already a closed silhouette (rule
  // 218 — direct flip). The clapper "ring" beneath it was a short open
  // stroke — redrawn as a small solid filled tab rather than a line.
  return (
    <svg viewBox="2.45 2.5 11.1 11.1" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M4 6.5a4 4 0 0 1 8 0c0 3 1 3.8 1 3.8H3s1-.8 1-3.8Z" />
      <path d="M6.5 12.1h3a1.5 1.5 0 0 1-3 0Z" />
    </svg>
  );
}

function IconTabIntegrity() {
  // jdp's own file now ([317]), a Material shield-check. Same silhouette
  // idea as the hand-drawn one it replaces, drawn by people who do this
  // for a living. See scripts/gen_glyphs.py.
  return <IconTabIntegrityGlyph />;
}

function IconTabSystem() {
  // Was inline sliders, which jdp read as too close to IconTabGeneral's
  // stacked toggles — two rounded horizontal bars with a knob each, at 20px
  // barely tellable apart. A chip is a different shape entirely and still
  // says "system". See scripts/gen_glyphs.py.
  return <IconTabSystemGlyph />;
}

export function keepRegistryAuths(
  auths: RegistryAuthEntry[],
  rowIds: string[]
): { auths: RegistryAuthEntry[]; rowIds: string[] } {
  const kept = auths
    .map((a, idx) => ({ a, idx }))
    .filter(
      ({ a }) =>
        a.host.trim() !== "" ||
        a.username.trim() !== "" ||
        a.token.trim() !== ""
    );
  return {
    auths: kept.map(({ a }) => ({
      ...a,
      tokenSet: a.tokenSet || a.token.trim() !== "",
    })),
    rowIds: kept.map(({ idx }) => rowIds[idx]),
  };
}

// markRegistryTokensStored — what the SCREEN should show once a registry save
// lands: EVERY row the user has, including the blank one they have only just
// added, with a freshly typed token marked "stored" so that field switches to
// its kept-placeholder. It is keepRegistryAuths' tokenSet half without the
// filter, and the two are deliberately separate functions rather than one with
// a flag: "what gets persisted" and "what stays under the cursor" are different
// questions, and merging them is what made a save delete a row.
//
// A blank row leaves the screen when the user removes it or reloads the page —
// nothing persisted it, so it does not come back. That is exactly what the
// add-row badge's own comment already promised.
export function markRegistryTokensStored(
  auths: RegistryAuthEntry[]
): RegistryAuthEntry[] {
  return auths.map((a) => ({
    ...a,
    tokenSet: a.tokenSet || a.token.trim() !== "",
  }));
}
const TAB_ICON: Record<TabKey, ReactNode> = {
  general: <IconTabGeneral />,
  storage: <IconTabStorage />,
  schedules: <IconTabSchedules />,
  offsite: <IconTabOffsite />,
  notifications: <IconTabNotifications />,
  integrity: <IconTabIntegrity />,
  system: <IconTabSystem />,
};

// keepRegistryAuths — what the SERVER should store for the Image Cleanup &
// Registries card: untouched blank rows dropped, and a freshly typed token
// marked "stored" so the field shows the kept-placeholder once the save lands.
// Pulled out as a standalone, exported function (no React, no `save()` side
// effect) so it's directly unit-testable without mounting SettingsPage — same
// "extract the pure decision, test it without a renderer" shape as isRemotePath
// (PathModeSwitch.tsx) and Selector.tsx's own nextFocusIndex/rovedIndex.
// `auths`/`rowIds` are always the SAME length and index-aligned by
// construction (every mutation site keeps them in lockstep) — the caller
// (saveRegistries below) passes the freshly computed pair rather than
// letting this function read component state directly.
//
// It answers for the PAYLOAD only. What stays on SCREEN is
// markRegistryTokensStored below, and keeping the two apart is the whole point:
// this filter used to run only when the user clicked the card's Save button,
// where "drop the blank rows" and "I am finished" meant the same thing. It now
// runs from every keystroke-triggered save, and applying it to the visible list
// deleted the row the user had just added and was about to fill in — a row
// vanishing from under the cursor because a typo was being corrected two rows
// up. A blank row is worth nothing to the server and everything to the person
// typing into it.

function offsiteRetentionOf(s: Settings) {
  return {
    retentionKeepLast: s.offsiteRetentionKeepLast,
    retentionKeepDaily: s.offsiteRetentionKeepDaily,
    retentionKeepWeekly: s.offsiteRetentionKeepWeekly,
    retentionKeepMonthly: s.offsiteRetentionKeepMonthly,
  };
}

export function SettingsPage() {
  const { t, lang } = useT();
  const { advanced } = useAdvanced();
  const { push, quiet, setQuiet } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const namedRepos = useNamedRepos();
  const [allTargets, setAllTargets] = useState<OffsiteTarget[]>([]);
  useEffect(() => {
    const load = () => {
      listOffsiteTargets()
        .then((r) => {
          if (r.ok) setAllTargets(r.targets ?? []);
        })
        .catch(() => undefined);
    };
    load();
    return subscribeOffsiteTargets(load);
  }, []);
  const fieldDirects = primaryDirects(allTargets, namedRepos);

  const [tab, setTab] = useState<TabKey>("general");
  // Settings tab slide (GlimStone motion-engine animation 7) — 1 = the tab
  // strip's onChange below just moved to a LATER tab (slide in from the
  // trailing edge), -1 = an EARLIER one. Computed synchronously in the SAME
  // event handler that calls setTab() (see that call site's own comment), so
  // by the time the tab-content wrapper remounts with the new `tab`, this
  // state has already committed alongside it in the same render. A ref, not
  // state, tracks the CURRENT tab for the hashchange effect below — that
  // effect only ever runs once (mount) and closes over a stale `tab`
  // otherwise; `setTab`/`setTabDir` themselves stay stable across renders
  // (React guarantees this), so only the VALUE read needs the ref, not the
  // setters.
  const [tabDir, setTabDir] = useState<1 | -1>(1);
  const tabRef = useRef<TabKey>(tab);
  useEffect(() => {
    tabRef.current = tab;
  }, [tab]);
  // Tab-strip width tracking (GlimStone follow-up pass, live-review round —
  // "the equal-width tab fix should match content, not stretch to fill" —
  // see Selector.tsx's own `equalWidth`/`stretch` header for the corrected
  // behaviour). The tab strip below now renders at its own hugged content
  // width (sum of 7 fixed, matched-to-the-widest-label segments) instead of
  // the page's full width, so the Card panels underneath it — which used to
  // rely on "both are plain, unconstrained full-width children, so they
  // match by construction" (see that wrapper's own comment) — need an
  // explicit width to track now that the strip is no longer full-width.
  // This width is measured, not guessed at, because the actual pixel value
  // depends on the active locale's longest label ("Benachrichtigungen" in
  // German is not the same width in every one of the 42 shipped locales) and
  // on the live font/zoom the browser is actually rendering with — nothing
  // about that is a fixed, hard-codable constant.
  //
  // A CALLBACK ref (state, not a plain useRef) — caught live, not in the
  // harness: this component has an early `if (!settings) return (<...
  // loading placeholder...>)` further down (before the tab strip's own JSX
  // even exists), so the FIRST commit of this component's lifetime never
  // renders the strip at all. A plain `useRef` + a mount-only
  // `useLayoutEffect(fn, [])` runs exactly once, against THAT first
  // (loading) commit, sees `tabStripRef.current === null`, and exits —
  // permanently, since an empty dependency array never re-fires once
  // `settings` later resolves and the real strip mounts. The ResizeObserver
  // then simply never gets attached, `tabStripWidth` stays `null` forever,
  // and the Card panels wrapper below silently never receives a max-width at
  // all (confirmed live: verified against a real deployed container at a
  // WIDE viewport where the strip fits on one line — the panels wrapper
  // rendered at <main>'s own full content width, not the narrower tab-strip
  // width, because no width was ever actually being applied; a narrower
  // viewport had merely LOOKED correct by coincidence, since the tab strip's
  // OWN `max-w-full` clamp and the panels wrapper's un-related default
  // full-width block sizing happened to resolve to the identical <main>
  // content-box number in that specific case). Storing the DOM node in STATE
  // via the ref CALLBACK below fixes this the standard React way: React
  // calls that callback exactly when the node is actually attached
  // (regardless of which render pass that happens on), so the effect below,
  // keyed on that state value, correctly (re-)runs once the strip genuinely
  // exists — not just once at this component's very first commit.
  const [tabStripEl, setTabStripEl] = useState<HTMLDivElement | null>(null);
  const [tabStripWidth, setTabStripWidth] = useState<number | null>(null);

  // ResizeObserver (not a resize-event listener): the strip's rendered width
  // can change WITHOUT the window resizing at all — a locale swap changes
  // every label's own natural width, and Selector's own two-pass measurement
  // effect (see its file header) settles onto a new matched width entirely
  // inside a layout-effect flush the window never hears about. Observing the
  // actual box directly catches both the window-resize case AND this one.
  // The ref is attached to a plain wrapping <div>, not `Selector` itself
  // (that component has no forwarded ref) — see the JSX below for why an
  // `inline-flex self-start` wrapper is also what makes this div hug the
  // strip's own content width rather than the page's full width in the
  // first place.
  useLayoutEffect(() => {
    if (!tabStripEl) return;
    const measure = () => setTabStripWidth(tabStripEl.getBoundingClientRect().width);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(tabStripEl);
    return () => ro.disconnect();
  }, [tabStripEl]);

  const [settings, setSettings] = useState<Settings | null>(null);
  // savedBaseline is the server's last-confirmed state. Every save persists its
  // own fields merged onto THIS baseline (not the live, possibly-edited
  // `settings`), so saving one field never silently commits another card's
  // unsaved edits.
  //
  // A REF, not state, and that is load-bearing rather than an optimisation. The
  // PUT is a FULL settings object, so a save's correctness depends on reading
  // the newest confirmed baseline at the moment the request is built. A state
  // value is frozen into the render that called save(), so two saves issued
  // inside one request round-trip both built their object from the pre-first
  // baseline and each overwrote the other's field with a stale value — the
  // later response winning the whole object. The ref is read at send time, and
  // queueSettingsWrite below makes sure "send time" is after the previous
  // write has landed and moved it.
  const savedBaseline = useRef<Settings | null>(null);
  // settingsWrites serializes every settings write this page makes: each save
  // (and the settings import) runs after the previous one has finished, so a
  // full-object PUT can never be built from a baseline another in-flight PUT is
  // about to invalidate. It is a promise chain rather than a busy flag because
  // nothing may be DROPPED — a debounced edit that arrives mid-flight has to
  // land, just afterwards.
  const settingsWrites = useRef<Promise<unknown>>(Promise.resolve());

  function queueSettingsWrite<T>(run: () => Promise<T>): Promise<T> {
    const next = settingsWrites.current.then(run);
    // The chain itself must never reject, or every later write would be
    // skipped: a failed save is reported by its own caller, not here.
    settingsWrites.current = next.then(
      () => undefined,
      () => undefined
    );
    return next;
  }
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  // The detected/overridden platform.Kind ("unraid" | "generic" | "truenas",
  // see internal/platform) — read-only host-environment info from GET
  // /api/settings' sibling "platform" field. Defaults to "unraid" (matching
  // the Go side's own nil-Platform default, platformFn()) so NotifyCard's
  // mismatch banner (below) never flashes on before this loads.
  const [platformKind, setPlatformKind] = useState<string>("unraid");
  const [loadError, setLoadError] = useState<string | null>(null);

  // Auth state for the Security card.
  const [authEnabled, setAuthEnabled] = useState(false);
  // The second factor's state, and the minimum the SERVER enforces. The
  // minimum is read rather than hard-coded so the field and the server can
  // never disagree about the number they both quote to the user.
  const [totpEnabled, setTotpEnabled] = useState(false);
  const [recoveryLeft, setRecoveryLeft] = useState<number | undefined>(undefined);
  const [minPasswordLen, setMinPasswordLen] = useState(12);
  const [pwNew, setPwNew] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwSaveState, setPwSaveState] = useState<SaveState>("idle");
  const [pwSaveMsg, setPwSaveMsg] = useState<string | null>(null);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"), found by
  // this same pass's own proactive sweep (not on the original finding list):
  // handleSetPassword is one of the FOUR documented genuine hold-outs on
  // manual Save (this function's own header comment) — its post-save failure
  // already pushes a toast, but nothing ever bumped a shake nonce for the
  // Save button. Same per-nonce mechanism as every other shake state on this
  // page (see ToggleRow's own shakeNonce doc comment).
  const [pwSaveShake, setPwSaveShake] = useState(0);
  const revealPwNew = useReveal();
  const revealPwConfirm = useReveal();
  const revealMetricsToken = useReveal();
  // Registry credentials are a per-row list (settings.registryAuths), and a
  // hook can't be called inside that row's own .map() callback (Rules of
  // Hooks — the call count would vary with the list length), so this is a
  // plain record here at the top level instead of a useReveal() per row.
  //
  // Keyed by a STABLE per-row id (registryRowIds below), NOT by array index.
  // Rows are removed/added by splicing settings.registryAuths, which shifts
  // every later row's index — an index-keyed record would then misattribute
  // a shifted-in row's slot to whatever reveal state the OLD occupant of that
  // index left behind (reveal row 0, remove row 0 → the row that slides into
  // index 0 renders already-revealed), and a freshly added row would inherit
  // whatever stale flag already lived at its new index. That's a real
  // secret-becomes-visible-without-being-asked-for bug, not just a cosmetic
  // one, so this is worth the extra bookkeeping below to get right.
  const [registryTokenVisible, setRegistryTokenVisible] = useState<Record<string, boolean>>({});
  // registryRowIds pairs 1:1 by index with settings.registryAuths, giving
  // each row a client-only stable identity to key registryTokenVisible (and
  // the row's React `key`) by — kept in lockstep at every place that changes
  // the array's length/order (load, add, remove, and the Save handler's
  // untouched-blank-row filter below). Deliberately NOT a field on the row
  // objects themselves: Settings PUT uses a strict decoder
  // (DisallowUnknownFields — internal/api/handlers.go) that must accept a
  // round-tripped GET body, so an extra client-only field riding along on a
  // spread entry would break every settings save, not just this card.
  const [registryRowIds, setRegistryRowIds] = useState<string[]>([]);

  // Accent colour state now lives entirely inside the exported AccentCard
  // component above (GlimStone follow-up pass, live-review round 6) — it
  // was never read anywhere else in this function, so nothing here needs to
  // track it any more.

  // Shape state (GlimStone form-engine — shape engine, the one axis both
  // prior GlimStone integration phases in this app deferred) — synced
  // to/from localStorage via shape.ts, the same pattern the old accentHex
  // state used before its move.
  const [shape, setShapeLocal] = useState<Shape>(() => getShape());

  // Motion-intensity state (GlimStone motion-engine — the deliberate
  // reversal of design-language.md's own prior "kein fünfter Nutzer-
  // Schalter" decision, see lib/motion.ts's own header) — synced to/from
  // localStorage via motion.ts, the identical pattern shape state above
  // already uses.
  const [motion, setMotionLocal] = useState<MotionIntensity>(() => getMotionIntensity());
  // GSS 1.17.0's hidden fourth level. Both of these are deliberately COMPONENT
  // state: `stormFound` must not survive leaving this page (an egg that
  // changes behaviour has to be switchable back off, never a permanent picker
  // entry), and the click counter has nothing to remember past the gesture.
  const [stormFound, setStormFound] = useState(false);
  const stormClicks = useRef({ taps: 0 });
  // Disco, the colour engine's own hidden mode, with the same two-part shape
  // the storm above uses: `discoFound` is component state so a found egg is
  // not a permanent row, and the counter has nothing to remember once the
  // gesture completes. Unlike the storm's, this counter carries a timestamp,
  // because its gesture is five turn-ONs of Rainbow Mode and somebody merely
  // comparing the mode on and off would otherwise unlock it by accident.
  const [discoFound, setDiscoFound] = useState(false);
  const [disco, setDiscoLocal] = useState<boolean>(() => getDisco());
  const discoClicks = useRef({ taps: 0, last: 0 });
  // #178: the three label modes, mirrored into local state so the selectors
  // show the current choice; the controls themselves read through
  // useLabelMode, which the labelModeChanged() call below wakes.
  const [labelModes, setLabelModes] = useState<Record<ControlAxis, LabelMode>>(() => ({
    buttons: getLabelMode("buttons"),
    sidebar: getLabelMode("sidebar"),
    tabs: getLabelMode("tabs"),
  }));

  // Rainbow state (GlimStone form-engine Phase 2, Task 1) — synced from/to
  // localStorage via appearance.ts, the same pattern as accentHex above.
  // setRainbow() persists + applies + returns the new (validated) state in
  // one call, so this only ever needs updating from that return value, never
  // a second localStorage read.
  const [rainbow, setRainbowLocal] = useState<RainbowState>(() => getRainbow());
  function updateRainbow(patch: Partial<RainbowState>) {
    setRainbowLocal(setRainbow(patch));
    // The disco walk reads the rainbow state, so a rainbow change has to
    // re-decide whether it runs: switching rainbow off parks it, switching
    // rainbow back on resumes it without touching the disco switch itself.
    applyStoredDisco();
  }

  /** Rainbow Mode's own onChange, which doubles as the disco unlock gesture:
   *  five turn-ons inside disco.ts's window. Only turn-ons count, so the
   *  gesture ends with rainbow on, which is the one state where a walking
   *  palette is visible at all. */
  function rainbowToggled(on: boolean) {
    updateRainbow({ on });
    if (discoTap(discoClicks.current, on, { now: Date.now() })) setDiscoFound(true);
  }

  // Per-section save state
  // Plain-export encryption / repository encryption (#28) — merged into one
  // auto-save card (GlimStone follow-up round, merge B): no SaveBar reads
  // these anymore, so only the setters survive, as the callback params
  // autoSaveField/debouncedSave still require — same "only the setters are
  // needed" shape as setDomSaveState/setDomSaveError above. Flash-zip-export's
  // own save state moved out along with the feature itself — see
  // FlashZipExportCard's own header comment for where it lives now.
  const [, setEncSaveState] = useState<SaveState>("idle");
  const [, setEncSaveError] = useState<string | null>(null);
  // Recovery-kit download refusal (e.g. the 403 "set a login password" fail-closed
  // answer when auth is off) — surfaced next to the download button.
  const [kitError, setKitError] = useState<string | null>(null);

  // Paths & off-site repo URLs — full-page Speichern-Button sweep: each field
  // now debounce-auto-saves itself (see the Paths/Off-site copy Cards' own
  // onChange handlers), so no SaveBar reads these anymore — only the setters
  // survive, as debouncedSave/save's own callback params still require. Same
  // "only the setters are needed" shape as setDomSaveState/setDomSaveError
  // above.
  const [, setPathSaveState] = useState<SaveState>("idle");
  const [, setPathSaveError] = useState<string | null>(null);
  const [, setExportEncSaveState] = useState<SaveState>("idle");
  const [, setExportEncSaveError] = useState<string | null>(null);
  const [, setOffsiteSaveState] = useState<SaveState>("idle");
  const [, setOffsiteSaveError] = useState<string | null>(null);
  // Which domain's guided off-site setup wizard is expanded (null = none).
  const [offsiteWizard, setOffsiteWizard] = useState<OffsiteDomain | null>(null);

  // Domains card (#142 — auto-save, no Speichern button): each row now saves
  // itself the instant it's clicked instead of batching into one SaveBar, so
  // there is no single "is the card saving" state left to show — only
  // setDomSaveState/setDomSaveError survive, as the two callback params the
  // shared save() helper still requires; nothing reads the values back
  // anymore. Same "only the setters are needed" shape as setTgtState/
  // setTgtError above (see that comment for the full reasoning) — save()'s
  // own toast already reports the outcome.
  const [, setDomSaveState] = useState<SaveState>("idle");
  const [, setDomSaveError] = useState<string | null>(null);
  // Per-row busy flag (disables that ONE toggle while its own request is in
  // flight) and shake-replay nonce (bumped on a rejected save so ToggleRow's
  // `.glim-shake` plays once more even on a second consecutive failure of the
  // SAME domain — see ToggleRow's shakeNonce doc comment). Both keyed by the
  // Settings field name, mirroring IncludeToggle.tsx's own per-row `busy`
  // state, adapted to a map since all 7 rows live inline in this one
  // component rather than as separate IncludeToggle instances.
  const [domainToggleBusy, setDomainToggleBusy] = useState<Partial<Record<DomainToggleKey, boolean>>>({});
  const [domainToggleShake, setDomainToggleShake] = useState<Partial<Record<DomainToggleKey, number>>>({});

  // Same "only the setters are needed" shape as pathSaveState above — this
  // retention grid's own SaveBar is gone too (each cell debounce-auto-saves).
  const [, setRetSaveState] = useState<SaveState>("idle");
  const [, setRetSaveError] = useState<string | null>(null);

  // Image cleanup / Unraid update-status reconciliation / registries (#56,
  // #116, #106) — merged into one auto-save card (GlimStone follow-up round,
  // merge A): no SaveBar reads these anymore, so only the setters survive, as
  // the callback params autoSaveField/saveRegistries still require — same
  // "only the setters are needed" shape as setDomSaveState/setDomSaveError
  // above (see that comment for the full reasoning).
  const [, setPruneSaveState] = useState<SaveState>("idle");
  const [, setPruneSaveError] = useState<string | null>(null);
  const [, setReconcileSaveState] = useState<SaveState>("idle");
  const [, setReconcileSaveError] = useState<string | null>(null);
  const [, setRegistrySaveState] = useState<SaveState>("idle");
  const [, setRegistrySaveError] = useState<string | null>(null);

  // Same "only the setters are needed" shape as pathSaveState above — every
  // field below debounce/toggle-auto-saves itself now, so nothing reads
  // these values back; save()'s own toast already reports the outcome.
  const [, setCacheSaveState] = useState<SaveState>("idle");
  const [, setCacheSaveError] = useState<string | null>(null);
  const [, setCoresSaveState] = useState<SaveState>("idle");
  const [, setCoresSaveError] = useState<string | null>(null);

  const [, setOffRetSaveState] = useState<SaveState>("idle");
  const [, setOffRetSaveError] = useState<string | null>(null);

  const [, setLimSaveState] = useState<SaveState>("idle");
  const [, setLimSaveError] = useState<string | null>(null);

  const [, setMetricsSaveState] = useState<SaveState>("idle");
  const [, setMetricsSaveError] = useState<string | null>(null);

  // Weekly digest (notifications tab) — persisted via the shared
  // baseline-merging save() (autoSaveToggle for the toggle, debouncedSave
  // for the cadence).
  const [, setDigestSaveState] = useState<SaveState>("idle");
  const [, setDigestSaveError] = useState<string | null>(null);

  // Overdue-backup watchdog (notifications tab) — same baseline-merging
  // save() as the digest card above it (autoSaveToggle).
  const [, setWatchdogSaveState] = useState<SaveState>("idle");
  const [, setWatchdogSaveError] = useState<string | null>(null);

  // Schedules tab (migrated from the retired Plans page). The container list
  // feeds the Containers schedule section's included-members list; syncSchedules
  // applies the Containers cadence to VMs + Flash + Folders.
  const [containers, setContainers] = useState<Container[]>([]);
  // VMs feed the VMs schedule section's per-item override list (#121).
  const [vms, setVMs] = useState<VM[]>([]);
  // File sets feed the Files schedule section's member list (live enabled toggles).
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);
  const [syncSchedules, setSyncSchedules] = useState(false);
  // Task 5 (live-review — "Speichern-Buttons können weg, es soll immer alles
  // live gespeichert werden"): this whole tab used to funnel every field
  // (buildSchedulePatch, now gone — see scheduleField/autoSaveScheduleField
  // below) into ONE bottom SaveBar keyed on this pair. No SaveBar reads them
  // anymore — only the setters survive, as the callback params save()/
  // debouncedSave still require. Same "only the setters are needed" shape as
  // setDomSaveState/setDomSaveError above (see that comment for the full
  // reasoning) — save()'s own toast already reports every outcome.
  const [, setSchedSaveState] = useState<SaveState>("idle");
  const [, setSchedSaveError] = useState<string | null>(null);
  // Task 5's plain-boolean half of the schedules-tab auto-save conversion:
  // perItemSchedules (Task 1's new ToggleRow), catchUpMissed (Missed
  // schedules Card) and RestoreChecksSection's own drillsEnabled/
  // offsiteDrillsEnabled — a dedicated key/map pair rather than widening
  // MergedAutoSaveKey/mergedFieldBusy/mergedFieldShake above: that type and
  // its two maps are named for, and documented against, the Paths & Storage
  // merge specifically, and folding an unrelated tab's fields into it would
  // make the name lie about what it covers. See autoSaveScheduleField below
  // for the actual save function (same optimistic-flip + revert + shake
  // shape as autoSaveField/toggleDomainEnabled).
  //   restartHealthWait joined this union in the full-page Speichern-Button
  // sweep (jdp, live review: "Die Speicher-Buttons sollen in allen Tabs weg.
  // Überall soll es automatisch speichern.") — it was the one field left on
  // this tab still batched into its own manual SaveBar (see the comment that
  // used to sit on restartSaveState/-Error below, now removed along with
  // that dead state). It genuinely belongs in THIS union, not a new one of
  // its own: same tab, same "single discrete boolean" shape as the other
  // four.
  const [schedFieldBusy, setSchedFieldBusy] = useState<Partial<Record<ScheduleBoolKey, boolean>>>({});
  const [schedFieldShake, setSchedFieldShake] = useState<Partial<Record<ScheduleBoolKey, number>>>({});
  // The "sync" toggle itself isn't a Settings field (syncSchedules above is
  // local UI state derived from whether the domain schedules already match),
  // so it can't go through autoSaveScheduleField's Settings-keyed generic —
  // see handleSyncSchedulesToggle below for its own dedicated busy/shake pair.
  const [syncToggleBusy, setSyncToggleBusy] = useState(false);
  const [syncToggleShake, setSyncToggleShake] = useState(0);
  // The Self-Backup Card's own on/off ToggleRow (jdp, live-review: "Selbst-
  // Backup-Zeitplan bitte mit Toggle für an/aus"). configSchedule is a
  // cadence STRING, same as containersSchedule/vmsSchedule/etc — not one of
  // autoSaveScheduleField's four plain ScheduleBoolKey booleans — so, like
  // the sync toggle above, it gets its own dedicated busy/shake pair rather
  // than widening that generic. See toggleConfigSchedule below.
  const [configScheduleToggleBusy, setConfigScheduleToggleBusy] = useState(false);
  const [configScheduleToggleShake, setConfigScheduleToggleShake] = useState(0);
  // Remembers the cadence in force before the self-backup schedule was switched
  // off, so switching it back on restores THAT instead of the shipped
  // daily-at-02:00 default. Same shape and same reason as the
  // FlashZipExportCard's rememberedKeep, which this toggle was missing: OFF
  // writes the literal "off" over the stored string, so a "weekly Sun 04:00" the
  // user had chosen existed nowhere afterwards and came back as a daily.
  //
  // Its reach is this page's lifetime, exactly like rememberedKeep's. The server
  // stores one cadence string per domain, so once "off" is saved the previous
  // value is genuinely gone and a reload cannot bring it back. Covering that
  // would take a second persisted field, i.e. a second source of truth for the
  // same fact, which is what toggleConfigSchedule's own comment rules out.
  const [rememberedConfigSchedule, setRememberedConfigSchedule] = useState("daily 02:00");

  // installSettings adopts a settings object the server just handed us as BOTH
  // the live state and the confirmed baseline, and re-derives the page state
  // that is computed from it. It is used by the mount load AND by the reload
  // after a settings import — the import replaces the whole configuration, so
  // the page has to adopt it exactly the way a fresh load would.
  function installSettings(s: Settings) {
    setSettings(s);
    savedBaseline.current = s;
    // Give every loaded registry row a stable client-only id (see
    // registryRowIds' declaration above) — a fresh GET never carries one of
    // its own, so one is minted here, once, per row. randomId() rather than
    // crypto.randomUUID(): the latter is secure-context-only and would throw
    // on BombVault's documented plain-HTTP origin, and a throw inside the
    // mount load lands in that promise's .catch — killing the whole Settings
    // page, not just this card (see lib/uuid.ts).
    setRegistryRowIds(s.registryAuths.map(() => randomId()));
    // Detect whether the domain schedules are already in sync (Containers ==
    // VMs == Flash == Folders, and not off), so the Schedules tab's sync
    // toggle reflects the server state. Reproduced from the retired Plans
    // page; filesSchedule is part of the comparison alongside Task 2's own
    // extension of the toggle's live effect to cover Folders too — without it,
    // a server state where Containers/VMs/Flash already matched but Folders
    // didn't would show the toggle ON while Folders still quietly held its own
    // independent value until the next edit.
    setSyncSchedules(
      s.vmsSchedule === s.containersSchedule &&
        s.flashSchedule === s.containersSchedule &&
        s.filesSchedule === s.containersSchedule &&
        s.containersSchedule !== "off" &&
        s.containersSchedule !== ""
    );
  }

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          installSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
          if (res.platform) setPlatformKind(res.platform);
        } else {
          setLoadError("Failed to load settings");
        }
      })
      .catch(() => setLoadError("Failed to load settings"));

    // Load auth status for the Security card.
    getAuth()
      .then((res) => {
        setAuthEnabled(res.enabled);
        setTotpEnabled(res.totp ?? false);
        setRecoveryLeft(res.recoveryCodesLeft);
        if (res.minPasswordLen) setMinPasswordLen(res.minPasswordLen);
      })
      .catch(() => {
        // Non-fatal: Security card shows auth as off.
      });

    // Load the container list for the Schedules tab's Containers section (its
    // included-members list). Non-fatal: an empty list just shows no members.
    listContainers()
      .then((r) => {
        if (r.ok) setContainers(r.containers ?? []);
      })
      .catch(() => {
        // Non-fatal: the Containers schedule section shows an empty member list.
      });

    // Load the VM list for the Schedules tab's VMs section per-item overrides (#121).
    listVMs()
      .then((r) => {
        if (r.ok) setVMs(r.vms ?? []);
      })
      .catch(() => {
        // Non-fatal: the VMs schedule section shows an empty per-item list.
      });

    // Load the file sets for the Schedules tab's Files section. Non-fatal too.
    loadFileSets();
  }, []);

  // loadFileSets (re)fetches the file-set list — on mount and after a Files
  // section toggle PATCHes a set, so the member rows track the server state.
  function loadFileSets() {
    listFileSets()
      .then((r) => {
        if (r.ok) setFileSets(r.fileSets ?? []);
      })
      .catch(() => {
        // Non-fatal: the Files schedule section shows an empty member list.
      });
  }

  // Deep-link support: /settings#offsite (and every other tab hash) selects the
  // matching tab instead of scrolling. Read once on mount, and also listen for
  // hashchange so an in-app "#offsite" link fired while already on /settings
  // switches the tab (no remount happens in that case). The Dashboard's
  // "Link to /settings#offsite" therefore lands on the Off-site tab.
  useEffect(() => {
    const applyHash = () => {
      const h = window.location.hash.replace(/^#/, "");
      if ((TAB_ORDER as string[]).includes(h)) {
        // Direction (motion-engine animation 7): computed the same way the
        // tab strip's own onChange below does, just reading the CURRENT tab
        // off tabRef instead of a closed-over (and here, permanently stale —
        // this effect only ever runs once, at mount) `tab` value.
        const from = TAB_ORDER.indexOf(tabRef.current);
        const to = TAB_ORDER.indexOf(h as TabKey);
        if (from !== -1 && to !== -1) setTabDir(to > from ? 1 : -1);
        setTab(h as TabKey);
      }
    };
    applyHash();
    window.addEventListener("hashchange", applyHash);
    return () => window.removeEventListener("hashchange", applyHash);
  }, []);

  // While "sync" is on, mirror the Containers cadence onto VMs + Flash +
  // Folders (Task 2 — "der toggle soll auch ordner einschließen") in live
  // state (not just in the save patch), so unchecking sync doesn't snap
  // those editors back to stale pre-sync values. The equality guard stops
  // re-renders from looping. Reproduced from the retired Plans page, with
  // filesSchedule folded in alongside the original vms/flash pair, and a
  // Task 5 auto-save persist: editing the Containers cadence WHILE synced
  // (e.g. typing in its cron field) used to only ever update local state,
  // relying on the bottom SaveBar to persist the mirrored fields later — now
  // debouncedSave (keyed "schedSync", independent of containersSchedule's
  // own "containersSchedule" debounce key below) coalesces rapid edits into
  // one PATCH of the three mirrored fields, 800ms after the last change.
  useEffect(() => {
    if (!syncSchedules || !settings) return;
    const merged = settings.containersSchedule;
    if (
      settings.vmsSchedule === merged &&
      settings.flashSchedule === merged &&
      settings.filesSchedule === merged
    ) {
      return;
    }
    setSettings((prev) =>
      prev ? { ...prev, vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged } : prev
    );
    debouncedSave("schedSync", () => {
      void save(
        { vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged },
        setSchedSaveState,
        setSchedSaveError
      );
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [syncSchedules, settings?.containersSchedule]);

  // ---------------------------------------------------------------------------
  // Generic save helper
  // ---------------------------------------------------------------------------

  // fieldPulse — GlimStone motion-engine, animation 2 (confirmation-pulse).
  // ONE shared map, keyed by Settings field name, covering every save() call
  // site at once (the same leverage this function's own header comment below
  // already describes for the toast/state-reset migration) — a ToggleRow
  // call site simply reads `fieldPulse.someKey` and passes it straight
  // through as `pulseNonce`, the exact same shape `domainToggleShake`/
  // `mergedFieldShake`/`fieldShake`/`schedFieldShake` already use for
  // `shakeNonce`, just bumped on the OPPOSITE outcome. Bumped for every key
  // in a successful patch, not just boolean/ToggleRow ones — a text/number
  // field's own entry here is simply never read by anything today, which is
  // harmless (this map costs nothing per unread key) and means a FUTURE
  // ToggleRow-backed field needs no new plumbing here to get the pulse, only
  // its own call site threading `pulseNonce={fieldPulse.thatKey}` through —
  // exactly the "wired once, works everywhere" outcome the standing
  // colour-engine rule asks for, applied to motion instead of colour.
  const [fieldPulse, setFieldPulse] = useState<Partial<Record<keyof Settings, number>>>({});

  // save persists one card's fields and returns true ONLY when the server confirmed
  // the write. Callers that gate a follow-up action on a confirmed save (e.g. the
  // off-site immutable toggle, which must not run a tamper test on a failed save)
  // await the boolean; fire-and-forget callers can still ignore it via `void`.
  //
  // GlimStone follow-up pass (v8.0.0): this is the ~21-site "SaveBar" chokepoint
  // Task 9 deliberately left alone (see lib/toast.tsx's own header comment) —
  // every card's Save button funnels through this ONE function (directly, or via
  // the `save` prop threaded into FleetSettingsCard/IntegrityCard/etc.), so
  // migrating it here migrates every one of those call sites at once, the same
  // way handleSetPassword/ConfigSettingsCard already did for their own single
  // completion notice. The 3000ms "saved"/"error" inline flash is gone — both
  // outcomes go through push() instead, and the state resets straight back to
  // "idle" (mirrors handleSetPassword's own pattern above). `setSaveError` is
  // still threaded through and always cleared to null: removing the parameter
  // would touch all ~21 call sites' signatures for zero behavioural gain (it was
  // only ever read by the now-deleted flash), so it stays as a harmless, always-
  // null vestige rather than a wide, risk-for-no-reason signature change.
  // Every save goes through queueSettingsWrite, so the object below is built
  // from a baseline no other in-flight write is about to change. Without that,
  // two saves issued inside one round-trip — which is the NORMAL case now that
  // every field auto-saves, e.g. editing the Containers cadence while "sync" is
  // on arms two 800ms debounces one render apart — each sent the other's field
  // at its pre-edit value, and whichever response landed last won the whole
  // object. The UI showed both edits; the server kept one.
  //
  // `echo` is for the one case where what the SERVER stores and what the SCREEN
  // shows are deliberately different objects: the caller passes a function that
  // is handed the LIVE settings at the moment the response lands and returns
  // what the screen should keep instead of the patch's own value. It has to be
  // a function, not a second object, because the round-trip is a window the user
  // keeps typing in — anything computed at send time is already stale by the
  // time it would be applied. Only saveRegistries needs it (see there).
  async function save(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void,
    echo?: (live: Settings) => Partial<Settings>
  ): Promise<boolean> {
    return queueSettingsWrite(() => sendSettingsPatch(patch, setSaveState, setSaveError, echo));
  }

  async function sendSettingsPatch(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void,
    echo?: (live: Settings) => Partial<Settings>
  ): Promise<boolean> {
    // Read at SEND time, not at call time: the previous write in the queue has
    // already advanced this ref by the time we get here.
    const base = savedBaseline.current ?? settings;
    if (!base) return false;
    setSaveState("saving");
    setSaveError(null);
    // Persist ONLY this card's fields, merged onto the server baseline — never the
    // live `settings`, which may hold unsaved edits from other cards.
    const updated: Settings = { ...base, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        // Advance the baseline; reflect just the saved fields in the live state so
        // other cards' in-progress edits are left untouched.
        savedBaseline.current = updated;
        setSettings((prev) =>
          prev ? { ...prev, ...patch, ...(echo ? echo(prev) : null) } : updated
        );
        setSaveState("idle");
        // Confirmation-pulse (GlimStone motion-engine animation 2) — bump
        // every key in THIS patch, not just the ones a ToggleRow happens to
        // read; see fieldPulse's own declaration comment above for why that
        // is deliberate rather than wasteful.
        setFieldPulse((p) => {
          const next = { ...p };
          for (const key of Object.keys(patch) as (keyof Settings)[]) {
            next[key] = (p[key] ?? 0) + 1;
          }
          return next;
        });
        // Tell the Layout/Sidebar to refetch so a newly enabled/disabled domain
        // tab appears or vanishes immediately — no page reload needed.
        window.dispatchEvent(new Event("bv:settings-changed"));
        push(t("settings.saved"), "success");
        pushSaveWarnings(push, t, res.warnings);
        return true;
      }
      setSaveState("idle");
      push(res.error ?? t("settings.error"), "fail");
      return false;
    } catch (err) {
      setSaveState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  // The global off-site retention is copied onto every domain's field target, so
  // a lowered value reaches each of their direct repositories at once.
  async function saveOffsiteRetention(key: OffsiteRetentionKey, n: number) {
    const before = savedBaseline.current;
    if (
      before &&
      fieldDirects.length > 0 &&
      retentionLowered(offsiteRetentionOf(before), offsiteRetentionOf({ ...before, [key]: n } as Settings)) &&
      !(await confirm(directAsk(t, lang, "offsite.directRetentionAsk", fieldDirects)))
    ) {
      setSettings((prev) => (prev ? { ...prev, [key]: before[key] } : prev));
      return;
    }
    await save({ [key]: n } as Partial<Settings>, setOffRetSaveState, setOffRetSaveError);
  }

  // toggleDomainEnabled (#142 — "Bei Domänen der Speichern-Button entfernen, es
  // soll automatisch speichern"): each Domains-card row saves ITSELF the instant
  // it's clicked, mirroring OffsiteWizard.tsx's toggleImmutable — the one other
  // place in this app already does "flip a single boolean settings field the
  // moment its switch is touched, no batching Save button": optimistic
  // setSettings flip, then the shared save() helper above (which already
  // merges onto the confirmed baseline, persists, dispatches
  // "bv:settings-changed" so Layout/Sidebar re-fetch and the tab appears/
  // disappears live, and pushes the toast) — never a new persistence path.
  //
  // A rejected save (e.g. enabling VMs with no working SSH connection to the
  // libvirt host — internal/api/handlers.go's handlePutSettings checks that
  // OFF→ON transition specifically) rolls the optimistic flip back to
  // whatever it was before this click and bumps this row's shake nonce so
  // ToggleRow replays `.glim-shake` — generic by construction: it keys off
  // `!ok`, not off which domain or why the backend refused, so ANY domain's
  // enable failing for ANY reason gets the same revert + shake + toast.
  async function toggleDomainEnabled(key: DomainToggleKey, next: boolean) {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setDomainToggleBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setDomSaveState, setDomSaveError);
    setDomainToggleBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      // Roll back to the pre-click state; save() already pushed the reason.
      setSettings((s) => (s ? { ...s, [key]: prev ?? !next } : s));
      setDomainToggleShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
  }

  // autoSaveField (GlimStone follow-up round, Paths & Storage tab rework,
  // merge A/B — "no Speichern button, every field auto-saves"): the SAME
  // optimistic-flip + persist + revert-on-failure shape toggleDomainEnabled
  // above already established, generalized from "one of 7 domain booleans"
  // to any single Settings field the two merged cards' own toggles need.
  // Busy/shake are keyed by field name, same map shape as
  // domainToggleBusy/domainToggleShake above, just covering a different,
  // smaller set of keys (the merged cards' own toggles, not the 7 domains).
  // "flashZipExportEnabled"/"flashZipExportKeep" moved out of this union along
  // with the feature itself — FlashZipExportCard now owns its own busy/shake
  // state independently (see that component's own header comment).
  const [mergedFieldBusy, setMergedFieldBusy] = useState<Partial<Record<MergedAutoSaveKey, boolean>>>({});
  const [mergedFieldShake, setMergedFieldShake] = useState<Partial<Record<MergedAutoSaveKey, number>>>({});

  async function autoSaveField<K extends MergedAutoSaveKey>(
    key: K,
    next: Settings[K],
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ): Promise<boolean> {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setMergedFieldBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setSaveState, setSaveError);
    setMergedFieldBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      // Roll back to the pre-click state; save() already pushed the reason —
      // meaningful for a toggle (a boolean has an obvious "before" to revert
      // to); the merged cards' free-text/number fields below use
      // debouncedSave instead, which intentionally has no revert (see that
      // function's own comment).
      setSettings((s) => (s ? { ...s, [key]: prev as Settings[K] } : s));
      setMergedFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
    return ok;
  }

  // autoSaveToggle (full-page Speichern-Button sweep, jdp, live review,
  // emphatic: "Die Speicher-Buttons sollen in allen Tabs weg. Überall soll
  // es automatisch speichern."): the SAME optimistic-flip + persist +
  // revert-on-failure + shake shape as autoSaveField/toggleDomainEnabled/
  // autoSaveScheduleField above, generalized ONE step further — those three
  // each own a small, closed key union (MergedAutoSaveKey/DomainToggleKey/
  // ScheduleBoolKey) because each covers a specific GROUP of related toggles
  // on one shared Card/tab, worth naming as a set. This sweep's remaining
  // holdouts (Monitoring's metricsEnabled, the Weekly-digest Card's
  // digestEnabled, the Overdue-watchdog Card's watchdogEnabled) are three
  // unrelated, standalone toggles on three different Cards across two
  // different tabs — inventing a same-shaped one-member union per Card would
  // just be MergedAutoSaveKey's own pattern copy-pasted three times for zero
  // benefit, so this widens the generic to any boolean Settings key instead,
  // with its own single shared busy/shake map keyed by field name (same
  // "keyed by field name in one map" shape every other per-field map on this
  // page already uses). Reach for one of the narrower, named unions above
  // instead when a NEW group of related toggles arrives together; reach for
  // this one for a standalone toggle that doesn't belong to any such group.
  const [fieldBusy, setFieldBusy] = useState<Partial<Record<keyof Settings, boolean>>>({});
  const [fieldShake, setFieldShake] = useState<Partial<Record<keyof Settings, number>>>({});

  async function autoSaveToggle<K extends keyof Settings>(
    key: K,
    next: Settings[K],
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ): Promise<boolean> {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setFieldBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setSaveState, setSaveError);
    setFieldBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      setSettings((s) => (s ? { ...s, [key]: prev as Settings[K] } : s));
      setFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
    return ok;
  }

  // debouncedSave/cancelDebounce — the free-text/number half of the same
  // merge A/B auto-save requirement: a registry host/username/token, a flash
  // zip export path/keep-count, or the age recipients list would be wasteful
  // (or outright annoying, mid-keystroke) to persist on every single change,
  // so these fire `run` DELAY_MS after the last edit to the same `key`
  // instead of immediately. Deliberately NO revert-on-failure here (unlike
  // autoSaveField above) — save()'s own toast already reports a failure, and
  // reverting a text field the user might still be actively typing into
  // would be jarring rather than helpful; the value simply stays as typed
  // and the next edit (or a page reload) gets another chance to save it.
  // Keyed by a caller-chosen string (not a Settings field name) so ONE
  // debounce line can cover several fields that only make sense saved
  // together (e.g. every registry-row edit shares the "registryAuths" key,
  // since they all resolve to the SAME registryAuths patch).
  // Each entry keeps the pending WRITE next to its timer, not just the timer
  // handle. That is what makes the edit recoverable: a debounce can then be
  // completed early (flushDebounces) instead of only being cancelled, which is
  // the difference between "the user's last edit is sent" and "it is gone".
  type PendingWrite = { timer: ReturnType<typeof setTimeout>; run: () => void };
  const debounceTimers = useRef<Record<string, PendingWrite>>({});
  const DEBOUNCE_MS = 800;
  // importing is true for the WHOLE import window — from the click that starts
  // an import until its re-loaded configuration has been installed — not only
  // for the stretch the import spends at the head of the write queue. See
  // applyImportedSettings for what it protects.
  const importing = useRef(false);

  function debouncedSave(key: string, run: () => void) {
    // An import is replacing the configuration this edit was typed against, so
    // arming it would only queue a write that lands on top of the imported one.
    // Dropped rather than deferred, for the same reason cancelAllDebounces
    // drops the edits that were already armed when the import started.
    if (importing.current) return;
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing.timer);
    debounceTimers.current[key] = {
      run,
      timer: setTimeout(() => {
        delete debounceTimers.current[key];
        run();
      }, DEBOUNCE_MS),
    };
  }

  function cancelDebounce(key: string) {
    const existing = debounceTimers.current[key];
    if (existing) {
      clearTimeout(existing.timer);
      delete debounceTimers.current[key];
    }
  }

  // flushDebounces sends every pending edit NOW instead of waiting out its
  // remaining delay. Cancelling and flushing are opposites and the page needs
  // both: an import replaces the configuration a pending edit was typed against,
  // so that edit must be dropped (cancelAllDebounces); leaving the page does not
  // invalidate anything, so those edits must be sent.
  //
  // Entries are removed from the map before their write runs, so a flush can
  // never double-send and a write that queues another edit is not re-collected.
  // The map object itself is mutated in place, never replaced — see the unmount
  // effect, which captures it.
  function flushDebounces() {
    for (const key of Object.keys(debounceTimers.current)) {
      const pending = debounceTimers.current[key];
      delete debounceTimers.current[key];
      clearTimeout(pending.timer);
      pending.run();
    }
  }

  // cancelAllDebounces drops every pending edit that has not been sent yet.
  // The one caller is the settings import: it replaces the whole configuration,
  // so a debounce armed seconds earlier would land on top of the imported
  // config with a value the user typed against the OLD one. It closes the first
  // half of that window only — the `importing` guard above closes the rest.
  function cancelAllDebounces() {
    for (const key of Object.keys(debounceTimers.current)) cancelDebounce(key);
  }

  // applyImportedSettings is the Import button's actual write. An import is a
  // settings write like any other, so it joins the SAME queue every save uses
  // and then re-loads the page from the server.
  //
  // Both halves matter. The card used to call the client directly and report
  // "Settings imported." — while the page kept its pre-import baseline, which
  // every field on the page merges its own value onto. One click on any toggle
  // afterwards therefore PUT the whole PRE-import object back and silently
  // undid the entire import, with the UI reporting a successful save. (On main
  // that needed a Save-button click; once every field auto-saves, it is one
  // stray click.) Queueing it stops a save that was already in flight from
  // landing on top of the fresh configuration, and cancelling the pending
  // debounces drops edits typed against the configuration that has just been
  // replaced.
  //
  // If the re-load fails, the page refuses to keep working from a baseline it
  // knows is stale: it shows the load error instead, which unmounts every card
  // and makes a stale-baseline save impossible. Reloading the browser is then
  // the honest recovery, and the import itself has already been applied.
  //
  // The debounces are dropped HERE, at the click, and not inside the queued
  // body below. Queued, the drop happens whenever the import reaches the head
  // of the write queue, which can be a whole round-trip later: a save already
  // in flight holds the import back, the 800ms debounce armed just before the
  // click elapses in the meantime, and its write is appended to the queue
  // BEHIND the import. It then lands on the freshly imported configuration
  // carrying the value the user typed against the one that was just replaced.
  // Cancelling at the click empties the map before anything can queue itself,
  // and `importing` keeps it empty for the rest of the window — otherwise a
  // keystroke during the apply would simply re-open the same hole.
  async function applyImportedSettings(fileText: string) {
    importing.current = true;
    cancelAllDebounces();
    return queueSettingsWrite(async () => {
      try {
        const res = await importSettingsApply(fileText);
        if (!res.ok) return res;
        const fresh = await getSettings();
        if (fresh.ok) {
          installSettings(fresh.settings);
          if (fresh.hostMountRoot) setHostMountRoot(fresh.hostMountRoot);
          if (fresh.platform) setPlatformKind(fresh.platform);
        } else {
          setLoadError("Settings were imported, but reloading them failed — reload the page.");
        }
        // Domains may have been switched on or off by the import: the sidebar and
        // layout listen for this and refetch, exactly as they do after a save.
        window.dispatchEvent(new Event("bv:settings-changed"));
        return res;
      } finally {
        importing.current = false;
      }
    });
  }

  // Leaving the page COMMITS the pending edits; it does not discard them.
  //
  // This cleanup used to clear every timer. With the Save buttons gone, the
  // debounce is the only thing that ever writes a text field, so clearing it
  // threw the user's last edit away: type a new cron expression into the
  // off-site cadence field (or a registry host, or the flash-zip export path),
  // click "Dashboard" within 800ms, and the PATCH never happened — no toast, no
  // error, and the value the field had shown as accepted was gone on return.
  // The old justification, "must not call setSettings/save with stale
  // closures", does not hold: scheduleField/debouncedSave capture their value
  // explicitly, and a setState after unmount is a no-op in React 18. Nothing was
  // being protected, and an edit was being lost.
  //
  // Flushing also matches the four card-level debounce maps in this file
  // (FlashZipExportCard, FleetSettingsCard, CloudCard, NotifyCard), none of
  // which cancel on unmount — so those already complete their pending write.
  // This page was the one place that did not.
  //
  // The flush itself is flushDebounces, captured into a local so the cleanup
  // closes over the function it had at mount — the plain, lint-satisfying
  // version of the same "don't reach for a fresh binding inside a cleanup" rule.
  // It reads debounceTimers.current, whose object identity never changes (only
  // its properties are mutated in place by debouncedSave/cancelDebounce/
  // flushDebounces above), so the entries it finds are the live ones.
  const flushOnUnmount = flushDebounces;
  useEffect(() => {
    return () => {
      flushOnUnmount();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount/unmount only: this must flush when the page GOES AWAY, not on every render that redefines the closure.
  }, []);

  // saveRegistries — the merge A registries sub-section's own save, shared by
  // both the debounced per-field edit path and the immediate Remove-row path
  // (see the Card below). It works against an EXPLICIT (auths, rowIds) pair
  // rather than reading `settings`/`registryRowIds` directly — both callers
  // already have the freshly computed arrays in hand (the state update and this
  // save race the same render otherwise), so passing them in avoids acting on a
  // one-render-stale snapshot.
  //
  // The payload and the screen are answered separately, and that separation is
  // the fix for a row disappearing mid-edit. The PUT carries the trimmed list
  // (keepRegistryAuths — a blank row is nothing the server should store); the
  // visible list keeps every row (markRegistryTokensStored), so a blank row the
  // user just added survives a save triggered by a keystroke in a DIFFERENT row.
  // Under the old batched Save button the two coincided, because clicking Save
  // meant "I am finished"; a debounce firing 800ms after a keystroke does not.
  //
  // The screen's half is handed to save() as its `echo` (see there) instead of
  // being computed here and applied afterwards, and that is the difference
  // between the two lists staying apart and the round-trip eating an edit. The
  // PUT takes a whole response to come back, and the user keeps typing in that
  // window: a list frozen at send time no longer describes the card by the time
  // it lands, so writing it back deleted a row added mid-flight and reverted
  // characters typed mid-flight — while registryRowIds, which nothing here
  // touches, kept the entry for the row that had just been erased and stopped
  // being index-aligned with the rows on screen. Marking the LIVE list keeps
  // both invariants: rows only ever leave the screen when the user removes
  // them, so the ids stay aligned by construction.
  function saveRegistries(nextAuths: RegistryAuthEntry[], nextRowIds: string[]) {
    const { auths } = keepRegistryAuths(nextAuths, nextRowIds);
    void save(
      { registryAuths: auths },
      setRegistrySaveState,
      setRegistrySaveError,
      (live) => ({ registryAuths: markRegistryTokensStored(live.registryAuths) })
    );
  }

  // Task 5 (live-review — "Speichern-Buttons können weg, es soll immer alles
  // live gespeichert werden"): replaces buildSchedulePatch + the Schedules
  // tab's one bottom SaveBar that used to persist every field in this comment's
  // old list (Containers/VMs/Flash/Folders cadences, the drills Card, every
  // offsite cadence, the self-backup cadence, tamperTestSchedule,
  // catchUpMissed, perItemSchedules) in a single manually-triggered PATCH.
  // Every one of those fields now saves itself the instant it changes, via
  // one of the three helpers below — the exact same "optimistic update,
  // immediate PATCH, revert + `.glim-shake` on failure" shape already proven
  // by toggleDomainEnabled (Domains card) and autoSaveField (Paths & Storage
  // merge cards), applied to a click/selection; or, for anything that fires
  // onChange on every keystroke (a raw cron `<input>`, CadenceBuilder's own
  // time/number/cron sub-fields), the debounced-no-revert shape those same
  // merge cards already established for free text.
  //
  // scheduleField — every CadenceBuilder-driven cadence AND every plain
  // offsite/self-backup cron <input> in this tab: optimistic setSettings +
  // a debouncedSave keyed by the field name, so rapid changes to the SAME
  // field (typing a cron expression, dragging through time-picker values)
  // coalesce into one PATCH 800ms after the last one, matching
  // debouncedSave's own "no revert — a user may still be typing, and save()'s
  // toast already reports a failure" reasoning above.
  function scheduleField<K extends keyof Settings>(key: K, value: Settings[K]) {
    setSettings((prev) => (prev ? { ...prev, [key]: value } : prev));
    debouncedSave(String(key), () => {
      void save({ [key]: value } as Partial<Settings>, setSchedSaveState, setSchedSaveError);
    });
  }

  // autoSaveScheduleField — the four plain booleans left in this tab
  // (perItemSchedules, catchUpMissed, drillsEnabled, offsiteDrillsEnabled):
  // a single discrete click, not continuous typing, so it gets the immediate
  // save + revert-on-failure + shake treatment instead, identical in shape to
  // autoSaveField above (see schedFieldBusy/schedFieldShake's own doc for why
  // this is a dedicated map rather than widening MergedAutoSaveKey).
  async function autoSaveScheduleField<K extends ScheduleBoolKey>(key: K, next: Settings[K]): Promise<boolean> {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setSchedFieldBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setSchedSaveState, setSchedSaveError);
    setSchedFieldBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      setSettings((s) => (s ? { ...s, [key]: prev as Settings[K] } : s));
      setSchedFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
    return ok;
  }

  // scheduleUpdate — RestoreChecksSection's own `update` prop, unchanged in
  // shape (a Partial<Settings> patch, always exactly one key from that
  // component's own four call sites) but now dispatching each key through
  // the right one of the two helpers above instead of a bare setSettings
  // merge: its two ToggleRows (drillsEnabled/offsiteDrillsEnabled) auto-save
  // immediately with revert+shake, its CadenceBuilder and % number field
  // debounce like every other cadence/text field in this tab.
  function scheduleUpdate(patch: Partial<Settings>) {
    for (const [key, value] of Object.entries(patch) as [keyof Settings, Settings[keyof Settings]][]) {
      if (key === "drillsEnabled" || key === "offsiteDrillsEnabled") {
        void autoSaveScheduleField(key, value as boolean);
      } else {
        scheduleField(key, value);
      }
    }
  }

  // handleSyncSchedulesToggle — the "sync" toggle's own save. Unlike the
  // Settings fields above, `syncSchedules` is local UI state (derived on
  // load from whether the domain schedules already matched, see the load
  // effect's own comment), not a field the backend stores directly, so it
  // cannot go through autoSaveScheduleField's Settings-keyed generic.
  // Flipping it ON is the one case with real, immediate side effects worth
  // persisting right away: VMs/Flash/Folders adopt the Containers cadence
  // this instant, mirroring toggleDomainEnabled's optimistic-flip +
  // save + revert-on-failure + shake shape, just against three fields at
  // once instead of one. Flipping it OFF persists nothing new — every field
  // already holds its own last-saved value, so there is nothing to PATCH;
  // subsequent edits to VMs/Flash/Folders individually go through
  // scheduleField again on their own, same as before sync was ever turned on.
  async function handleSyncSchedulesToggle(next: boolean) {
    setSyncSchedules(next);
    if (!next || !settings) return;
    const merged = settings.containersSchedule;
    const prevVms = settings.vmsSchedule;
    const prevFlash = settings.flashSchedule;
    const prevFiles = settings.filesSchedule;
    setSettings((s) =>
      s ? { ...s, vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged } : s
    );
    setSyncToggleBusy(true);
    const ok = await save(
      { vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged },
      setSchedSaveState,
      setSchedSaveError
    );
    setSyncToggleBusy(false);
    if (!ok) {
      setSyncSchedules(false);
      setSettings((s) =>
        s ? { ...s, vmsSchedule: prevVms, flashSchedule: prevFlash, filesSchedule: prevFiles } : s
      );
      setSyncToggleShake((n) => n + 1);
    }
  }

  // toggleConfigSchedule — the Self-Backup Card's own on/off ToggleRow. Same
  // immediate optimistic-flip + save + revert-on-failure + shake shape as
  // handleSyncSchedulesToggle above, applied to configSchedule's cadence
  // string instead of the three VMs/Flash/Folders fields that one touches.
  // OFF writes the literal "off" cadence string; ON restores the cadence that
  // was in force before the last OFF (rememberedConfigSchedule above), falling
  // back to "daily 02:00" when there is none to restore — the same
  // daily-at-02:00 baseline this grammar already uses elsewhere, e.g.
  // ContainersSchedule's own portable-settings test fixture. This
  // does NOT introduce a new configScheduleEnabled field: parseCadenceString/
  // buildCadenceString (CadenceBuilder.tsx) already round-trip "off"/""
  // through CadenceMode "off" cleanly, so a second boolean would just be a
  // second source of truth for the exact same fact. (BombVault's separate
  // `configEnabled` field, toggled in the Domains card above, is a different
  // concept — whether the self-backup domain exists at all — left untouched.)
  async function toggleConfigSchedule(next: boolean) {
    const prev = settings?.configSchedule ?? "off";
    // Switching OFF is the only moment the cadence is lost, and `prev` is
    // exactly the value being overwritten — whether it came from the server, the
    // CadenceBuilder below, or an earlier flip of this toggle.
    if (!next && prev && prev !== "off") setRememberedConfigSchedule(prev);
    const value = next ? rememberedConfigSchedule : "off";
    setSettings((s) => (s ? { ...s, configSchedule: value } : s));
    setConfigScheduleToggleBusy(true);
    const ok = await save({ configSchedule: value }, setSchedSaveState, setSchedSaveError);
    setConfigScheduleToggleBusy(false);
    if (!ok) {
      setSettings((s) => (s ? { ...s, configSchedule: prev } : s));
      setConfigScheduleToggleShake((n) => n + 1);
    }
  }

  if (loadError) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-statusFail">{loadError}</p>
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Auth / Security helpers
  // ---------------------------------------------------------------------------

  // GlimStone form-engine Task 9 (toasts): the SaveBar success/error pattern
  // here used to hold "saved"/"error" in pwSaveState for a 3000ms inline-text
  // flash. The two ASYNC completion notices (did setAuthPassword succeed)
  // now go through a toast instead — but the pre-flight mismatch check below
  // deliberately stays exactly as it was: it's a field-validation error the
  // user is actively looking at (both password fields, mid-edit), not a
  // "did the save finish" notice, so it keeps its own persistent inline
  // surface rather than a 4-second toast that could vanish while they're
  // still typing (design-language.md: a toast duplicating a surface that
  // already exists and is meant to persist is the wrong tool here).
  //
  // GENUINE EXCEPTION to the full-page Speichern-Button sweep (jdp, live
  // review, emphatic: "Die Speicher-Buttons sollen in allen Tabs weg...
  // Nur dort sollen Speicher-Buttons bleiben, wo es unbedingt sein muss."):
  // this Save button stays. Two independent hard reasons, either one alone
  // would qualify: (1) it requires TWO fields (new + confirm) to agree
  // before the write is even safe to attempt — exactly the "requires
  // explicit two-step confirmation for safety" exception named in the
  // sweep's own criteria, and there is no sane per-keystroke auto-save
  // trigger for a two-field agreement check (auto-saving on the FIRST
  // field alone, before the second is even filled in, would either save a
  // password the user never finished typing or spam a mismatch failure on
  // every keystroke of the second field). (2) setAuthPassword takes effect
  // IMMEDIATELY and controls access to the whole instance — a blank save
  // disables auth entirely — the closest thing on this page to "immediately
  // rotating a live credential," the sweep's own worked example of a
  // legitimate hold-out. Not kept because it's "security-related" in a
  // vague sense; kept because auto-saving would either be unsafe or simply
  // impossible to trigger correctly.
  async function handleSetPassword() {
    if (pwNew !== pwConfirm) {
      setPwSaveMsg(t("auth.passwordMismatch"));
      setPwSaveState("error");
      return;
    }
    // The server refuses a short password too, and its answer is what the
    // user would eventually see. Checking here as well saves a round trip and
    // puts the message next to the field instead of in a toast. An EMPTY
    // password is not "too short": it means "switch authentication off".
    if (pwNew !== "" && [...pwNew].length < minPasswordLen) {
      setPwSaveMsg(t("auth.passwordMinHint", minPasswordLen));
      setPwSaveState("error");
      setPwSaveShake((n) => n + 1);
      return;
    }
    setPwSaveState("saving");
    setPwSaveMsg(null);
    try {
      const res = await setAuthPassword(pwNew);
      if (res.ok) {
        setAuthEnabled(res.enabled ?? false);
        // The same response carries the session, so the card can go straight to
        // its signed-in state. Without this the second factor sat one reload
        // away: the enable button was there, and the request behind it answered
        // 401 because the login it had just switched on had issued nobody a
        // session yet.
        setPwSaveState("idle");
        push(pwNew === "" ? t("auth.passwordCleared") : t("auth.passwordSaved"), "success");
        setPwNew("");
        setPwConfirm("");
      } else {
        setPwSaveState("idle");
        push(res.error ?? t("auth.saveError"), "fail");
        setPwSaveShake((n) => n + 1);
      }
    } catch {
      setPwSaveState("idle");
      push(t("auth.saveError"), "fail");
      setPwSaveShake((n) => n + 1);
    }
  }

  const tabProps: SettingsTabProps = {
    t,
    advanced,
    quiet,
    setQuiet,
    allTargets,
    fieldDirects,
    settings,
    setSettings,
    savedBaseline,
    hostMountRoot,
    platformKind,
    authEnabled,
    totpEnabled,
    setTotpEnabled,
    recoveryLeft,
    setRecoveryLeft,
    minPasswordLen,
    pwNew,
    setPwNew,
    pwConfirm,
    setPwConfirm,
    pwSaveState,
    pwSaveMsg,
    pwSaveShake,
    revealPwNew,
    revealPwConfirm,
    revealMetricsToken,
    registryTokenVisible,
    setRegistryTokenVisible,
    registryRowIds,
    setRegistryRowIds,
    shape,
    setShapeLocal,
    motion,
    setMotionLocal,
    stormFound,
    setStormFound,
    stormClicks,
    discoFound,
    disco,
    setDiscoLocal,
    labelModes,
    setLabelModes,
    rainbow,
    updateRainbow,
    rainbowToggled,
    setEncSaveState,
    setEncSaveError,
    kitError,
    setKitError,
    setPathSaveState,
    setPathSaveError,
    setExportEncSaveState,
    setExportEncSaveError,
    setOffsiteSaveState,
    setOffsiteSaveError,
    offsiteWizard,
    setOffsiteWizard,
    domainToggleBusy,
    domainToggleShake,
    setRetSaveState,
    setRetSaveError,
    setPruneSaveState,
    setPruneSaveError,
    setReconcileSaveState,
    setReconcileSaveError,
    setCacheSaveState,
    setCacheSaveError,
    setCoresSaveState,
    setCoresSaveError,
    setLimSaveState,
    setLimSaveError,
    setMetricsSaveState,
    setMetricsSaveError,
    setDigestSaveState,
    setDigestSaveError,
    setWatchdogSaveState,
    setWatchdogSaveError,
    containers,
    vms,
    fileSets,
    syncSchedules,
    schedFieldBusy,
    schedFieldShake,
    syncToggleBusy,
    syncToggleShake,
    configScheduleToggleBusy,
    configScheduleToggleShake,
    loadFileSets,
    fieldPulse,
    save,
    saveOffsiteRetention,
    toggleDomainEnabled,
    mergedFieldBusy,
    mergedFieldShake,
    autoSaveField,
    fieldBusy,
    fieldShake,
    autoSaveToggle,
    debouncedSave,
    cancelDebounce,
    applyImportedSettings,
    saveRegistries,
    scheduleField,
    autoSaveScheduleField,
    scheduleUpdate,
    handleSyncSchedulesToggle,
    toggleConfigSchedule,
    handleSetPassword,
  };

  return (
    // gap-10 (live-review round — "gap between the tab strip and the first
    // card is too small"): was gap-6 (24px), same value the tab-panels
    // wrapper further down used to use for the SAME job before its own
    // gap-10 bump (see that wrapper's own comment). This outer wrapper had
    // exactly two children when that bump landed — the heading+tab-strip
    // block immediately below, and the tab-panels wrapper — so bumping ITS
    // gap to gap-10 is what actually widens the space between the tab strip
    // and the first Card's top edge to the same 40px rhythm every
    // Card-to-Card gap already uses, without touching the (unrelated, still
    // gap-6) space between the heading and the tab strip itself. AboutFooter
    // (sticky-footer round, see its own header comment) is now a third
    // child, after the tab-panels wrapper — the same gap-10 rhythm applies
    // there too, for free, with no extra spacing utility needed on the
    // footer itself.
    //
    // `flex-1` (sticky-footer round): makes this whole page root grow to
    // fill the scrollable viewport's available height (app/Layout.tsx's
    // `main` → its `glim-page-enter` child, both given a matching `flex-1 flex
    // flex-col` for exactly this — see that file's own comments) instead of
    // shrink-wrapping to its own content height. On its own this would just
    // make the ROOT taller with blank space at the bottom (flex columns
    // don't redistribute leftover space to children unless a child asks for
    // it) — the tab-panels wrapper further down carries the matching
    // `flex-1` that actually consumes that space, which is what pushes
    // AboutFooter down to this column's bottom edge. Content taller than the
    // available height still simply grows this element (and `main`'s
    // scrollHeight with it) past that floor, which is what lets `main`
    // scroll normally instead of clipping anything — see the tab-panels
    // wrapper's own comment for why `flex-1` produces exactly that
    // fill-or-grow behaviour with no separate min-height override needed.
    //
    // PAGE_SHELL_TABBED — the ONE stated exception to the app-wide page width
    // (jdp live-review, "Können wir die nicht überall gleich breit machen?").
    // Every other page now renders at PAGE_SHELL's 1152px; this root keeps the
    // shared 40px rhythm but deliberately has NO max-width, and that is not an
    // oversight. Measured live before deciding: capping this root at 1152px
    // caps the 7-tab Selector strip inside it too, and the strip — `size="lg"`
    // + `equalWidth`, so 7x its widest segment, 1424px in de — no longer fits
    // on one line there (strip height 32px → 68px, the 7 tabs falling onto 2
    // rows). That two-row strip is a bug an earlier round already fixed once,
    // and the panels below are capped to this strip's MEASURED width per a
    // standing instruction ("Settings cards should match the tab row's
    // width"), so capping the root would regress both at once.
    //   This is a genuine conflict between two of jdp's own asks rather than
    // something to resolve silently: the honest fix is to make the STRIP
    // narrower (drop `equalWidth`, whose natural hugged width is ~814px in de,
    // or step `size` down from "lg"), after which this page could join the
    // shared cap. That is a change to a deliberate prior decision, so it is
    // flagged for jdp rather than taken here. See lib/pageShell.ts.
    <div className={PAGE_SHELL_TABBED}>
      {confirmDialog}
      {/* Heading + tab strip, grouped in their own gap-6 column (GlimStone
          follow-up pass, live-review round — the width-mismatch fix below
          needed a wrapper here to isolate this pair's own 24px gap from the
          new gap-10 the OUTER wrapper now uses for the tab-strip-to-first-
          card gap; before this pass, heading/strip/panels were three
          siblings sharing one flat gap value). Deliberately NOT inside the
          max-w-3xl reading column the panels wrapper further down used to
          own alone (GlimStone follow-up pass, live-review point 7): every
          OTHER page's own <h1>/<p> (Dashboard.tsx, Containers.tsx, VMs.tsx,
          Files.tsx, Flash.tsx, Config.tsx, Receiver.tsx, Fleet.tsx) renders
          at the page's own full width, un-capped — Settings.tsx was the one
          page that swept its heading into the same narrow column as its
          form content, which that pass undid to match that convention. */}
      <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("settings.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          {t("settings.subtitle")}
        </p>
      </div>

      {/* The tab strip. Each tab owns the rainbow position of its list index.
          It sits outside the max-w-3xl column of the panels, because seven
          segments need about 814px in German and a capped strip wraps a lone
          tab onto a second line. equalWidth pins every segment to the widest
          label, and the panels below take the strip's measured width, so both
          line up. `title` shows a label that equalWidth truncates.

          self-start keeps the wrapper at the strip's own width: as a child of
          a flex column it would stretch to the column, and the measurement
          would read the column instead of the strip. */}
      <div ref={setTabStripEl} className="inline-flex self-start max-w-full">
      <Selector
        items={([
          ["general", t("settings.tab.general")],
          ["storage", t("settings.tab.storage")],
          ["schedules", t("settings.tab.schedules")],
          ["offsite", t("settings.tab.offsite")],
          ["notifications", t("settings.tab.notifications")],
          ["integrity", t("settings.tab.integrity")],
          ["system", t("settings.tab.system")],
        ] as const).map(([key, label]) => ({ id: key, label, icon: TAB_ICON[key], title: label }))}
        label={t("settings.title")}
        // 0, which is also the default - stated anyway, because it is the one
        // start every other selector in the tree has to avoid.
        hueOffset={HUE_OFFSET.tabs}
        select="one"
        active={tab}
        onChange={(key) => {
          // Settings tab slide (GlimStone motion-engine animation 7) —
          // computed HERE, in the same synchronous event handler that also
          // calls setTab() below, because this is the one place that still
          // has BOTH the old tab (the `tab` closure variable, not yet
          // updated) and the new one (`key`) at once. React batches this
          // setTabDir alongside the setTab() call into the same commit, so
          // the tab-content wrapper's very first render with the new `tab`
          // already carries the correct --tab-dir (see that wrapper's own
          // comment further down for why keying it on `tab` is what makes
          // the slide replay on every click).
          const from = TAB_ORDER.indexOf(tab);
          const to = TAB_ORDER.indexOf(key as TabKey);
          if (from !== -1 && to !== -1) setTabDir(to > from ? 1 : -1);
          setTab(key as TabKey);
          // Keep the URL hash in sync so reload/bookmark restores the tab
          // (replaceState avoids polluting history and won't re-fire applyHash).
          try {
            window.history.replaceState(null, "", `#${key}`);
          } catch {
            /* history unavailable — tab state still switches */
          }
        }}
        size="lg"
        equalWidth
        /* #178, [200]: the strip joins the size system, with jdp's stated
           exception that these segments must be equal ALWAYS. groupWidth picks
           the stage the longest tab name needs in the current language and
           gives it to every tab, so the strip is uniform by construction
           rather than by measurement. That also retires the failure this
           file's own header describes: the measured pin once grew to 1424px
           in German and wrapped the seven tabs onto two rows. */
        // The rail's own row width, via the shared token — "gleich groß wie die
        // tabs in der sidebar" is a promise, and a promise needs one number,
        // not two that happen to agree today. It also gives the longest label
        // ("Benachrichtigungen") the 16px it was missing, which is why that tab
        // clipped its own text in reactive mode.
        segmentWidth="var(--nav-row-w)"
      />
      </div>
      </div>

      {/* Tab panels. GlimStone follow-up pass, live-review round ("Settings
          cards should match the tab row's width"): the `max-w-3xl` cap that
          used to live on this wrapper is GONE — removed, not resized to a
          new guessed number.

          UPDATED (equalWidth correction round — see the tab strip's own
          comment block above): back when `equalWidth` stretched the strip to
          fill the full row, this wrapper needed no cap at all — both it and
          the strip were simply full-width by construction, so they matched
          automatically. Now that the strip hugs its own (narrower, content-
          matched) width instead, that "both happen to be full-width"
          assumption no longer holds — a truly uncapped Card would render
          wider than the tabs sitting above it again, the exact mismatch this
          whole feature exists to prevent. `style={{ maxWidth: tabStripWidth
          }}` (below) is the fix: `tabStripWidth` is a REAL measured pixel
          value (this component's own ResizeObserver, set up in the state
          block near the top of SettingsPage), read off the actual rendered
          tab strip rather than a guessed literal — so it tracks correctly
          across every locale's own longest label, a window resize, or a
          zoom level change, none of which a hard-coded number could.
          `?? undefined` for the one frame before the observer's first
          measurement lands (mount): `maxWidth: null` is not valid CSS and
          React would warn, `undefined` simply omits the style property that
          render, matching this wrapper's original uncapped look until the
          real number is known.
            gap-10 (live-review round — "more air between Cards, there's
          plenty of room"): was gap-6 (24px), already the single largest gap
          value used anywhere in this app before this bump (verified — no
          other call site reaches past gap-6). Every direct child of this
          wrapper is either a whole Card (own bg-carbon-surface + p-5 box) or
          an equivalent top-level section, so this one value IS the vertical
          rhythm between Settings' Domains/Language/Theme/Accent/Shape/
          Rainbow/Quiet-toasts blocks — bumping it here, and only here,
          reaches every one of them. 40px (~1.67x the old 24px, inside the
          requested 1.5-2x range) reads as a deliberate step up without the
          Cards feeling disconnected from each other on the page. The outer
          wrapper above reuses this same gap-10 value for the tab-strip-to-
          first-card gap (a separate live-review ask, its own comment) —
          matching this established rhythm rather than inventing a different
          number for that gap too. */}
      {/* key={tab} (GlimStone motion-engine animation 7, Settings tab slide):
          this ONE div wraps every `{tab === "x" && ...}` panel below — every
          Card inside it ALREADY fully unmounts/remounts on a tab switch via
          those conditionals alone, key or no key; keying the WRAPPER too
          changes nothing about which children exist, it only makes the
          wrapper itself a fresh DOM node each click, which is what lets
          `.glim-tab-slide`'s own entrance animation (index.css) replay every
          time instead of only once at Settings' own first mount (a
          persistent class on a node that never gets recreated never
          replays its animation, the same reasoning glim-stagger-row's own
          comment gives for why a list re-render does NOT replay). --tab-dir
          is set from `tabDir` state, computed by whichever caller last
          changed `tab` (the Selector's onChange below, or the hashchange
          effect above) in the SAME synchronous handler that called setTab —
          see either call site's own comment for the exact "old index vs new
          index" math.
            `flex-1` (sticky-footer round, jdp live review — see AboutFooter's
          own header comment for the full before/after): this is the ONE
          child of the page root (above) that should absorb whatever extra
          height that root has beyond its own natural content size — the
          heading+tab-strip block above it is a fixed-content block that
          should never stretch, and AboutFooter below it is the thing being
          pushed down, not the thing doing the pushing. flex-basis 0 + grow 1
          (Tailwind's `flex-1`) means this wrapper fills the ROOT's leftover
          vertical space when its own Cards don't need all of it (short tabs
          like General), while its automatic minimum height still floors at
          whatever its own content actually needs — so on a long tab
          (Storage, Schedules) it simply renders at full content height
          exactly as before, growing `main` past the viewport and letting it
          scroll normally, with AboutFooter still following right after it
          rather than sitting fixed over top of it. */}
      <div
        key={tab}
        className="flex flex-col gap-10 glim-tab-slide flex-1"
        style={{ maxWidth: tabStripWidth ?? undefined, "--tab-dir": tabDir } as CSSProperties}
      >
      {tab === "general" && <GeneralTab {...tabProps} />}
      {tab === "storage" && <StorageTab {...tabProps} />}
      {tab === "schedules" && <SchedulesTab {...tabProps} />}
      {tab === "offsite" && <OffsiteTab {...tabProps} />}
      {tab === "notifications" && <NotificationsTab {...tabProps} />}
      {tab === "integrity" && <IntegrityTab {...tabProps} />}
      {tab === "system" && <SystemTab {...tabProps} />}
      </div>

    </div>
  );
}
