import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { getSettings, putSettings, getAuth, setAuthPassword, logout, logoutAll, getVMSSH, testVMSSH, getRclone, setRclone, getCloud, setCloud, getCloudCredSets, setCloudCredSets, checkDomain, unlockDomain, pruneDomain, replicateOffsite, testOffsite, tamperTest, getStatus, getNotify, setNotify, testNotify, runDrill, getDrills, listContainers, listVMs, setScheduleCadence, setVMScheduleCadence, listFileSets, patchFileSet, downloadRecoveryKit, exportSettings, importSettingsPreview, importSettingsApply, getHealth, generateWidgetToken, disableWidgetToken, generateFleetToken, disableFleetToken, getDashboardPlugin, installDashboardPlugin, removeDashboardPlugin } from "../lib/api";
import type { CloudCredSet, CloudCredSetInfo } from "../lib/api";
import { SourceToggle, isOffsiteSource, type RepoSource } from "../components/SourceToggle";
import { useOffsiteTargets } from "../lib/useOffsiteTargets";
import { FolderBrowser } from "../components/FolderBrowser";
import { OffsiteWizard } from "../components/OffsiteWizard";
import { PathModeSwitch } from "../components/PathModeSwitch";
import { InfoBubble } from "../components/InfoBubble";
import { IconTipButton } from "../components/IconTipButton";
import { OffsiteTargetsSection } from "../components/OffsiteTargetsSection";
import { CadenceBuilder } from "../components/CadenceBuilder";
import { ItemScheduleOverride } from "../components/ItemScheduleOverride";
import { Toggle } from "../components/Toggle";
import { Badge, type BadgeTone } from "../components/Badge";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { useConfirm } from "../lib/useConfirm";
import type { Settings, NotifyConfig, RestoreDrill, Container, VM, FileSetView, RegistryAuthEntry, ImportSettingsSummary } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { copyText } from "../lib/clipboard";
import { useToast } from "../lib/toast";
import { withLtrFragments, REPO_LOCAL_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { randomId } from "../lib/uuid";
import { useAdvanced } from "../lib/advanced";
import { SpikePanel } from "../components/SpikePanel";
import { ColorPickerSwatch } from "../components/ColorPickerPopover";
import {
  getAccent,
  setAccent,
  DEFAULT_ACCENT,
  getAccentPresets,
  setAccentPresets,
  DEFAULT_ACCENT_PRESETS,
} from "../lib/accent";
import { RAINBOW, getRainbow, setRainbow, hueVars, rainbowAt, type RainbowState } from "../lib/appearance";
import { SHAPES, getShape, setShape, type Shape } from "../lib/shape";
import { Selector } from "../components/Selector";
import { relativeTime } from "../lib/reltime";
import { Flag, IconAdd, IconDownload, IconTrash } from "../components/Sidebar";
import { getResolvedTheme, getTheme, onSystemThemeChange, setTheme, type ResolvedTheme } from "../lib/theme";

// AboutFooter shows the running version (linking to the releases page) and a
// "Report a bug" link at the very bottom of Settings, so the sidebar stays clean.
function AboutFooter() {
  const { t } = useT();
  const [version, setVersion] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    getHealth()
      .then((h) => { if (active) setVersion(h.version ?? null); })
      .catch(() => { /* version is best-effort; ignore */ });
    return () => { active = false; };
  }, []);
  return (
    // Task 5 (rule 13, "everything clickable is a badge — including links"):
    // both footer links were plain underline-on-hover text. `as="a"` (not
    // "button") keeps real anchor semantics — right-click "copy link",
    // middle-click to open in a new tab, the browser's own status-bar URL
    // preview — none of which a synthetic onClick reproduces.
    <div className="pt-6 pb-4 flex flex-col items-center gap-1.5 text-xs text-carbon-textMuted">
      {version && (
        <Badge
          as="a"
          href="https://github.com/junkerderprovinz/bombvault/releases"
          target="_blank"
          rel="noopener noreferrer"
          tone="neutral"
          size="small"
          title={`BombVault ${version}`}
        >
          BombVault {version}
        </Badge>
      )}
      <Badge
        as="a"
        href="https://github.com/junkerderprovinz/bombvault/issues"
        target="_blank"
        rel="noopener noreferrer"
        tone="neutral"
        size="small"
      >
        {t("nav.reportBug")}
      </Badge>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Card wrapper
// ---------------------------------------------------------------------------

function Card({
  title,
  hint,
  children,
  hueIndex,
}: {
  title: string;
  /** Optional one-line explanation of what this whole Card does, rendered as
   *  a neutral (i) beside the title (design-language.md rule 8, "explanations
   *  live in a bubble, not on the page") instead of a permanent grey <p>
   *  under it — GlimStone form-engine Phase 2 Task 4's hint→bubble content
   *  migration. Optional, but not byte-for-byte additive: the <h2> below
   *  changed className for EVERY Card, hint or not (block → `flex items-
   *  center gap-1.5`, so a bubble can sit inline next to the title when one
   *  is passed). That's visually inert for the 35+ Cards that don't pass a
   *  hint — a flex row with a single text child lays out identically to a
   *  block element, confirmed with no pixel difference across several
   *  viewport widths — but it is a real className change, not a no-op. */
  hint?: string;
  children: React.ReactNode;
  /** Rainbow position for THIS Card's own heading notch, by its position
   *  among the Cards visible on the CURRENTLY ACTIVE tab (jdp's live-review
   *  override — see Badge.tsx's own tone="heading" section for the full
   *  history). Threaded straight through to Badge's own `hueIndex`; see that
   *  prop's doc for the "only meaningful on tone='heading'" / "omit for a
   *  tab's only Card" rules, both of which apply identically here. Assigned
   *  at every call site via the single `nextHue()` counter declared at the
   *  top of this component's own return — see that counter's comment for
   *  why a shared counter, not per-tab hand-counted literals, is what keeps
   *  ~50 Card call sites across seven tabs correctly numbered. */
  hueIndex?: number;
}) {
  return (
    // GlimStone follow-up pass (live-review round, "half-overlap card
    // notch"): `relative` added here so the heading Badge — now
    // `position: absolute`, straddling THIS card's own top edge, see
    // Badge.tsx's badgeClassName comment — resolves its offset against
    // this actual card, not some larger ancestor. This also retires the
    // old "InfoBubble sits as a sibling outside the badge" placement noted
    // below: a flex sibling can no longer ride beside the badge's text once
    // the badge leaves normal flow (it would drop to the badge's old flow
    // position instead — alone, at the top of the card, no longer next to
    // any visible title text). The InfoBubble moves INSIDE the Badge's own
    // children instead, riding along as one floating unit.
    //
    // `onAccent` (live-review follow-up: the badge went from an accent-soft
    // wash to a full solid `bg-accent` fill — see Badge.tsx's tone="heading"
    // section — and InfoBubble's OLD fixed-neutral icon colour, fine on that
    // soft wash, turned out hard to see against a bright/light accent hue).
    // Passed here specifically because this is the one InfoBubble call site
    // that actually sits INSIDE a tone="heading" Badge; every other hint in
    // this file (ToggleRow's caption, a plain Card-body hint) sits on the
    // ordinary card surface and correctly omits it, keeping the original
    // neutral icon. See InfoBubble.tsx's own header for what the prop does.
    // InfoBubble's tooltip itself is unaffected either way — it's portal-
    // rendered off the icon's live getBoundingClientRect, not a descendant of
    // the coloured badge, so the icon's new ancestor being position:absolute
    // (and, now, its own colour inheriting from that ancestor) never reaches it.
    // `glim-notch-card` (jdp, live-review: "the heading badge only lights up
    // in reactive mode when hovering the badge itself, not the whole card")
    // — a plain marker class (no styling of its own) that index.css's
    // `[data-rainbow="reactive"] .glim-notch-card:hover .glim-notch-hue`
    // rule keys off, so this Card's own hueIndex'd heading notch reveals its
    // colour on hover/focus anywhere in this card, not just its own ~22px
    // glyph. See that rule's own comment in index.css for why it's a
    // dedicated class rather than the general `rounded-card` utility.
    <div className="relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
      {/* Task 5 (design-language.md rule 11, "every heading is a filled
          section badge") resolution, for whoever finds this next: the <h2>
          tag stays (screen readers still get a real heading, e.g. "heading
          level 2: Off-site Copy"), but its VISIBLE content is now a Badge
          (tone="heading" size="heading" — see Badge.tsx's file header for
          the full colour/size reasoning). */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {title}
          {hint && <InfoBubble tip={hint} onAccent />}
        </Badge>
      </h2>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Toggle row
// ---------------------------------------------------------------------------

export function ToggleRow({
  label,
  description,
  hint,
  checked,
  onChange,
  disabled,
  hideLabel = false,
  shakeNonce,
  hueIndex,
}: {
  label: string;
  description?: string;
  /** Optional (i) bubble beside the visible label — same content contract as
   *  Card's own `hint` (design-language.md rule 8, "explanations live in a
   *  bubble, not on the page"). Added for the GlimStone follow-up pass's
   *  rainbow-section rework: "Reactive mode"/"Colour rotation" need a node
   *  (icon + text) next to the label, which a plain string `label` can't
   *  carry — see Card's own header comment for the identical constraint on
   *  its `title`/`hint` pair. Renders only when `!hideLabel` (nothing to sit
   *  beside once the label itself is hidden). */
  hint?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  /** Suppress the row's own visible caption when a Card title directly above
   *  already says the same thing (e.g. a single-purpose Card whose title IS
   *  the decision this switch makes) — the label still reaches screen readers
   *  via the underlying Toggle's aria-label. Any `description` still renders. */
  hideLabel?: boolean;
  /** Bump this (any new, truthy number) to replay the `.glim-shake` error
   *  feedback animation once — e.g. an auto-save rejected by the backend
   *  (design-language.md's motion engine, "shake" pattern). Passed straight
   *  through as the underlying Toggle's React `key`: a genuinely NEW value
   *  unmounts and remounts that one <button>, so the CSS animation restarts
   *  from its first frame even when the previous shake never fully settled
   *  and even for the SAME domain failing twice in a row — a class toggled
   *  on an element that never leaves the DOM would need an `animationend`
   *  handler (or a forced-reflow class-remove-then-readd trick) to replay on
   *  a repeated identical failure; this codebase's existing precedent for
   *  "replay on a repeated identical trigger" is lib/toast.tsx's push(),
   *  which mints a fresh `id` per toast so an identical message still gets a
   *  fresh DOM node and its entrance animation always plays. Undefined/0
   *  (never shaken yet) renders no `.glim-shake` class at all, so a normal
   *  page load or a successful toggle never shakes. */
  shakeNonce?: number;
  /** Rainbow position for THIS row's own switch, by LIST INDEX among the
   *  ToggleRows rendered together in one place (jdp, live-review: "Die
   *  ganzen Toggle... sind nicht in der Farbengine!!") — the Domains list
   *  (settings.domains Card: Container/VMs/Flash/Ordner/Selbst-Backup/
   *  Empfänger-Dashboard/Fleet-Ansicht) was the first case fixed this way,
   *  design-language.md's own "walk every genuine equal-member set" rule.
   *  Own local 0-based index PER GROUP (unrelated to Card's/Badge's own
   *  `nextHue()` sequence) — the merged Colors Card's own three rainbow
   *  toggles (master/Reactive/Rotate) are a second, independent group with
   *  their own 0/1/2, not a continuation of the Domains list's 0-6.
   *    CORRECTED (jdp, live-review, extremely emphatic standing rule — "auch
   *  nicht die Toggles der Regenbogen-Card! ... Es soll immer alles in die
   *  Farb- und Formengine integriert werden!! IMMER!!"): this doc used to
   *  claim the Rainbow Card's own three toggles correctly stayed flat,
   *  reasoned as "not members of an equal, trackable list the way seven
   *  independent domain toggles are." That reasoning was exactly the kind
   *  of self-authored design exception jdp has now ruled out categorically
   *  — three toggles rendered together in the same Card, one per row, are a
   *  list by construction regardless of whether they're logically
   *  independent switches or a master-plus-sub-options group. They now
   *  carry hueIndex 0/1/2 (see the merged Colors Card's own comment in
   *  SettingsPage). Omit only for a genuinely LONE ToggleRow with no
   *  siblings of its own kind on screen (e.g. "Leise Benachrichtigungen",
   *  the sole toggle in its own single-purpose Card) — that case has no
   *  list to walk at all, not a list this doc is choosing to exclude. */
  hueIndex?: number;
}) {
  // Deliberately NO useRainbow() subscription — ToggleRow (like Toggle and
  // Badge) is a pure, hookless function component by established convention
  // (Settings.toggleRow.test.ts's own header: "invoked directly as a plain
  // function... no jsdom/testing-library needed"). A first attempt at this
  // added the hook and broke that entire suite with "Cannot read properties
  // of null (reading 'useSyncExternalStore')" — calling a component as a
  // plain function outside React's reconciler leaves hooks with no
  // dispatcher. hueVars()/rainbowAt() below are plain functions reading
  // module state, not hooks, so they still resolve correctly at render time.
  // Callers of `hueIndex` today (the Domains Card's seven rows, and the
  // merged Colors Card's own three rainbow toggles) live inside
  // Settings.tsx's own SettingsPage(), the SAME component whose `rainbow`
  // plain useState() (`rainbow`/`setRainbowLocal`) backs the mode itself —
  // flipping it already re-renders this entire function, recomputing every
  // ToggleRow's hueVars right along with it. See Badge.tsx's own identical
  // comment for the fuller version of this reasoning.
  // The switch dims itself via its own `disabled:opacity-50` (Toggle.tsx), but
  // that left the caption and description next to it at full opacity, so a
  // disabled row misleadingly still read as enabled. Rule 15 rules out opacity
  // on the CONTAINER (it composites the whole subtree), so each text node
  // carries its own — the same per-element dimming the controls use.
  //
  // Deliberately a plain <div>, NOT the <fieldset disabled> + `group-disabled:`
  // mechanism CadenceBuilder uses: that fieldset earns its keep (it really does
  // group many controls, natively disables all of them without threading a prop
  // into CronEditor, and names itself with a <legend>). A row holds exactly ONE
  // control, which already receives `disabled` directly — wrapping it in a
  // fieldset would only add an unnamed `group` to the accessibility tree at
  // every call site, the very defect the <legend> was added to fix.
  const dim = disabled ? " opacity-50" : "";
  const hueOn = hueIndex !== undefined;
  return (
    <div
      className={`flex items-start justify-between gap-4${hueOn ? " glim-hue" : ""}`}
      style={hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      <div className="flex flex-col gap-0.5">
        {!hideLabel && (
          <span className={`flex items-center gap-1.5 text-sm text-carbon-text${dim}`}>
            {label}
            {hint && <InfoBubble tip={hint} />}
          </span>
        )}
        {description && (
          <span className={`text-xs text-carbon-textMuted${dim}`}>{description}</span>
        )}
      </div>
      <Toggle
        key={shakeNonce}
        hideLabel
        label={label}
        checked={checked}
        onChange={onChange}
        disabled={disabled}
        className={`mt-0.5${shakeNonce ? " glim-shake" : ""}`}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Save bar shared component
//
// GlimStone form-engine Task 9 (lib/toast.tsx) formalized this exact
// pattern — a save button whose "saved"/"error" outcome flashes inline for
// a few seconds — into a real toast at TWO self-contained sites that don't
// route through this shared component (ConfigSettingsCard in Config.tsx,
// and SettingsPage's own handleSetPassword), as a deliberate proof of
// adoption, not a full migration. This SaveBar component itself, and the
// ~30 call sites across this file that render it (all sharing the single
// generic `save()` helper further down), were DELIBERATELY left on the
// original "saved"/"error" inline-flash behaviour — migrating a helper
// this widely shared would silently convert every one of those call sites
// in one pass, which was explicitly out of Task 9's scope.
//
// GlimStone follow-up pass (v8.0.0): that follow-up work. `save()` (below)
// now pushes a toast on both outcomes and resets straight back to "idle" —
// same shape as handleSetPassword's own migration — so the "saved"/"error"
// states this component used to render are never produced by any caller
// anymore. The two branches that rendered them are gone; `error` stays in
// the prop signature (still passed by all ~30 call sites, always null now)
// rather than forcing a signature change across every one of them for a
// prop that would otherwise go unused — `_error` names that deliberately.
// ---------------------------------------------------------------------------

type SaveState = "idle" | "saving" | "saved" | "error";

// `hueIndex` (GlimStone follow-up round, Paths & Storage tab rework, point 3
// — jdp, live review: "die Speichern-Buttons sind nicht in der Farbengine,
// im Regenbogenmodus haben alle die gleiche Farbe"). Every one of this file's
// ~25 remaining SaveBar call sites rendered the identical flat `bg-accent`
// fill even with rainbow mode on — the exact same bug Badge's tone="heading"
// notches and ToggleRow's Domains rows already had fixed for themselves.
// Reuses the SAME mechanism (`.glim-hue` + `hueVars(rainbowAt(i))`), and the
// SAME index its own Card's title notch already got from `nextHue()` —
// passed straight through by each call site (`hueIndex={cardHue}` alongside
// `<Card hueIndex={cardHue}>`) rather than a second, independent `nextHue()`
// call. A card and its own action button read as one coloured group this
// way, instead of two unrelated rainbow positions competing for attention
// inside the same card. `export`ed for the same reason ToggleRow already is
// (Settings.toggleRow.test.ts) — a plain function component, testable
// directly with no jsdom needed.
export function SaveBar({
  state,
  onSave,
  t,
  disabled = false,
  hueIndex,
}: {
  state: SaveState;
  /** Always null post-migration — see this component's header comment. */
  error?: string | null;
  onSave: () => void;
  t: ReturnType<typeof useT>["t"];
  disabled?: boolean;
  hueIndex?: number;
}) {
  const hueOn = hueIndex !== undefined;
  return (
    <div className="flex items-center gap-3 pt-1">
      <button
        onClick={onSave}
        disabled={disabled || state === "saving"}
        style={hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
        className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${hueOn ? " glim-hue" : ""}`}
      >
        {state === "saving" ? (
          <>
            <span
              className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("common.saving")}
          </>
        ) : (
          t("settings.save")
        )}
      </button>
    </div>
  );
}

// Accent preset swatches: DEFAULT_ACCENT_PRESETS (lib/accent.ts) now owns
// both the hex values AND the persistence/reset shape — see that module's
// own header comment. AccentCard/AccentPresetSwatch further down are the UI
// half.

// ---------------------------------------------------------------------------
// Palette swatch — one editable colour in the rainbow palette editor
// (GlimStone form-engine Phase 2, Task 1). Deliberately matches the existing
// accent-preset swatches' own visual language above (a rounded-pill circle
// showing the colour, a border) rather than introducing a new component
// family: ColorPickerSwatch (the shared GlimStone-picker trigger — see its
// own header comment) opens the same floating popover the Accent Card's
// custom swatch below uses, pre-synced to this position's own value, instead
// of a native <input type="color"> (a genuinely separate browser/OS window —
// jdp: "kein eigenes Fenster welches sich öffnet"). `disabled` dims the
// control on its OWN element (native `disabled` + `disabled:opacity-50`),
// never via a wrapping container's opacity (rule 15 / this branch's own
// established "dimmed via disabled, not opacity-on-container" fix from Phase
// 1 Task 4) — ColorPickerSwatch's own `disabled` prop follows that same
// contract.
//
// `rounded-pill`, not a hardcoded `rounded-full` (GlimStone follow-up pass,
// live-review point 4): a literal `rounded-full` is a fixed 50% radius that
// never moves, so this swatch (and the accent-preset swatches below) used to
// stay a perfect circle no matter what shape the user picked in Settings —
// the one pair of controls on this page that silently ignored the shape
// engine. `rounded-pill` is the SAME token every pill-shaped Badge already
// reads (Badge.tsx's shape="pill"/"circle" -> RADIUS_CLASSES.pill/circle),
// and both swatches here are already fixed equal-width/-height boxes, so no
// `aspect-square` is needed the way Badge's circle shape needs one — points
// at var(--radius-pill), which index.css already varies per data-shape:
// 9999px (round, a true circle), 0.3125rem (soft, a lightly rounded square),
// 0 (square, a hard corner) — reactive with zero new CSS.
// ---------------------------------------------------------------------------
function PaletteSwatch({
  hex,
  index,
  disabled,
  onChange,
  t,
}: {
  hex: string;
  index: number;
  disabled?: boolean;
  onChange: (hex: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const label = `${t("settings.rainbowPalette")} ${index + 1}`;
  return (
    <ColorPickerSwatch
      value={hex}
      onChange={onChange}
      label={label}
      disabled={disabled}
      className="h-7 w-7 shrink-0 rounded-pill border-2 border-carbon-border transition-transform hover:scale-110 disabled:cursor-not-allowed disabled:opacity-50"
    />
  );
}

// ---------------------------------------------------------------------------
// AccentPresetSwatch — one preset in the Accent Card's own preset row
// (GlimStone follow-up pass, live-review round 6: jdp asked for the presets
// to become individually editable, get a reset, and for there to be more of
// them). Wraps the SAME ColorPickerSwatch trigger PaletteSwatch above
// already uses for the rainbow palette's 8 editable swatches — clicking it
// opens the shared popover, pre-synced to THIS preset's own stored value,
// exactly like a palette swatch does for its own position.
//
// Interaction (chosen over "click only selects, editing needs a separate
// trigger"): a click both SELECTS this preset as the live accent AND opens
// its editor, and further edits (drag/type) inside the open popover keep
// updating the live accent too — selecting and editing are the same
// gesture, matching how PaletteSwatch/ColorPickerSwatch already behave
// (there is no separate "select" step for a palette position either; opening
// IS the only interaction). Concretely: ColorPickerSwatch's own trigger
// button owns the click (it opens the popover); a plain wrapping
// `<span onClick>` catches the SAME click event on its way up through the
// DOM's ordinary bubble phase and fires the selection — both handlers run
// inside the one click (and the one synthetic click a screen reader /
// keyboard Enter-or-Space activation of the inner <button> produces), so
// there is no visible order or flicker between "opened" and "selected".
//
// The active-preset ring used to be a border colour swap on the swatch
// button itself (the plain-<button> version before this round). It moves to
// this wrapping span instead — ColorPickerSwatch owns its own border via its
// fixed `className` prop, with no per-call override for it — so the inner
// swatch renders borderless and the wrapper supplies the SAME 2px border,
// scaling and hover-transform together as one visual unit.
// ---------------------------------------------------------------------------
function AccentPresetSwatch({
  hex,
  index,
  active,
  onSelect,
  onChangePreset,
  t,
}: {
  hex: string;
  index: number;
  /** Whether this preset's OWN stored colour currently matches the live
   *  accent — drives the highlighted ring, same "is this the active one"
   *  signal the old plain-button version's border colour swap gave. */
  active: boolean;
  /** Fires on click (select this preset as the live accent) AND on every
   *  live edit inside the popover — see this component's own header comment. */
  onSelect: (hex: string) => void;
  /** Fires on every live edit inside the popover — persists the edited
   *  value back into THIS preset's own slot in the stored array. */
  onChangePreset: (hex: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const label = `${t("settings.accentPreset")} ${index + 1}`;
  return (
    <span
      onClick={() => onSelect(hex)}
      className="inline-flex rounded-pill border-2 transition-transform hover:scale-110"
      style={{ borderColor: active ? "var(--carbon-text)" : "var(--carbon-border)" }}
    >
      <ColorPickerSwatch
        value={hex}
        onChange={(v) => {
          onChangePreset(v);
          onSelect(v);
        }}
        label={label}
        className="w-6 h-6 rounded-pill"
      />
    </span>
  );
}

// ---------------------------------------------------------------------------
// Accent Card (GlimStone follow-up pass, live-review round 6 — jdp, in
// German: "Die Voreinstellungsfelder der Akzentfarbe sollen auch bearbeitbar
// sein und auch ein Reset-Badge bekommen. Bitte mehr
// Voreinstellungsfarbfelder"). Pulled out into its own exported component
// (previously inlined directly in SettingsPage's JSX), the same way
// LanguageCard below already was: its state (accentHex + presets) was never
// read anywhere else in SettingsPage — `accentHex` had exactly one
// declaration and one consuming Card before this round — so the split is
// behaviour-preserving, not a refactor of anything shared.
//
// Presets are now individually EDITABLE (previously five fixed buttons that
// only ever selected, never changed, their own colour) — see
// AccentPresetSwatch's own header comment for the click-both-selects-and-
// opens interaction, and lib/accent.ts's own header comment for the
// persistence shape and the 5→8 preset-count/colour reasoning.
//
// Reset is ROW-LEVEL, not per-preset: the rainbow palette right below this
// Card in the same "general" tab is the closest existing precedent for "a
// fixed-size set of individually editable swatches, resettable," and it
// ships exactly ONE reset for its whole row of 8
// (`updateRainbow({ palette: RAINBOW })`), not eight individual ones —
// matched here for the same granularity, the same Badge shape="circle" icon,
// the same SVG glyph. Unlike the rainbow reset (always shown while its
// master switch is on), this one only renders once at least one preset has
// actually drifted from ITS OWN shipped default — matching the pre-existing
// "Reset to default" text button a few pixels to its right in THIS SAME
// Card (which already only shows once accentHex has drifted from
// DEFAULT_ACCENT), rather than the separate Rainbow Card's own
// always-visible convention: two reset affordances sharing one Card read
// more coherently following EACH OTHER's rule than each independently
// matching a different sibling Card.
//
// No own `<Card>` wrapper any more (jdp, live-review: "Die card von
// Akzentfarbe und Regenbogenmodus in eine mergen. Gehört ja zusammen") —
// this now returns just its own body content, composed inside the shared
// "settings.colors" Card alongside the Rainbow controls at that Card's own
// call site in SettingsPage. `hueIndex` is gone from the signature too: it
// used to be threaded straight through to this component's own Card, which
// no longer exists here — the merged Card's single heading notch owns that
// position now. Settings.accentCard.dom.test.tsx renders this standalone
// (`<AccentCard t={t} />`, no wrapping Card) and only ever asserted against
// the preset buttons/dialogs inside, never the old Card chrome, so it keeps
// passing unchanged.
// ---------------------------------------------------------------------------
export function AccentCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [accentHex, setAccentHex] = useState<string>(() => getAccent());
  const [presets, setPresets] = useState<string[]>(() => getAccentPresets());

  function selectAccent(hex: string) {
    setAccentHex(hex);
    setAccent(hex);
  }

  function changePreset(index: number, hex: string) {
    setPresets((prev) => {
      const next = prev.slice();
      next[index] = hex;
      return setAccentPresets(next);
    });
  }

  const presetsAreDefault = presets.every(
    (hex, i) => hex.toLowerCase() === DEFAULT_ACCENT_PRESETS[i]?.toLowerCase()
  );

  return (
    <div className="flex items-center gap-3 flex-wrap">
      {/* Custom-colour trigger — a flat swatch, same size/shape as the
          preset swatches beside it (design-language.md, "The user-owned
          axes" > Accent: every custom colour value gets the SAME
          trigger). Opens the shared GlimStone picker popover instead of
          a native <input type="color"> — see ColorPickerPopover.tsx's
          own header comment for why (jdp: "kein eigenes Fenster welches
          sich öffnet" — no separate window opening). */}
      <ColorPickerSwatch
        value={accentHex}
        onChange={selectAccent}
        label={t("settings.accentColor")}
        className="w-6 h-6 rounded-pill border-2 border-carbon-border transition-transform hover:scale-110"
      />
      {/* Preset swatches */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.accentPresets")}:</span>
        {presets.map((hex, i) => (
          <AccentPresetSwatch
            key={i}
            hex={hex}
            index={i}
            active={accentHex.toLowerCase() === hex.toLowerCase()}
            onSelect={selectAccent}
            onChangePreset={(v) => changePreset(i, v)}
            t={t}
          />
        ))}
        {/* Reset the PRESET SWATCHES back to their shipped defaults — a
            separate concern from the "reset the active accent" button
            below (that one resets accentHex to DEFAULT_ACCENT; this one
            resets the presets array to DEFAULT_ACCENT_PRESETS). Row-level,
            not per-preset — see this component's own header comment. */}
        {!presetsAreDefault && (
          <Badge
            as="button"
            shape="circle"
            size="icon"
            tone="neutral"
            onClick={() => setPresets(setAccentPresets(DEFAULT_ACCENT_PRESETS))}
            title={t("settings.accentPresetsReset")}
            ariaLabel={t("settings.accentPresetsReset")}
          >
            <svg viewBox="0 0 16 16" width="16" height="16" fill="none" aria-hidden="true">
              <path
                d="M0.67 2.67 L0.67 6.67 L4.67 6.67 M2.34 10 a6 6 0 1 0 1.42 -6.24 L0.67 6.67"
                stroke="currentColor"
                strokeWidth="1.3"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </Badge>
        )}
        {/* Reset to default — the ACTIVE accent, not the presets above. */}
        {accentHex.toLowerCase() !== DEFAULT_ACCENT.toLowerCase() && (
          <button
            onClick={() => selectAccent(DEFAULT_ACCENT)}
            className="text-xs text-carbon-textMuted hover:text-carbon-text transition-colors ms-1"
          >
            {t("common.reset")}
          </button>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Language Card (GlimStone follow-up pass, live-review point 9) — the app's
// UI-language switcher, MOVED here out of Sidebar.tsx's own footer, not
// duplicated (jdp: "verschieb den Sprachschalter... auch als eigene card ins
// allgemein setting"). Same picker mechanism as before: useT()'s
// lang/setLanguage/languages (lib/i18n.ts — a flat 26-locale list, persisted
// to localStorage's "bv-lang" key, applied to <html lang>/[dir] immediately,
// no Save step) and Sidebar.tsx's own exported `Flag` glyph for each entry.
// Only the TRIGGER's styling changed, from the sidebar's nav-rail look
// (navBase/navInactive, which key off `--sidebar-text`/`--sidebar-hover` and
// mean nothing outside the rail) to a plain bg-carbon-surface2 button — the
// same idle-chip fill every other inline picker trigger in this file already
// uses (e.g. VMSSHCard's copy buttons above). The dropdown listbox itself
// (role="listbox", flag+label options, outside-click/Escape-to-close) is
// reused verbatim; only the open direction flipped from `bottom-full` (the
// sidebar footer sits at the viewport's bottom edge) to `top-full` (this
// Card sits in normal page flow, so it opens downward like any other
// dropdown on this page).
export function LanguageCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { lang, setLanguage, languages } = useT();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = languages.find((l) => l.code === lang) ?? languages[0];

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    function handler(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open]);

  return (
    <Card title={t("settings.language")} hueIndex={hueIndex}>
      <div className="relative inline-block" ref={ref}>
        {/* w-48 (GlimStone follow-up pass, live-review round — "widen the
            Language button, then match the Theme button to it"): was
            content-hugging (only as wide as the current flag+label pair),
            which read as too narrow/incidental for a deliberate settings
            control. w-48 (192px) isn't an arbitrary new number — it's the
            SAME width this button's own dropdown listbox already uses
            (`w-48` a few lines below), so the trigger now sits flush above
            the exact footprint of the menu it opens, rather than a narrower
            button popping open a visibly wider list. `truncate`/`min-w-0` on
            the label span below keeps a genuinely long locale name (this
            list has 26) from overflowing the now-fixed width instead of
            just growing the button the way it used to. */}
        <button
          type="button"
          aria-label={`${t("language.label")}: ${current.label}`}
          title={`${t("language.label")}: ${current.label}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
          className="flex items-center gap-2.5 w-48 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
        >
          <Flag code={current.flag} />
          <span className="min-w-0 truncate text-start">{current.label}</span>
        </button>
        {open && (
          <div
            role="listbox"
            aria-label={t("language.label")}
            className="absolute start-0 top-full mt-1 z-50 w-48 max-h-60 overflow-y-auto rounded-card bg-carbon-surface shadow-xl"
            style={{ scrollbarColor: "var(--carbon-border) transparent" }}
          >
            {languages.map((l) => (
              <button
                key={l.code}
                type="button"
                role="option"
                aria-selected={l.code === lang}
                onClick={() => { setLanguage(l.code); setOpen(false); }}
                className={`flex items-center gap-2.5 w-full px-3 py-2 text-sm text-start transition-colors ${
                  l.code === lang
                    ? "bg-carbon-surface3 text-carbon-text"
                    : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                }`}
              >
                <Flag code={l.flag} />
                <span>{l.label}</span>
              </button>
            ))}
          </div>
        )}
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Theme Card (GlimStone follow-up pass, live-review round — jdp asked for the
// dark/light toggle to move out of Sidebar.tsx's footer into its own Card in
// Settings' General tab, the exact same "move, don't duplicate" request
// already applied to the language picker above — see LanguageCard's own
// header comment for that precedent, and Sidebar.tsx's SidebarControls for
// where this toggle used to render). Reuses lib/theme.ts's own
// getResolvedTheme()/getTheme()/setTheme()/onSystemThemeChange() — the exact
// same state machine the sidebar button used, only the surrounding chrome
// changed.
//
// A horizontal Selector, not the old single toggle button (jdp, live-review:
// "das design dunkel/hell bitte ein horizontaler selektor machen") — this IS
// "one of a small, mutually exclusive set of options," design-language.md's
// own "The one horizontal selector" case, the exact shape the Shape Card's
// own round/soft/square picker right below it already uses. Matched to that
// picker's OWN exact established treatment (jdp approved that look live for
// the same kind of control): `size="lg"` (the page's own full Settings-
// decision register, not a tight toolbar chip) and `variant="well"`
// (TrickWork's shared padded track, flush crossfade-only segments — see
// Selector.tsx's file header item 5). No `hue={false}` — see that Card's own
// comment for why an opt-out here would be exactly the self-authored
// aesthetic exception jdp has ruled out; this Selector uses the plain `true`
// default like every other hue-enabled one in the app, so its active segment
// reads its own rainbow position in Rainbow Mode instead of one flat colour
// (positions 0/1 — RAINBOW[0]/[1] — light/dark in that order below).
//
// Two segments only (light/dark), not three (adding "system"): lib/theme.ts's
// own toggleTheme() header already flags "no UI path back to system" as a
// KNOWN GAP, explicitly deferred as its own separate piece of UI design work,
// not a mechanical follow-on — jdp's own ask here named "dunkel/hell"
// specifically, not a third state, so this stays scoped to what was asked
// rather than silently expanding it. onChange sets the theme DIRECTLY to the
// clicked segment's id (never toggleTheme()'s old flip-the-current-value
// logic, which only made sense for a single two-state button) — clicking the
// segment that's already active is a harmless no-op, same as clicking the
// already-active Shape segment.
export function ThemeCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  // getResolvedTheme(), not the raw stored preference: the default is
  // "system" (GlimStone form-engine #1), and this picker only ever shows/
  // sets an explicit dark or light state, so it must reflect what's actually
  // painted rather than the unresolved "system" value.
  const [theme, setThemeState] = useState(getResolvedTheme);

  function selectTheme(next: ResolvedTheme) {
    setTheme(next);
    setThemeState(next);
  }

  // Keep the displayed selection live-accurate on "system": lib/theme.ts's own
  // matchMedia listener repaints data-theme (and every colour token)
  // immediately on an OS-level flip, but this component's `theme` state was
  // only ever set at mount and on an explicit pick — without this, the
  // Card's active segment goes stale the moment the OS changes out from
  // under a "system" user, even though the rest of the UI has already
  // repainted correctly.
  useEffect(() => {
    return onSystemThemeChange(() => {
      if (getTheme() === "system") setThemeState(getResolvedTheme());
    });
  }, []);

  // Filled + coloured glyphs (live-review point 4: "the sun/moon icon is a
  // thin outline, jdp wants it filled and preferably coloured"). Fixed hex
  // fills rather than `currentColor`/an accent token on purpose — a sun and
  // a moon each already carry their OWN conventional colour (warm gold, cool
  // indigo) independent of whatever accent hue or rainbow position the rest
  // of the app is using; tying either to --accent would make the sun render
  // blue if the user's accent happened to be blue, which reads wrong
  // regardless of theme. Both values are solid, mode-independent literals —
  // no [data-theme] variance needed, they read fine on a "well" segment's
  // idle/hover/active fills in both palettes.
  const sunIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      <circle cx="10" cy="10" r="4.25" fill="#FACC15" />
      <path
        d="M10 2v2M10 16v2M2 10h2M16 10h2M4.93 4.93l1.41 1.41M13.66 13.66l1.41 1.41M4.93 15.07l1.41-1.41M13.66 6.34l1.41-1.41"
        stroke="#FACC15" strokeWidth="1.75" strokeLinecap="round"
      />
    </svg>
  );
  const moonIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      {/* Solid fill, no stroke — the crescent path already closes with "z",
          so a plain fill renders the correct silhouette. */}
      <path
        d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z"
        fill="#818CF8"
      />
    </svg>
  );

  return (
    <Card title={t("settings.theme")} hueIndex={hueIndex}>
      {/* `inline-flex self-start max-w-full` (jdp, live-review: "Die
          horizontalen Selektoren sollen nicht auf die ganze Card-Breite
          gestreckt werden, sondern eine standardisierte Breite bekommen") —
          this Card's own root is `flex flex-col` (Card's own comment above),
          whose default `align-items: stretch` blockifies every direct child
          to the Card's own full content width regardless of what display
          value the child itself specifies (a `well` Selector's row is
          `display:flex`, which is ALREADY a block-level box in normal flow —
          nothing about `variant="well"` opts out of that on its own). Same
          exact wrapper, same reasoning, as the Settings tab strip's own
          `tabStripEl` wrapper further down this file (see that div's own,
          much longer comment for the live-measured proof of why
          `self-start` — not just `inline-flex` alone — is the part that
          actually does the work): this is the ONE standardized "don't
          stretch" mechanism for a horizontal Selector in this app now,
          reused verbatim rather than inventing a second one for this call
          site. `max-w-full` still guards a genuinely narrow viewport. */}
      <div className="inline-flex self-start max-w-full">
      <Selector
        items={[
          { id: "light", label: t("theme.light"), icon: sunIcon },
          { id: "dark", label: t("theme.dark"), icon: moonIcon },
        ]}
        label={t("settings.theme")}
        select="one"
        active={theme}
        onChange={(id) => selectTheme(id as ResolvedTheme)}
        size="lg"
        variant="well"
      />
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Flash-ZIP-Export Card (#28) — MOVED here from Settings' Storage tab (jdp,
// two live-review messages in sequence, the second superseding/refining the
// first: "trenn bitte flash zip export und den rest wieder in zwei separate
// cards", then "soll die flash zip export toggle nicht einfach in den flash
// tab? macht doch mehr sinn" — implemented as the SECOND ask, not both: this
// setting now lives ONLY on the dedicated Flash page (pages/Flash.tsx), not
// as a Settings card at all any more). Exported from Settings.tsx and
// imported into Flash.tsx the same way Config.tsx already imports ToggleRow
// and Recovery.tsx already imports CloudCard/RcloneCard/ToggleRow from this
// same file — a real, pre-existing cross-page reuse pattern, not a new one
// invented for this move.
//
// Self-contained the SAME way VMSSHCard/RcloneCard/CloudCard below already
// are ("fetches its own data so the large SettingsPage doesn't need extra
// state") — necessarily so here, since this Card now renders OUTSIDE
// SettingsPage entirely and can't reach that component's own settings/
// savedSettings state or its save()/autoSaveField()/debouncedSave() helpers.
// Persists via the same "re-fetch the latest settings, merge only the
// fields THIS card owns, PUT" pattern Config.tsx's ConfigSettingsCard
// already established for exactly this situation (a card living on a
// domain's own page, editing a few fields of the shared Settings object) —
// see that component's own handleSave() for the reference implementation.
// Kept the ORIGINAL per-field auto-save behaviour (optimistic flip + revert-
// on-failure + shake for the two toggles, debounce for the path/keep-count
// fields) rather than switching to Config.tsx's single "Save" button: this
// feature already had, and jdp already approved, the no-Speichern-button
// auto-save UX (#142) before this move — relocating a control is not licence
// to silently redesign how it saves.
export function FlashZipExportCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [enabled, setEnabled] = useState(false);
  const [path, setPath] = useState("");
  const [keep, setKeep] = useState(0);
  // Remembers the last "keep N" the user picked so toggling history OFF
  // (which zeroes the persisted count) and back ON restores their count
  // instead of the shipped default — same behaviour, same variable name, as
  // the one this card used to keep inside SettingsPage's own state before
  // the move.
  const [rememberedKeep, setRememberedKeep] = useState(7);
  const [busyEnabled, setBusyEnabled] = useState(false);
  const [busyKeep, setBusyKeep] = useState(false);
  const [shakeEnabled, setShakeEnabled] = useState(0);
  const [shakeKeep, setShakeKeep] = useState(0);
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  const DEBOUNCE_MS = 800;

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (!res.ok) return;
        setEnabled(res.settings.flashZipExportEnabled);
        setPath(res.settings.flashZipExportPath);
        setKeep(res.settings.flashZipExportKeep);
        if (res.settings.flashZipExportKeep > 0) setRememberedKeep(res.settings.flashZipExportKeep);
        setHostMountRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
  }, []);

  // persist re-fetches the current server state and merges ONLY this card's
  // own patch onto it before PUTting — never a stale mount-time snapshot,
  // which could otherwise re-assert a field some OTHER open tab/page has
  // since changed. Mirrors Config.tsx's ConfigSettingsCard.handleSave()
  // exactly (see that component's own comment for the fuller rationale);
  // unlike that one, this fires per-field rather than from one batched
  // "Save" click, so it also mirrors SettingsPage's own save() helper's
  // return-a-boolean contract so callers can revert an optimistic flip.
  async function persist(patch: Partial<Settings>): Promise<boolean> {
    try {
      const latest = await getSettings();
      if (!latest.ok) {
        push(latest.error ?? t("settings.error"), "fail");
        return false;
      }
      const merged: Settings = { ...latest.settings, ...patch };
      const res = await putSettings(merged);
      if (res.ok) {
        push(t("settings.saved"), "success");
        return true;
      }
      push(res.error ?? t("settings.error"), "fail");
      return false;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  async function toggleEnabled(next: boolean) {
    const prev = enabled;
    setEnabled(next);
    setBusyEnabled(true);
    const ok = await persist({ flashZipExportEnabled: next });
    setBusyEnabled(false);
    if (!ok) {
      setEnabled(prev);
      setShakeEnabled((n) => n + 1);
    }
  }

  async function toggleKeepHistory(next: boolean) {
    const prev = keep;
    const nextKeep = next ? rememberedKeep : 0;
    setKeep(nextKeep);
    setBusyKeep(true);
    const ok = await persist({ flashZipExportKeep: nextKeep });
    setBusyKeep(false);
    if (!ok) {
      setKeep(prev);
      setShakeKeep((n) => n + 1);
    }
  }

  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, DEBOUNCE_MS);
  }

  return (
    <Card
      title={t("flash.zipExport.title")}
      hint={`${t("flash.zipExport.hint")} ${t("flash.zipExport.enableHint")}`}
      hueIndex={hueIndex}
    >
      {/* hideLabel: the Card's own title/hint above already carry this
          switch's own explanation (the exact same content, verbatim) — the
          same "everything but the heading moves into a bubble" shape this
          section kept while it was a sub-heading inside Settings' merged
          Exports & Encryption card, now promoted one level since the Card
          itself IS this section. */}
      <ToggleRow
        label={t("flash.zipExport.enable")}
        hideLabel
        checked={enabled}
        onChange={(v) => void toggleEnabled(v)}
        disabled={busyEnabled}
        shakeNonce={shakeEnabled}
      />
      {enabled && (
        <>
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("flash.zipExport.plaintextWarn")}
          </div>
          <FolderBrowser
            label={t("flash.zipExport.path")}
            value={path}
            hostMountRoot={hostMountRoot}
            hint={t("flash.zipExport.pathHint")}
            onChange={(v) => {
              setPath(v);
              debounced("flashZipExportPath", () => void persist({ flashZipExportPath: v }));
            }}
          />
          {!path.trim() && (
            <p className="text-xs text-statusFail -mt-1">{t("flash.zipExport.pathRequired")}</p>
          )}
          <ToggleRow
            label={t("flash.zipExport.keepHistory")}
            hint={t("flash.zipExport.keepHistoryHint")}
            // History is "on" whenever we keep more than a single overwritten zip.
            checked={keep > 0}
            onChange={(v) => void toggleKeepHistory(v)}
            disabled={busyKeep}
            shakeNonce={shakeKeep}
          />
          {keep > 0 ? (
            <label className="flex flex-col gap-1 max-w-40">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t("flash.zipExport.keepN")}
                <InfoBubble tip={t("flash.zipExport.keepNHint")} />
              </span>
              <input
                type="number"
                min={1}
                value={keep}
                onChange={(e) => {
                  const n = Math.max(1, parseInt(e.target.value, 10) || 1);
                  setRememberedKeep(n);
                  setKeep(n);
                  debounced("flashZipExportKeep", () => void persist({ flashZipExportKeep: n }));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ) : (
            <p className="text-xs text-carbon-textMuted">{t("flash.zipExport.latestNote")}</p>
          )}
        </>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Settings page
// ---------------------------------------------------------------------------

// VMSSHCard shows BombVault's SSH public key (to authorize on the Unraid host)
// and a connection test. Self-contained: fetches its own data so the large
// SettingsPage doesn't need extra state.
function VMSSHCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [host, setHost] = useState("");
  const [pub, setPub] = useState("");
  const [testState, setTestState] = useState<"idle" | "testing" | "ok" | "fail">("idle");
  const [testMsg, setTestMsg] = useState<string | null>(null);

  // Ready-to-paste command that authorizes this key on the Unraid host, both for
  // the live session and persistently (Unraid restores root.pubkeys on boot).
  const authorizeCmd = pub
    ? `mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
echo '${pub}' | tee -a /root/.ssh/authorized_keys /boot/config/ssh/root.pubkeys >/dev/null
chmod 600 /root/.ssh/authorized_keys`
    : "";

  useEffect(() => {
    getVMSSH()
      .then((r) => {
        if (r.ok) {
          setHost(r.host ?? "");
          setPub(r.publicKey ?? "");
        }
      })
      .catch(() => undefined);
  }, []);

  async function handleTest() {
    setTestState("testing");
    setTestMsg(null);
    try {
      const r = await testVMSSH();
      if (r.ok) {
        setTestState("ok");
      } else {
        setTestState("fail");
        setTestMsg(r.error ?? t("vm.ssh.testFail"));
      }
    } catch {
      setTestState("fail");
      setTestMsg(t("vm.ssh.testFail"));
    }
  }

  // copyText falls back to execCommand in non-secure contexts (#112). The
  // "Copied" flash used to be a local 2000ms button-label swap
  // (GlimStone form-engine Task 9's copy-feedback candidate); it's now a
  // routine (quiet-mode-suppressible) toast instead — see lib/toast.tsx.
  async function handleCopy() {
    if (await copyText(pub)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      // "failures always surface" (design-language.md) — copyText() only
      // returns false when BOTH the Clipboard API and the execCommand
      // fallback failed, so this is a real, user-actionable failure, not
      // routine noise a quiet-mode user would want suppressed.
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  async function handleCopyCmd() {
    if (await copyText(authorizeCmd)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <Card title={t("vm.ssh.title")} hint={t("vm.ssh.desc")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-3">
        <div className="text-sm text-carbon-text">
          {t("vm.ssh.host")}: <span dir="ltr" className="font-mono text-start">{host || "—"}</span>
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textMuted">{t("vm.ssh.publicKey")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {pub || "—"}
            </code>
            <button
              onClick={handleCopy}
              disabled={!pub}
              className="shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast disabled:opacity-50"
            >
              {t("vm.ssh.copy")}
            </button>
          </div>
        </div>

        {/* One-time setup instructions */}
        <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-2">
          <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
            {t("vm.ssh.setupTitle")}
          </span>
          <ol className="list-decimal ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
            <li>{t("vm.ssh.step1")}</li>
            <li>{t("vm.ssh.step2")}</li>
            <li>{t("vm.ssh.step3")}</li>
          </ol>
          <div className="flex items-start gap-2">
            <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">{authorizeCmd || "—"}</pre>
            <button
              onClick={handleCopyCmd}
              disabled={!pub}
              className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("vm.ssh.copyCmd")}
            </button>
          </div>
          {/* Task 5 (rule 13): was a plain underline-on-hover text link. Task 7:
              tone was "info" (the old fifth hue) only because it was the
              nearest tone available at the time — a plain doc-link badge
              isn't activity or a state, it's the same kind of element as
              Recovery.tsx's own tone="neutral" reload-link badge. */}
          <Badge
            as="a"
            href="https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md"
            target="_blank"
            rel="noreferrer"
            tone="neutral"
            size="small"
            className="self-start"
          >
            {t("vm.ssh.guide")} →
          </Badge>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={handleTest}
            disabled={testState === "testing"}
            className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {testState === "testing" ? t("vm.ssh.testing") : t("vm.ssh.test")}
          </button>
          {testState === "ok" && (
            <span className="text-sm text-statusOk">{t("vm.ssh.testOk")}</span>
          )}
          {testState === "fail" && (
            <span className="text-sm text-statusFail">{testMsg ?? t("vm.ssh.testFail")}</span>
          )}
        </div>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// SettingsPortabilityCard — export this instance's configuration to a JSON file,
// or import a previously exported file. Self-contained: it moves only settings +
// off-site destinations (and, opt-in, the decrypted credentials). Backups,
// snapshots and history are never touched. Import always previews first and asks
// for confirmation before it replaces anything.
// ---------------------------------------------------------------------------

// The machine ids the import summary returns for populated setting areas, mapped
// to their translation keys so the preview lists them human-readably. An unknown
// (future) id falls back to its raw value.
const IMPORT_GROUP_KEYS: Record<string, TranslationKey> = {
  domains: "settingsIO.group.domains",
  schedules: "settingsIO.group.schedules",
  retention: "settingsIO.group.retention",
  offsite: "settingsIO.group.offsite",
  drills: "settingsIO.group.drills",
  digest: "settingsIO.group.digest",
  monitoring: "settingsIO.group.monitoring",
  language: "settingsIO.group.language",
  exportEncryption: "settingsIO.group.exportEncryption",
};

function SettingsPortabilityCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const [includeCreds, setIncludeCreds] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  // Import is a two-step flow: pick a file → preview summary + confirm → apply.
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [importBusy, setImportBusy] = useState<"idle" | "reading" | "applying">("idle");
  const [importError, setImportError] = useState<string | null>(null);
  const [importDone, setImportDone] = useState(false);
  // The parsed preview and the raw file text held for the confirmed apply.
  const [preview, setPreview] = useState<ImportSettingsSummary | null>(null);
  const [pendingText, setPendingText] = useState<string | null>(null);

  function resetImport() {
    setPreview(null);
    setPendingText(null);
    setImportError(null);
    setImportDone(false);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  async function handleExport() {
    setExportError(null);
    setExporting(true);
    // Backend-provided error text (if any) is shown verbatim BY DESIGN — the API
    // answers English and is not translated client-side.
    const err = await exportSettings(includeCreds);
    setExportError(err);
    setExporting(false);
  }

  async function handleFilePicked(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setImportError(null);
    setImportDone(false);
    setPreview(null);
    setImportBusy("reading");
    try {
      const text = await file.text();
      const res = await importSettingsPreview(text);
      if (res.ok && res.summary) {
        setPendingText(text);
        setPreview(res.summary);
      } else {
        setImportError(res.error ?? t("settingsIO.importFailed"));
      }
    } catch (err) {
      setImportError(err instanceof Error ? err.message : String(err));
    } finally {
      setImportBusy("idle");
    }
  }

  async function handleConfirmImport() {
    if (!pendingText) return;
    setImportError(null);
    setImportBusy("applying");
    try {
      const res = await importSettingsApply(pendingText);
      if (res.ok) {
        setImportDone(true);
        setPreview(null);
        setPendingText(null);
        if (fileInputRef.current) fileInputRef.current.value = "";
      } else {
        setImportError(res.error ?? t("settingsIO.importFailed"));
      }
    } catch (err) {
      setImportError(err instanceof Error ? err.message : String(err));
    } finally {
      setImportBusy("idle");
    }
  }

  const busy = importBusy !== "idle" || exporting;

  return (
    <Card title={t("settingsIO.title")} hint={t("settingsIO.desc")} hueIndex={hueIndex}>
      {/* EXPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.exportHeading")}
        </h3>
        <label className="flex items-start gap-2.5 cursor-pointer">
          <input
            type="checkbox"
            checked={includeCreds}
            onChange={(e) => setIncludeCreds(e.target.checked)}
            className="mt-0.5 h-4 w-4 shrink-0 accent-accent"
          />
          <span className="text-sm text-carbon-text">{t("settingsIO.includeCreds")}</span>
        </label>
        {includeCreds && (
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settingsIO.credsWarning")}
          </div>
        )}
        <button
          type="button"
          onClick={() => void handleExport()}
          disabled={busy}
          className="self-start rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50"
        >
          {exporting ? t("settingsIO.exporting") : t("settingsIO.exportButton")}
        </button>
        {exportError && (
          // Backend error text shown verbatim BY DESIGN (English, not translated).
          <span className="text-xs text-statusFail wrap-break-word">✗ {exportError}</span>
        )}
      </div>

      {/* IMPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.importHeading")}
          <InfoBubble tip={t("settingsIO.importHint")} />
        </h3>

        <input
          ref={fileInputRef}
          type="file"
          accept="application/json,.json"
          onChange={(e) => void handleFilePicked(e)}
          className="hidden"
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          className="self-start rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50"
        >
          {importBusy === "reading" ? t("settingsIO.reading") : t("settingsIO.chooseFile")}
        </button>

        {/* Preview + confirmation before anything is written. */}
        {preview && (
          <div className="rounded-card bg-carbon-surface2 p-4 flex flex-col gap-3">
            <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("settingsIO.previewTitle")}
            </span>
            <dl className="flex flex-col gap-1.5 text-xs text-carbon-text">
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewExportedAt")}</dt>
                <dd className="font-mono text-end wrap-break-word">{preview.exportedAt || "-"}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewAppVersion")}</dt>
                <dd className="font-mono text-end wrap-break-word">{preview.appVersion || "-"}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewOffsiteTargets")}</dt>
                <dd dir="ltr" className="font-mono text-start">{preview.offsiteTargets}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewCredentials")}</dt>
                <dd className="text-end">
                  {preview.credentials.present
                    ? t("settingsIO.previewCredsIncluded")
                    : t("settingsIO.previewCredsNotIncluded")}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted shrink-0">{t("settingsIO.previewSettingsAreas")}</dt>
                <dd className="text-end wrap-break-word">
                  {preview.settingsGroups.length > 0
                    ? preview.settingsGroups
                        .map((g) => (IMPORT_GROUP_KEYS[g] ? t(IMPORT_GROUP_KEYS[g]) : g))
                        .join(", ")
                    : t("settingsIO.previewNone")}
                </dd>
              </div>
            </dl>
            <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
              {t("settingsIO.confirmWarning")}
            </div>
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => void handleConfirmImport()}
                disabled={busy}
                className="rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {importBusy === "applying" ? t("settingsIO.importing") : t("settingsIO.confirmButton")}
              </button>
              <button
                type="button"
                onClick={resetImport}
                disabled={busy}
                className="rounded-control bg-carbon-surface3 hover:bg-carbon-border px-4 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50"
              >
                {t("settingsIO.cancel")}
              </button>
            </div>
          </div>
        )}

        {importDone && (
          <span className="text-xs text-statusOk">✓ {t("settingsIO.importSuccess")}</span>
        )}
        {importError && (
          // Backend error text shown verbatim BY DESIGN (English, not translated).
          <span className="text-xs text-statusFail wrap-break-word">✗ {importError}</span>
        )}
      </div>
    </Card>
  );
}

// The companion dashboard-tile plugin's .plg URL + repo — shown for manual
// install when SSH is missing, and linked for transparency before installing.
// (Install itself uses a hard-coded server-side constant; these are display-only.)
const DASH_PLUGIN_PLG_URL =
  "https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg";
const DASH_PLUGIN_REPO_URL = "https://github.com/junkerderprovinz/bombvault-widget";

type DashPluginStatus =
  | { kind: "loading" }
  | { kind: "noSsh" }
  | { kind: "absent" }
  | { kind: "installed"; version: string }
  | { kind: "error"; message: string; output?: string };

// UnraidTileSection — the "Unraid dashboard tile" block inside the Dashboard
// widget card: one-click install/remove of the companion bombvaultwidget plugin
// over the existing host SSH connection. Without SSH it degrades to manual
// instructions (the copyable .plg URL + a CA hint).
function UnraidTileSection({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const { push } = useToast();
  // `status` stays exactly as it was — GlimStone follow-up pass (v8.0.0)
  // audit note: this is a PERSISTENT "is the tile currently installed" fact
  // (plus, on failure, a possibly multi-line command `output` block), not a
  // one-shot completion notice — a poor fit for a 4s, w-80 toast, so it's
  // deliberately left as inline status rather than forced into one. Only the
  // two genuinely ephemeral notices below (the URL copy-feedback swap and the
  // ok-alongside-persistent-status install flash) moved to toasts.
  const [status, setStatus] = useState<DashPluginStatus>({ kind: "loading" });
  const [busy, setBusy] = useState<"idle" | "install" | "remove">("idle");

  function refresh() {
    getDashboardPlugin()
      .then((r) => {
        if (!r.ok) {
          setStatus({ kind: "error", message: r.error ?? t("settings.error") });
        } else if (!r.sshConfigured) {
          setStatus({ kind: "noSsh" });
        } else if (r.installed) {
          setStatus({ kind: "installed", version: r.version ?? "" });
        } else {
          setStatus({ kind: "absent" });
        }
      })
      .catch((err) => {
        setStatus({
          kind: "error",
          message: err instanceof Error ? err.message : t("settings.error"),
        });
      });
  }

  useEffect(refresh, []); // eslint-disable-line react-hooks/exhaustive-deps -- status check on card mount only

  async function run(op: "install" | "remove") {
    setBusy(op);
    try {
      const r = await (op === "install" ? installDashboardPlugin() : removeDashboardPlugin());
      if (r.ok) {
        if (op === "install") push(t("settings.dashTileInstallOk"), "success");
        refresh();
      } else {
        setStatus({
          kind: "error",
          message: r.error ?? t("settings.error"),
          output: r.output,
        });
      }
    } catch (err) {
      setStatus({
        kind: "error",
        message: err instanceof Error ? err.message : t("settings.error"),
      });
    } finally {
      setBusy("idle");
    }
  }

  async function handleCopyUrl() {
    if (await copyText(DASH_PLUGIN_PLG_URL)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("settings.dashTile")}
        <InfoBubble tip={t("settings.dashTileHint")} />
      </h3>

      {status.kind === "loading" && (
        <span className="text-xs text-carbon-textMuted">{t("settings.dashTileChecking")}</span>
      )}

      {status.kind === "noSsh" && (
        <div className="flex flex-col gap-2">
          <p className="text-xs text-carbon-textSub">{t("settings.dashTileNoSsh")}</p>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {DASH_PLUGIN_PLG_URL}
            </code>
            <button
              type="button"
              onClick={() => void handleCopyUrl()}
              className="shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast"
            >
              {t("vm.ssh.copy")}
            </button>
          </div>
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileCa")}</p>
        </div>
      )}

      {status.kind === "absent" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-carbon-text">{t("settings.dashTileNotInstalled")}</span>
          {/* Transparency BEFORE the call: what Install does, and where the code lives. */}
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileConfirm")}</p>
          {/* Task 5 (rule 13): was a plain underline-on-hover text link. Task 7:
              same reasoning as vm.ssh.guide's badge above — plain doc-link,
              tone="neutral" not the old fifth hue. */}
          <Badge
            as="a"
            href={DASH_PLUGIN_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            tone="neutral"
            size="small"
            className="self-start"
          >
            {t("settings.dashTileRepo")} →
          </Badge>
          <button
            type="button"
            onClick={() => void run("install")}
            disabled={busy !== "idle"}
            className="self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {busy === "install" ? t("settings.dashTileInstalling") : t("settings.dashTileInstall")}
          </button>
        </div>
      )}

      {status.kind === "installed" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-statusOk">
            ✓{" "}
            {status.version
              ? t("settings.dashTileInstalled").replace("{version}", status.version)
              : t("settings.dashTileInstalledNoV")}
          </span>
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileInstalledHint")}</p>
          <button
            type="button"
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            className="self-start rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50"
          >
            {busy === "remove" ? t("settings.dashTileRemoving") : t("settings.dashTileRemove")}
          </button>
        </div>
      )}

      {status.kind === "error" && (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-statusFail wrap-break-word">✗ {status.message}</span>
          {status.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {status.output}
            </pre>
          )}
          <button
            type="button"
            onClick={() => {
              setStatus({ kind: "loading" });
              refresh();
            }}
            className="self-start rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover"
          >
            {t("whatsnew.retry")}
          </button>
        </div>
      )}
    </div>
  );
}

// DashboardWidgetCard manages the embeddable activity-log widget (GET /widget):
// generate/rotate/disable its access token, show the copyable widget URL and a
// live iframe preview. The token is a show-once secret — the server stores it
// but never echoes it back (settings GET only reports widgetTokenSet), so the
// URL + preview render only right after generating; after a reload the card
// shows the kept-placeholder until the user regenerates.
function DashboardWidgetCard({
  t,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const reveal = useReveal();

  const widgetUrl = token ? `${window.location.origin}/widget?token=${token}` : null;

  // GlimStone follow-up pass (v8.0.0): same migration as FleetSettingsCard's
  // own generate/disable/copy handlers further down — see that card's comment.
  async function handleGenerate() {
    setBusy(true);
    try {
      const r = await generateWidgetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    try {
      const r = await disableWidgetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!widgetUrl) return;
    if (await copyText(widgetUrl)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <Card title={t("settings.widget")} hint={t("settings.widgetHint")} hueIndex={hueIndex}>

      <ul className="list-disc ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.widgetHow")}</li>
        <li>{t("settings.widgetAccess")}</li>
        <li>{t("settings.widgetEnglish")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.widgetToken")}</span>
          {/* Show-once secret: value is only the freshly generated token; a
              stored-but-unknown one renders the cloud.secretSet placeholder.
              The verify/regenerate/disable actions sit on their OWN line
              below the field (design-language.md's reveal-eye rule), not
              beside it in the same row. */}
          <RevealInput
            {...reveal}
            readOnly
            value={token ?? ""}
            placeholder={token ? "" : t("cloud.secretSet")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
          />
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.widgetRegenerate")}
            </button>
            <button
              type="button"
              onClick={() => void handleDisable()}
              disabled={busy}
              className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.widgetDisable")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className="self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {t("settings.widgetGenerate")}
        </button>
      )}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.widgetUrlOnce")}</p>
      )}
      {widgetUrl && (
        <>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetUrl")}</span>
            <div className="flex items-start gap-2">
              <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
                {widgetUrl}
              </code>
              <button
                type="button"
                onClick={() => void handleCopy()}
                className="shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast"
              >
                {t("vm.ssh.copy")}
              </button>
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetPreview")}</span>
            <iframe
              src={widgetUrl}
              title={t("settings.widgetPreview")}
              className="w-full max-w-[560px] h-[300px] rounded-card bg-carbon-surface2"
            />
          </div>
        </>
      )}

      {/* Companion Unraid dashboard-tile plugin (one-click install over SSH). */}
      <UnraidTileSection t={t} />
    </Card>
  );
}

// FleetSettingsCard manages this instance's own identity for the Fleet view:
// the display name reported to polling peers, and the peer status token (GET
// /api/fleet/status) that authorizes OTHER instances to poll THIS one. The
// token follows the exact same show-once secret contract as the widget token
// (generate/rotate/disable, never echoed back after the fact).
function FleetSettingsCard({
  t,
  settings,
  setSettings,
  save,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [nameSaveState, setNameSaveState] = useState<SaveState>("idle");
  const [nameSaveError, setNameSaveError] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const reveal = useReveal();

  // GlimStone follow-up pass (v8.0.0): the persistent "✗ {error}" banner
  // (never auto-cleared; only reset by the next generate/disable attempt) is
  // now a toast — a generate/disable outcome is the same one-shot completion
  // notice handleSetPassword's own migration already established the pattern
  // for, above.
  async function handleGenerate() {
    setBusy(true);
    try {
      const r = await generateFleetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    try {
      const r = await disableFleetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy(false);
    }
  }

  // copyText falls back to execCommand in non-secure contexts (#112). Mirrors
  // VMSSHCard's own handleCopy migration: the local button-label swap is now
  // a routine (quiet-mode-suppressible) toast — see lib/toast.tsx.
  async function handleCopy() {
    if (!token) return;
    if (await copyText(token)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <Card title={t("settings.fleet")} hint={t("settings.fleetHint")} hueIndex={hueIndex}>

      <div className="flex flex-col gap-1.5">
        <label className="text-xs text-carbon-textSub">{t("settings.instanceName")}</label>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={settings.instanceName}
            onChange={(e) => setSettings((prev) => (prev ? { ...prev, instanceName: e.target.value } : prev))}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower"
            className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
          />
        </div>
        <SaveBar
          state={nameSaveState}
          error={nameSaveError}
          onSave={() => void save({ instanceName: settings.instanceName }, setNameSaveState, setNameSaveError)}
          t={t}
        />
      </div>

      <ul className="list-disc ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.fleetHow")}</li>
        <li>{t("settings.fleetAccess")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetToken")}</span>
          {/* Actions on their own line below the field, not beside it —
              same reveal-eye layout rule as DashboardWidgetCard above. */}
          <RevealInput
            {...reveal}
            readOnly
            value={token ?? ""}
            placeholder={token ? "" : t("cloud.secretSet")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
          />
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.fleetRegenerate")}
            </button>
            <button
              type="button"
              onClick={() => void handleDisable()}
              disabled={busy}
              className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.fleetDisable")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className="self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {t("settings.fleetGenerate")}
        </button>
      )}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.fleetTokenOnce")}</p>
      )}
      {token && (
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetTokenPasteHint")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {token}
            </code>
            <button
              type="button"
              onClick={() => void handleCopy()}
              className="shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast"
            >
              {t("vm.ssh.copy")}
            </button>
          </div>
          <p className="text-caption text-carbon-textMuted">
            {t("settings.fleetUrlHint").replace("{url}", window.location.origin)}
          </p>
        </div>
      )}
    </Card>
  );
}

// RcloneCard manages the off-site rclone config (paste rclone.conf). It is
// stored encrypted; only the remote NAMES are read back for display. Backup
// paths can then be set to "rclone:<remote>:<bucket>" in Backup Paths.
export function RcloneCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [remotes, setRemotes] = useState<string[]>([]);
  const [conf, setConf] = useState("");
  const [state, setState] = useState<SaveState>("idle");

  function refresh() {
    getRclone()
      .then((r) => {
        if (r.ok) setRemotes(r.remotes ?? []);
      })
      .catch(() => undefined);
  }
  useEffect(() => {
    refresh();
  }, []);

  // GlimStone follow-up pass (v8.0.0): the "saved"/"error" 3000ms inline flash
  // is now a toast, same shape as the shared save() helper further down.
  async function handleSave() {
    setState("saving");
    try {
      const r = await setRclone(conf);
      if (r.ok) {
        setState("idle");
        setConf("");
        refresh();
        push(t("settings.saved"), "success");
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  return (
    <Card title={t("rclone.title")} hint={t("rclone.hint")} hueIndex={hueIndex}>
      <div className="text-sm text-carbon-text">
        {t("rclone.configured")}:{" "}
        <span dir="ltr" className="font-mono text-start">{remotes.length > 0 ? remotes.join(", ") : "—"}</span>
      </div>
      <textarea
        value={conf}
        onChange={(e) => setConf(e.target.value)}
        spellCheck={false}
        rows={6}
        placeholder={"[b2]\ntype = b2\naccount = ...\nkey = ..."}
        dir="ltr"
        className="rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-3 py-2 bv-field-focus text-start"
      />
      {/* GlimStone follow-up pass (Phase 2 Task 4's remainder): stays permanent
          text, NOT bubbled — it names the exact "rclone:<remote>:<bucket>/path"
          Backup Path syntax, which is the ONLY place that convention is
          documented (PathModeSwitch's own remote-mode placeholder shows only
          s3:/rest: examples, never rclone:). Someone back on the Storage tab
          filling in a Backup Path for a domain they just wired up here needs
          this findable without already knowing to hover an icon on a
          different tab — the same "exact syntax to copy correctly" carve-out
          the task spec calls out by name. */}
      <p className="text-xs text-carbon-textMuted">{t("rclone.pathHint")}</p>
      <div className="flex items-center gap-3 pt-1">
        <button
          onClick={() => void handleSave()}
          disabled={state === "saving" || conf.trim() === ""}
          className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {state === "saving" ? t("auth.saving") : t("rclone.save")}
        </button>
      </div>
    </Card>
  );
}

// CloudCard stores credentials for off-site restic backends (S3 + restic REST),
// kept encrypted. Secrets are write-only: blank on load, blank-on-save keeps the
// stored value. Field labels are restic's actual env var names (self-documenting).
export function CloudCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [c, setC] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
  const [secretSet, setSecretSet] = useState(false);
  const [pwSet, setPwSet] = useState(false);
  const [state, setState] = useState<SaveState>("idle");
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();

  function refresh() {
    getCloud()
      .then((r) => {
        if (r.ok) {
          setC((p) => ({ ...p, s3KeyId: r.s3KeyId ?? "", s3Region: r.s3Region ?? "", restUser: r.restUser ?? "", s3StorageClass: r.s3StorageClass ?? "" }));
          setSecretSet(!!r.s3SecretSet);
          setPwSet(!!r.restPasswordSet);
        }
      })
      .catch(() => undefined);
  }
  useEffect(refresh, []);

  function set<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
  }

  // GlimStone follow-up pass (v8.0.0): "saved"/"error" flash -> toast.
  async function handleSave() {
    setState("saving");
    try {
      const r = await setCloud(c);
      if (r.ok) {
        setState("idle");
        setC((p) => ({ ...p, s3Secret: "", restPassword: "" }));
        refresh();
        push(t("settings.saved"), "success");
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.title")} hueIndex={hueIndex}>
      {/* GlimStone follow-up pass (Phase 2 Task 4's remainder): stays permanent
          text, NOT bubbled — it is the only complete reference for all four
          remote-URL prefixes this card's credentials unlock (s3:/rest:/b2:/
          sftp:), used on a DIFFERENT tab's Backup Path fields. Those fields'
          own placeholder only ever shows two of the four (s3:/rest:), so this
          paragraph is the sole place b2: and sftp: are documented at all —
          exactly the "exact path syntax they need to copy correctly" carve-out
          the task spec names, same reasoning as RcloneCard's own pathHint. */}
      <p className="text-xs text-carbon-textMuted -mt-1">{t("cloud.hint")}</p>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
        <label className={fieldCls}>AWS_ACCESS_KEY_ID
          <input value={c.s3KeyId} onChange={(e) => set("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
        <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
          <RevealInput {...revealS3Secret} value={c.s3Secret} onChange={(e) => set("s3Secret", e.target.value)} spellCheck={false}
            placeholder={secretSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
        <label className={fieldCls}>AWS_DEFAULT_REGION
          <input value={c.s3Region} onChange={(e) => set("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
        <label className={fieldCls}>
          <span className="flex items-center gap-1">
            {t("cloud.storageClass.label")}
            <InfoBubble tip={t("cloud.storageClass.hint")} />
          </span>
          <select value={c.s3StorageClass} onChange={(e) => set("s3StorageClass", e.target.value)} className={inputCls}>
            <option value="">{t("cloud.storageClass.default")}</option>
            <option value="STANDARD">STANDARD</option>
            <option value="STANDARD_IA">STANDARD_IA</option>
            <option value="ONEZONE_IA">ONEZONE_IA</option>
            <option value="INTELLIGENT_TIERING">INTELLIGENT_TIERING</option>
            <option value="GLACIER_IR">GLACIER_IR</option>
          </select></label>
      </div>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
        <label className={fieldCls}>RESTIC_REST_USERNAME
          <input value={c.restUser} onChange={(e) => set("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
        <label className={fieldCls}>RESTIC_REST_PASSWORD
          <RevealInput {...revealRestPassword} value={c.restPassword} onChange={(e) => set("restPassword", e.target.value)} spellCheck={false}
            placeholder={pwSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
      </div>

      <div className="flex items-center gap-3 pt-1">
        <button
          onClick={() => void handleSave()}
          disabled={state === "saving"}
          className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {state === "saving" ? t("auth.saving") : t("settings.save")}
        </button>
      </div>
    </Card>
  );
}

// toDraft turns a secret-blanked CloudCredSetInfo (from GET) into an editable
// CloudCredSet with blank secret fields — sending it back with those fields
// blank is exactly what makes the backend's keep-prior-if-blank merge (matched
// by id) preserve the real stored secret, so an untouched set's key/password
// survives a save that only edited a DIFFERENT set in the same list.
function toDraft(s: CloudCredSetInfo): CloudCredSet {
  return { id: s.id, name: s.name, s3KeyId: s.s3KeyId, s3Secret: "", s3Region: s.s3Region, restUser: s.restUser, restPassword: "", s3StorageClass: s.s3StorageClass };
}

// CloudCredSetsCard manages ADDITIONAL named credential sets (#141 stage 2):
// lets an off-site target (OffsiteTargetsSection's editor) opt into its OWN
// S3/restic-REST credentials instead of sharing the single set CloudCard
// manages above — e.g. two S3 endpoints (Hetzner + a local Garage/MinIO) that
// need different keys. Same write-only-secret contract as CloudCard; the
// whole list round-trips through setCloudCredSets (replace-all), which is why
// every save resends every set (via toDraft — see its own comment for why
// that is safe for the sets NOT being edited).
export function CloudCredSetsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [sets, setSets] = useState<CloudCredSetInfo[]>([]);
  const [editing, setEditing] = useState<CloudCredSet | null>(null);
  const [state, setState] = useState<SaveState>("idle");
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();

  function refresh() {
    getCloudCredSets()
      .then((r) => { if (r.ok) setSets(r.sets ?? []); })
      .catch(() => undefined);
  }
  useEffect(refresh, []);

  function openNew() {
    // randomId(), not crypto.randomUUID() — the latter is secure-context-only
    // and BombVault ships a documented plain-HTTP mode, where it is undefined
    // and this click would throw instead of opening the editor (see lib/uuid.ts).
    setEditing({ id: randomId(), name: "", s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
    setState("idle");
  }
  function openEdit(s: CloudCredSetInfo) {
    setEditing(toDraft(s));
    setState("idle");
  }
  function closeEditor() {
    setEditing(null);
    setState("idle");
  }
  function setField<K extends keyof CloudCredSet>(k: K, v: CloudCredSet[K]) {
    setEditing((p) => (p ? { ...p, [k]: v } : p));
  }

  // GlimStone follow-up pass (v8.0.0): the "saved"/"error" flash below is now
  // a toast (save() previously had no success notice at all, since closeEditor()
  // removed the form before it could show one — push() fixes that too). remove()'s
  // failure used to set `msg` with nothing left mounted to render it (the editor
  // closes on remove, and `state` never becomes "error" from remove() alone) — a
  // latent dead branch this migration also resolves, now that both routes push
  // the same way.
  async function save() {
    if (!editing) return;
    setState("saving");
    const isNew = !sets.some((s) => s.id === editing.id);
    const rest = sets.filter((s) => s.id !== editing.id).map(toDraft);
    const next = isNew ? [...rest, editing] : [...rest, editing];
    try {
      const r = await setCloudCredSets(next);
      if (r.ok) {
        setState("idle");
        closeEditor();
        refresh();
        push(t("settings.saved"), "success");
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  async function remove(id: string) {
    setRemovingId(id);
    try {
      const next = sets.filter((s) => s.id !== id).map(toDraft);
      const r = await setCloudCredSets(next);
      if (r.ok) {
        refresh();
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setRemovingId(null);
      setConfirmRemove(null);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.credSets.title")} hint={t("cloud.credSets.hint")} hueIndex={hueIndex}>

      {sets.length === 0 && !editing && (
        <span className="text-xs text-carbon-textMuted">{t("cloud.credSets.none")}</span>
      )}

      {sets.map((s) => (
        <div key={s.id} className="flex items-start justify-between gap-3 rounded-card bg-carbon-surface2 p-3">
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-sm text-carbon-text truncate">{s.name}</span>
            <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">
              {s.s3KeyId || s.restUser || "—"}
            </span>
          </div>
          <div className="flex shrink-0 items-start gap-2">
            <button
              type="button"
              onClick={() => openEdit(s)}
              className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover"
            >
              {t("offsite.targets.edit")}
            </button>
            {confirmRemove === s.id ? (
              <button
                type="button"
                onClick={() => void remove(s.id)}
                disabled={removingId === s.id}
                className="rounded-control bg-statusFailBg px-2.5 py-1 text-xs font-medium text-statusFail hover:bg-statusFailBgHover disabled:opacity-50"
              >
                {removingId === s.id ? t("offsite.targets.removing") : t("offsite.targets.confirmRemove")}
              </button>
            ) : (
              <button
                type="button"
                onClick={() => setConfirmRemove(s.id)}
                className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-statusFail hover:bg-carbon-hover"
              >
                {t("offsite.targets.remove")}
              </button>
            )}
          </div>
        </div>
      ))}

      {editing ? (
        <div className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3">
          <label className={fieldCls}>{t("cloud.credSets.name")}
            <input value={editing.name} onChange={(e) => setField("name", e.target.value)} className={inputCls} /></label>
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface3/40 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
            <label className={fieldCls}>AWS_ACCESS_KEY_ID
              <input value={editing.s3KeyId} onChange={(e) => setField("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
            <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
              <RevealInput {...revealS3Secret} value={editing.s3Secret} onChange={(e) => setField("s3Secret", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.s3SecretSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
            <label className={fieldCls}>AWS_DEFAULT_REGION
              <input value={editing.s3Region} onChange={(e) => setField("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
            <label className={fieldCls}>{t("cloud.storageClass.label")}
              <select value={editing.s3StorageClass} onChange={(e) => setField("s3StorageClass", e.target.value)} className={inputCls}>
                <option value="">{t("cloud.storageClass.default")}</option>
                <option value="STANDARD">STANDARD</option>
                <option value="STANDARD_IA">STANDARD_IA</option>
                <option value="ONEZONE_IA">ONEZONE_IA</option>
                <option value="INTELLIGENT_TIERING">INTELLIGENT_TIERING</option>
                <option value="GLACIER_IR">GLACIER_IR</option>
              </select></label>
          </div>
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface3/40 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
            <label className={fieldCls}>RESTIC_REST_USERNAME
              <input value={editing.restUser} onChange={(e) => setField("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
            <label className={fieldCls}>RESTIC_REST_PASSWORD
              <RevealInput {...revealRestPassword} value={editing.restPassword} onChange={(e) => setField("restPassword", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.restPasswordSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => void save()}
              disabled={state === "saving"}
              className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {state === "saving" ? t("auth.saving") : t("settings.save")}
            </button>
            <button
              onClick={closeEditor}
              className="rounded-control bg-carbon-surface3 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover"
            >
              {t("common.close")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={openNew}
          className="self-start rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-hover"
        >
          + {t("cloud.credSets.add")}
        </button>
      )}
    </Card>
  );
}

// emptyNotify is the default notification config shown before the saved one loads.
const emptyNotify: NotifyConfig = {
  on: "never",
  webhookUrl: "",
  webhookFormat: "generic",
  matrixHomeserver: "",
  matrixToken: "",
  matrixRoom: "",
  healthchecksUrl: "",
  unraid: false,
  smtpEnabled: false,
  smtpHost: "",
  smtpPort: 587,
  smtpUsername: "",
  smtpPassword: "",
  smtpFrom: "",
  smtpTo: "",
  smtpTls: "starttls",
  appriseUrl: "",
  appriseTags: "",
  scheduledSummary: false,
  notifyOnUpdate: false,
};

// NotifyCard configures backup notifications (webhook / Matrix / Healthchecks).
// Stored encrypted at rest; the form pre-fills from the saved config and Test
// sends to the CURRENT form values (no save needed).
// NotifyCard's own hint→bubble pass (GlimStone form-engine Phase 2, Task 4):
// this Card, plus the Weekly-digest and Overdue-watchdog Cards further down
// (the whole "notifications" tab — the only complete, self-contained tab
// migrated by that task; every OTHER Settings tab's permanent hint <p>s were
// left untouched, deliberately, same scope discipline as Phase 1 Task 9's
// toast adoption — Task 4 documented its own remainder rather than force a
// same-sitting judgment call on ~80 more sites it hadn't yet triaged), moved
// these 7 disposable-after-first-read hints into
// InfoBubble: three card-level intros — NotifyCard's own, the Weekly-digest
// Card's, and the Overdue-watchdog Card's (all three now Card's own `hint`
// prop) — plus four inline ones: the "scheduled summary" and "notify on
// update" checkbox captions, the Apprise section intro, and the
// per-domain-Healthchecks section intro.
//
// Two hints in THIS card were deliberately left as permanent text, not
// bubbled, because they read as reference a user consults again later
// rather than a one-time "what does this do" explainer (the spec's own
// test): notify.unraidHint names the EXACT error string ("libvirt not
// reachable") to ignore when VMs aren't backed up — someone hitting that
// message while debugging needs it findable on the page, not behind a hover
// target they have to already know exists; notify.healthchecksLifecycle
// documents a non-obvious cross-setting interaction (Healthchecks pings
// regardless of the "notify on" policy above it) that's exactly the kind of
// "why is this behaving unexpectedly" answer someone comes back to, not
// something read once and never needed again. Both stay as-is below.
//
// GlimStone follow-up pass (v8.0.0): closed out the rest of the file's
// remainder Task 4 flagged above — every other tab's Card-level and
// field-level permanent hint <p>s are now bubbled too, on the exact same
// mechanism (Card's `hint` prop; FolderBrowser gained the identical optional
// `hint` prop for its two Settings.tsx call sites that had one). A small
// family of sites earned the SAME "reference, not a one-time explainer"
// carve-out as this card's own two: RcloneCard's pathHint and CloudCard's
// own hint (both name exact Backup Path URL-prefix syntax used on a
// different tab), settings.metricsHint (names the exact /metrics path +
// Authorization header — see its own call site's comment), and
// jobs.flashNotImplemented (a behavioural caveat, not an explainer). One
// site — settings.offsiteHint — was a genuine toss-up between "syntax
// reference" and "already covered by the field's own placeholder + caption"
// and was left as-is with its own comment rather than force that call here.
function NotifyCard({
  t,
  platformKind,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  // The detected/overridden platform.Kind ("unraid" | "generic" | "truenas"),
  // sourced from GET /api/settings' sibling "platform" field. Drives the
  // mismatch banner below the Unraid toggle (code-review fix: a c.Unraid=true
  // + Kind()!=KindUnraid mismatch used to silently disable the feature with
  // no UI trace — see unraidGate's doc comment in internal/api/service.go for
  // why the backend gate itself stays hard rather than trusting the toggle).
  platformKind: string;
  hueIndex?: number;
}) {
  const { push } = useToast();
  // Simple mode still gets notify-on-failure via Unraid; the extra channels
  // (webhook/Matrix/Healthchecks/SMTP) are power-user features, so gate those.
  const { advanced } = useAdvanced();
  const [cfg, setCfg] = useState<NotifyConfig>(emptyNotify);
  const [state, setState] = useState<SaveState>("idle");
  // The SMTP password / Matrix token are never sent to the browser; track whether
  // one is stored so the field shows "configured" and a blank submit keeps it.
  const [secretSet, setSecretSet] = useState({ smtp: false, matrix: false });
  const revealMatrixToken = useReveal();
  const revealSmtpPassword = useReveal();

  useEffect(() => {
    getNotify()
      .then((r) => {
        if (r.ok && r.notify) setCfg({ ...emptyNotify, ...r.notify });
        setSecretSet({ smtp: !!r.smtpPasswordSet, matrix: !!r.matrixTokenSet });
      })
      .catch(() => undefined);
  }, []);

  function set<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
  }

  // GlimStone follow-up pass (v8.0.0): both the save flash and the separate
  // "tested" flash below are now toasts — same one-shot-completion-notice
  // reasoning as the shared save() helper further down.
  async function handleSave() {
    setState("saving");
    try {
      const r = await setNotify(cfg);
      if (r.ok) {
        setState("idle");
        push(t("settings.saved"), "success");
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  async function handleTest() {
    try {
      const r = await testNotify(cfg);
      if (r.ok) {
        push(t("notify.tested"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus-well";
  // Card-level sibling of selectCls: same styling, but this one sits directly on
  // the Card (bg-carbon-surface), so its fill is surface2 — the panel-level
  // fields above use surface3 because they sit ON a surface2 panel.
  const selectCardCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus";
  const labelCls = "flex flex-col gap-1 text-xs text-carbon-textSub";

  return (
    <Card title={t("notify.title")} hint={t("notify.hint")} hueIndex={hueIndex}>
      <label className={labelCls}>
        {t("notify.on")}
        <select value={cfg.on} onChange={(e) => set("on", e.target.value)} className={selectCardCls}>
          <option value="never">{t("notify.onNever")}</option>
          <option value="failure">{t("notify.onFailure")}</option>
          <option value="always">{t("notify.onAlways")}</option>
        </select>
      </label>

      {/* #56: one summary per scheduled run instead of one message per container. */}
      <label className="flex items-start gap-2 rounded-card bg-carbon-surface2 p-3 cursor-pointer">
        <input
          type="checkbox"
          checked={cfg.scheduledSummary}
          onChange={(e) => set("scheduledSummary", e.target.checked)}
          className="mt-0.5"
          style={{ accentColor: "var(--accent)" }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="flex items-center gap-1 text-sm text-carbon-text">
            {t("notify.scheduledSummary")}
            <InfoBubble tip={t("notify.scheduledSummaryHint")} />
          </span>
        </span>
      </label>

      {/* #56: notify when a container is updated by the post-backup image update. */}
      <label className="flex items-start gap-2 rounded-card bg-carbon-surface2 p-3 cursor-pointer">
        <input
          type="checkbox"
          checked={cfg.notifyOnUpdate}
          onChange={(e) => set("notifyOnUpdate", e.target.checked)}
          className="mt-0.5"
          style={{ accentColor: "var(--accent)" }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="flex items-center gap-1 text-sm text-carbon-text">
            {t("notify.notifyOnUpdate")}
            <InfoBubble tip={t("notify.notifyOnUpdateHint")} />
          </span>
        </span>
      </label>

      {/* Unraid native notifications (delivered over the host SSH connection).
          notify.unraidHint stays permanent text, NOT a bubble — see this
          Card's own header comment above for why (it names the exact
          "libvirt not reachable" error string to ignore, which needs to stay
          findable on the page for someone debugging that message, not
          hidden behind a hover target). */}
      <label className="flex items-start gap-2 rounded-card bg-carbon-surface2 p-3 cursor-pointer">
        <input
          type="checkbox"
          checked={cfg.unraid}
          onChange={(e) => set("unraid", e.target.checked)}
          className="mt-0.5"
          style={{ accentColor: "var(--accent)" }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="text-sm text-carbon-text">{t("notify.unraid")}</span>
          <span className="text-xs text-carbon-textMuted">{t("notify.unraidHint")}</span>
        </span>
      </label>

      {/* Platform-mismatch banner (code-review fix): the toggle above is ON,
          but BombVault's platform detection did not resolve to Unraid, so the
          backend gate (unraidGate, internal/api/service.go) is silently
          keeping every Unraid-only push disabled. Most often a genuinely
          Unraid host whose container is missing the /boot -> /host/boot bind
          mount the template wires up — surfaced here so a user relying only
          on the toggle (never clicking "Send test" below) still finds out. */}
      {cfg.unraid && platformKind !== "unraid" && (
        <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
          {t("notify.unraidPlatformMismatch").replace("{platform}", platformKind)}
        </div>
      )}

      {advanced && (
        <>
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <label className={labelCls}>
          {t("notify.webhook")}
          <input value={cfg.webhookUrl} onChange={(e) => set("webhookUrl", e.target.value)} spellCheck={false}
            placeholder="https://discord.com/api/webhooks/..." dir="ltr" className={`${inputCls} text-start`} />
        </label>
        <label className={labelCls}>
          {t("notify.webhookFormat")}
          <select value={cfg.webhookFormat} onChange={(e) => set("webhookFormat", e.target.value)} className={selectCls}>
            <option value="generic">Generic JSON</option>
            <option value="discord">Discord</option>
            <option value="slack">Slack</option>
            <option value="gotify">Gotify</option>
            <option value="ntfy">ntfy</option>
          </select>
        </label>
      </div>

      {/* Apprise API: posts to a user-run apprise-api server, unlocking Apprise's
          100+ services without bundling Python. Shares the card's Save + Test bar
          like the other channels. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="flex items-center gap-1 text-xs font-medium text-carbon-textSub">
          {t("notify.apprise")}
          <InfoBubble tip={t("notify.appriseHint")} />
        </span>
        <label className={labelCls}>
          {t("notify.appriseUrl")}
          <input value={cfg.appriseUrl} onChange={(e) => set("appriseUrl", e.target.value)} spellCheck={false}
            placeholder="http://apprise:8000/notify/bombvault" dir="ltr" className={`${inputCls} text-start`} />
        </label>
        <label className={labelCls}>
          {t("notify.appriseTags")}
          <input value={cfg.appriseTags} onChange={(e) => set("appriseTags", e.target.value)} spellCheck={false}
            placeholder="backups,homelab" className={inputCls} />
        </label>
      </div>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-medium text-carbon-textSub">{t("notify.matrix")}</span>
        <label className={labelCls}>
          {t("notify.matrixHomeserver")}
          <input value={cfg.matrixHomeserver} onChange={(e) => set("matrixHomeserver", e.target.value)} spellCheck={false}
            placeholder="https://matrix.org" dir="ltr" className={`${inputCls} text-start`} />
        </label>
        <label className={labelCls}>
          {t("notify.matrixToken")}
          <RevealInput {...revealMatrixToken} value={cfg.matrixToken} onChange={(e) => set("matrixToken", e.target.value)} spellCheck={false}
            placeholder={secretSet.matrix ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} />
        </label>
        <label className={labelCls}>
          {t("notify.matrixRoom")}
          <input value={cfg.matrixRoom} onChange={(e) => set("matrixRoom", e.target.value)} spellCheck={false}
            placeholder="!abcdef:matrix.org" dir="ltr" className={`${inputCls} text-start`} />
        </label>
      </div>

      <label className={labelCls}>
        {t("notify.healthchecks")}
        <input value={cfg.healthchecksUrl} onChange={(e) => set("healthchecksUrl", e.target.value)} spellCheck={false}
          placeholder="https://hc-ping.com/your-uuid" className={inputCls} />
      </label>
      {/* notify.healthchecksLifecycle stays permanent text, NOT a bubble —
          see this Card's own header comment above for why (it documents a
          non-obvious cross-setting interaction someone comes back to when
          debugging an unexpected check status, not a one-time explainer). */}
      <p className="text-xs text-carbon-textMuted -mt-1">{t("notify.healthchecksLifecycle")}</p>

      {/* Per-domain Healthchecks overrides (advanced). A blank field falls back to the global URL above. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="flex items-center gap-1 text-xs font-medium text-carbon-textSub">
          {t("notify.hcPerDomain")}
          <InfoBubble tip={t("notify.hcPerDomainHint")} />
        </span>
        {(
          [
            ["container", t("nav.containers")],
            ["VM", t("nav.vms")],
            ["flash", t("nav.flash")],
            ["config", t("nav.config")],
            ["files", t("nav.files")],
          ] as const
        ).map(([key, label]) => (
          <label key={key} className={labelCls}>
            {label}
            <input
              value={cfg.healthchecksByDomain?.[key] ?? ""}
              onChange={(e) =>
                setCfg((c) => ({
                  ...c,
                  healthchecksByDomain: { ...c.healthchecksByDomain, [key]: e.target.value },
                }))
              }
              spellCheck={false}
              placeholder="https://hc-ping.com/your-uuid"
              dir="ltr"
              className={`${inputCls} text-start`}
            />
          </label>
        ))}
      </div>

      {/* Email (SMTP), sent via the configured mail server. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <label className="flex items-start gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={cfg.smtpEnabled}
            onChange={(e) => set("smtpEnabled", e.target.checked)}
            className="mt-0.5"
            style={{ accentColor: "var(--accent)" }}
          />
          <span className="text-sm text-carbon-text">{t("notify.smtp")}</span>
        </label>
        {cfg.smtpEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.smtpHost")}
              <input value={cfg.smtpHost} onChange={(e) => set("smtpHost", e.target.value)} spellCheck={false}
                placeholder="smtp.example.com" dir="ltr" className={`${inputCls} text-start`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPort")}
              <input value={cfg.smtpPort} onChange={(e) => set("smtpPort", Number(e.target.value) || 0)} spellCheck={false}
                type="number" placeholder="587" className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTls")}
              <select value={cfg.smtpTls} onChange={(e) => set("smtpTls", e.target.value)} className={selectCls}>
                <option value="starttls">STARTTLS</option>
                <option value="tls">TLS (implicit)</option>
                <option value="none">None</option>
              </select>
            </label>
            <label className={labelCls}>
              {t("notify.smtpUser")}
              <input value={cfg.smtpUsername} onChange={(e) => set("smtpUsername", e.target.value)} spellCheck={false}
                dir="ltr" className={`${inputCls} text-start`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPass")}
              <RevealInput {...revealSmtpPassword} value={cfg.smtpPassword} onChange={(e) => set("smtpPassword", e.target.value)} spellCheck={false}
                placeholder={secretSet.smtp ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpFrom")}
              <input value={cfg.smtpFrom} onChange={(e) => set("smtpFrom", e.target.value)} spellCheck={false}
                placeholder="bombvault@example.com" dir="ltr" className={`${inputCls} text-start`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTo")}
              <input value={cfg.smtpTo} onChange={(e) => set("smtpTo", e.target.value)} spellCheck={false}
                placeholder="admin@example.com" dir="ltr" className={`${inputCls} text-start`} />
            </label>
          </>
        )}
      </div>
        </>
      )}

      <div className="flex items-center gap-3 pt-1 flex-wrap">
        <button onClick={() => void handleSave()} disabled={state === "saving"}
          className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50">
          {state === "saving" ? t("auth.saving") : t("notify.save")}
        </button>
        <button onClick={() => void handleTest()}
          className="rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors">
          {t("notify.test")}
        </button>
      </div>
    </Card>
  );
}

// ReplicateNowButton triggers an on-demand off-site replication for one domain
// (restic copy local→off-site), surfacing the result inline.
// GlimStone follow-up pass (v8.0.0): both this button's "started"/"failed"
// flash AND TestConnectionButton's ok/uninit/fail verdict below moved to
// toasts — same one-shot completion-notice reasoning as the shared save()
// helper further down, just for the off-site tab's per-domain action buttons
// instead of a settings PUT. Each is a single shared component instantiated
// per domain (containers/vms/flash/files), so this migrates every one of
// those call sites at once, the same leverage as the save() helper.
function ReplicateNowButton({
  domain,
  t,
}: {
  domain: "containers" | "vms" | "flash" | "files";
  t: ReturnType<typeof useT>["t"];
}) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  async function go() {
    setBusy(true);
    try {
      const r = await replicateOffsite(domain);
      if (r.ok) {
        push(t("offsite.replicateStarted"), "success");
      } else {
        push(r.error ?? t("offsite.replicateFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.replicateFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }
  return (
    <button
      type="button"
      onClick={() => void go()}
      disabled={busy}
      className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
    >
      {busy ? t("offsite.replicating") : t("offsite.replicateNow")}
    </button>
  );
}

// TestConnectionButton probes a domain's off-site repo (reachable / initialised)
// without modifying it, showing the verdict inline — so the user can verify the
// configured location before relying on it.
function TestConnectionButton({
  domain,
  t,
}: {
  domain: "containers" | "vms" | "flash" | "files";
  t: ReturnType<typeof useT>["t"];
}) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  // This button probes the PRIMARY target only. Once a domain has more than one
  // off-site copy, say so on the label — an unqualified "Test connection" going
  // green while a second destination was broken is exactly what issue #138
  // reported. Each additional target has its own button in OffsiteTargetsSection.
  const multiTarget = useOffsiteTargets(domain).length > 1;
  async function go() {
    setBusy(true);
    try {
      const r = await testOffsite(domain);
      if (r.ok && r.reachable && r.initialized) {
        push(t("offsite.testOk"), "success");
      } else if (r.ok && r.reachable) {
        push(t("offsite.testUninitialized"), "warn");
      } else {
        push(r.error ?? t("offsite.testFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.testFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }
  return (
    <button
      type="button"
      onClick={() => void go()}
      disabled={busy}
      className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
    >
      {multiTarget ? t("offsite.testPrimary") : t("offsite.test")}
    </button>
  );
}

// IntegrityCard runs per-domain repository maintenance: verify (restic check),
// unlock (clear stale locks), prune (reclaim space), and a restore-verification
// "drill". The drill has two kinds, chosen by the "Drill type" toggle:
//   - "Integrity check" (subset): restic check --read-data-subset on the selected
//     source repo — proves the backup data is intact + restorable.
//   - "Real restore (off-site)" (dr): a REAL sandbox restore of the newest
//     off-site snapshot, then verification + cleanup. All domains except config
//     (config's real recovery path is the in-place staged restart, not a sandbox
//     restore of the settings DB).
// The DR-drill target (which container's/VM's off-site snapshot to restore) binds
// to the shared settings.drDrillTarget / drDrillTargetVm via the parent's
// baseline-merging save().
function IntegrityCard({
  t,
  settings,
  setSettings,
  save,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  hueIndex?: number;
}) {
  // Prune deletes snapshots, so it stays advanced-only even though the rest of
  // this card (verify, unlock, DR drill) is a first-class default-mode feature.
  const { advanced } = useAdvanced();
  const { confirm, confirmDialog } = useConfirm();
  type ActState = "idle" | "busy" | "ok" | "fail";
  type DrillKind = "subset" | "dr";
  const [state, setState] = useState<Record<string, ActState>>({});
  const [msg, setMsg] = useState<Record<string, string>>({});
  const [source, setSource] = useState<RepoSource>("local");
  const [kind, setKind] = useState<DrillKind>("subset");
  // The last recorded drill per domain (for the current source), keyed by domain.
  const [lastDrill, setLastDrill] = useState<Record<string, RestoreDrill | null>>({});
  // Append-only check (#109): the off-site wizard's tamper test, surfaced here
  // under its plainer name because this card is where users look for checks.
  // TamperRes mirrors the wizard's tri-state verdict: not-testable (amber) /
  // protected (green) / delete-accepted (red); lastTamper feeds the idle
  // "append-only protection · Last checked …" caption from /api/status.
  type TamperRes =
    | { kind: "busy" }
    | { kind: "verdict"; testable: boolean; protected: boolean }
    | { kind: "error"; message: string };
  const [tamper, setTamper] = useState<Record<string, TamperRes | undefined>>({});
  const [lastTamper, setLastTamper] = useState<Record<string, { at: number; ok: boolean } | null>>({});
  // Container list feeding the DR-drill target dropdown (kind "dr", containers).
  const [containers, setContainers] = useState<Container[]>([]);
  // VM list feeding the DR-drill target dropdown (kind "dr", VMs).
  const [vms, setVMs] = useState<VM[]>([]);
  // Save state for the drill-target dropdowns (persisted via the parent
  // save()). GlimStone follow-up pass (v8.0.0): save() now pushes a toast on
  // both outcomes instead of setting a "saved"/"error" render state (see
  // save()'s own header comment), so only the setters are needed here —
  // save() still requires them as callback params, but nothing reads the
  // values back anymore.
  const [, setTgtState] = useState<SaveState>("idle");
  const [, setTgtError] = useState<string | null>(null);
  const [, setTgtVMState] = useState<SaveState>("idle");
  const [, setTgtVMError] = useState<string | null>(null);

  type Domain = "containers" | "vms" | "flash" | "files";
  type Action = "verify" | "unlock" | "prune";

  const domains: { key: Domain; label: string }[] = [
    { key: "containers", label: t("settings.containersEnabled") },
    { key: "vms", label: t("settings.vmsEnabled") },
    { key: "flash", label: t("settings.flashEnabled") },
    { key: "files", label: t("settings.filesEnabled") },
  ];

  // Load the containers once for the DR-drill target picker (includes orphans
  // that still have off-site backups, so any drillable target is selectable).
  useEffect(() => {
    let active = true;
    listContainers()
      .then((r) => {
        if (active && r.ok) setContainers(r.containers ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // Load the VMs once for the DR-drill target picker, same reasoning as containers.
  useEffect(() => {
    let active = true;
    listVMs()
      .then((r) => {
        if (active && r.ok) setVMs(r.vms ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // Load the latest drill for each domain on mount and whenever the source
  // changes, so the "last verified" line reflects the selected repo.
  useEffect(() => {
    let active = true;
    for (const { key: domain } of domains) {
      getDrills(domain, source, 1)
        .then((r) => {
          if (!active) return;
          if (r.ok) setLastDrill((m) => ({ ...m, [domain]: r.latest ?? null }));
        })
        .catch(() => undefined);
    }
    return () => {
      active = false;
    };
    // domains is a stable literal list; re-run only when the source changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  // Load each domain's last tamper-test verdict once, so the append-only row's
  // idle caption mirrors the drill row's "last verified" line. The check always
  // probes the OFF-SITE repo, so the source toggle never re-triggers this.
  useEffect(() => {
    let active = true;
    getStatus()
      .then((r) => {
        if (!active || !r.ok || !r.domains) return;
        const m: Record<string, { at: number; ok: boolean } | null> = {};
        for (const d of r.domains) {
          m[d.domain] = d.lastTamperAt > 0 ? { at: d.lastTamperAt, ok: d.lastTamperOK } : null;
        }
        setLastTamper(m);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // runTamperFor proves the domain's off-site repo still refuses deletes — the
  // exact tamper-test API behind the wizard's "Test append-only now" (#109: users
  // searched for it here, and "append-only" is the plainer word for it).
  async function runTamperFor(domain: Domain) {
    setTamper((m) => ({ ...m, [domain]: { kind: "busy" } }));
    try {
      const r = await tamperTest(domain);
      if (r.ok) {
        setTamper((m) => ({
          ...m,
          [domain]: { kind: "verdict", testable: !!r.testable, protected: !!r.protected },
        }));
        // A decisive verdict is also the new "last checked" fact; a not-testable
        // repo records no verdict server-side, so leave the caption untouched.
        if (r.testable) {
          setLastTamper((m) => ({ ...m, [domain]: { at: Math.floor(Date.now() / 1000), ok: !!r.protected } }));
        }
        // The verdict + its run row land in /api/status (scorecard tamper state) —
        // broadcast so the dashboard refetches, mirroring runDrillFor above.
        window.dispatchEvent(new Event("bv:settings-changed"));
      } else {
        setTamper((m) => ({ ...m, [domain]: { kind: "error", message: r.error ?? t("offsite.tamperError") } }));
      }
    } catch (err) {
      setTamper((m) => ({
        ...m,
        [domain]: { kind: "error", message: err instanceof Error ? err.message : t("offsite.tamperError") },
      }));
    }
  }

  async function run(domain: Domain, action: Action) {
    if (action === "prune" && !(await confirm(t("integrity.pruneConfirm")))) return;
    const key = `${domain}:${action}`;
    setState((s) => ({ ...s, [key]: "busy" }));
    setMsg((m) => ({ ...m, [key]: "" }));
    try {
      const r =
        action === "verify" ? await checkDomain(domain, source)
        : action === "unlock" ? await unlockDomain(domain, source)
        : await pruneDomain(domain, source);
      if (r.ok) {
        setState((s) => ({ ...s, [key]: "ok" }));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        setMsg((m) => ({ ...m, [key]: r.error ?? t("integrity.failed") }));
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      setMsg((m) => ({ ...m, [key]: err instanceof Error ? err.message : t("integrity.failed") }));
    }
  }

  // runDrillFor runs a restore-verification drill and records its result inline,
  // mirroring the per-action result-state pattern above (keyed "<domain>:drill").
  // A "dr" drill does a REAL off-site restore into a sandbox — it always targets
  // the off-site repo (source is ignored) and asks for confirmation first.
  async function runDrillFor(domain: Domain) {
    if (kind === "dr" && !(await confirm(t("drill.confirmDR")))) return;
    const key = `${domain}:drill`;
    setState((s) => ({ ...s, [key]: "busy" }));
    setMsg((m) => ({ ...m, [key]: "" }));
    try {
      const r = await runDrill(domain, kind === "dr" ? "offsite" : source, kind);
      if (r.ok && r.drill) {
        const drill = r.drill;
        setLastDrill((m) => ({ ...m, [domain]: drill }));
        setState((s) => ({ ...s, [key]: drill.ok ? "ok" : "fail" }));
        if (!drill.ok) setMsg((m) => ({ ...m, [key]: drill.detail || t("verify.failed") }));
        // A recorded drill (pass OR fail) changes the shared /api/status the
        // dashboard scorecard reads. Broadcast so the Dashboard refetches its
        // drill / "proven restorable" pills without a page reload — mirrors how
        // saving settings signals the app to refresh.
        window.dispatchEvent(new Event("bv:settings-changed"));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        setMsg((m) => ({ ...m, [key]: r.error ?? t("verify.failed") }));
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      setMsg((m) => ({ ...m, [key]: err instanceof Error ? err.message : t("verify.failed") }));
    }
  }

  const actions: { key: Action; label: string; busy: string }[] = [
    { key: "verify", label: t("integrity.verify"), busy: t("integrity.checking") },
    { key: "unlock", label: t("integrity.unlock"), busy: "…" },
    // Prune deletes snapshots — keep it behind Advanced so novices can't reach it.
    ...(advanced ? [{ key: "prune" as Action, label: t("integrity.prune"), busy: "…" }] : []),
  ];

  // Append-only check eligibility: only a domain whose off-site repo is set AND
  // flagged immutable gets the button — the same precondition the wizard's manual
  // test has (anything else could only ever surface a backend error).
  const appendOnlyEligible: Record<Domain, boolean> = {
    containers: settings.containersOffsite !== "" && settings.containersOffsiteImmutable,
    vms: settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable,
    flash: settings.flashOffsite !== "" && settings.flashOffsiteImmutable,
    files: settings.filesOffsite !== "" && settings.filesOffsiteImmutable,
  };

  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus-well";

  return (
    <Card title={t("integrity.title")} hint={t("integrity.hint")} hueIndex={hueIndex}>
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
        <SourceToggle
          source={source}
          onChange={(next) => {
            // The ok/fail indicators belong to the previously selected source —
            // clear them so a "healthy" result doesn't carry over to the other
            // repo where no maintenance has run yet. The drill state + cached
            // last-drill clear here too; the effect above reloads them for `next`.
            setSource(next);
            setState({});
            setMsg({});
            setLastDrill({});
          }}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* Drill-type toggle: subset integrity check vs a real off-site DR
          restore — on the shared Selector component (GlimStone form-engine
          Phase 2, Task 3; found only by re-grepping the current codebase,
          not on the original Phase 1 audit's own 11-site list). */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("drill.kindLabel")}</span>
        <Selector
          items={[
            { id: "subset", label: t("drill.kindSubset") },
            { id: "dr", label: t("drill.kindDR") },
          ]}
          label={t("drill.kindLabel")}
          select="one"
          active={kind}
          onChange={(val) => {
            // Clear any lingering per-domain result so a subset "healthy"
            // doesn't read as a DR pass (or vice versa) after switching kind.
            setKind(val as DrillKind);
            setState({});
            setMsg({});
          }}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* DR-drill controls: an explainer + the container/VM target pickers. Each
          target is a shared setting (settings.drDrillTarget / drDrillTargetVm)
          saved via the parent's baseline-merging save(), so it never clobbers
          other cards' edits. Flash and files have no picker (their whole
          snapshot is restored). */}
      {kind === "dr" && (
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            <span className="flex items-center gap-1">
              {t("drill.target")}
              <InfoBubble tip={t("drill.drNote")} />
            </span>
            <select
              value={settings.drDrillTarget}
              onChange={(e) => {
                const v = e.target.value;
                setSettings((prev) => (prev ? { ...prev, drDrillTarget: v } : prev));
                void save({ drDrillTarget: v }, setTgtState, setTgtError);
              }}
              className={selectCls}
            >
              <option value="">{t("drill.targetMostRecent")}</option>
              {containers.map((c) => (
                <option key={c.name} value={c.name}>{c.name}</option>
              ))}
            </select>
          </label>
          {/* GlimStone follow-up pass (v8.0.0): the "saved"/"error" flash this
              used to render is gone — the shared save() now pushes a toast
              on both outcomes (see its own header comment). */}
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            {t("drill.targetVM")}
            <select
              value={settings.drDrillTargetVm}
              onChange={(e) => {
                const v = e.target.value;
                setSettings((prev) => (prev ? { ...prev, drDrillTargetVm: v } : prev));
                void save({ drDrillTargetVm: v }, setTgtVMState, setTgtVMError);
              }}
              className={selectCls}
            >
              <option value="">{t("drill.targetMostRecent")}</option>
              {vms.map((v) => (
                // value must be the raw libvirt name: pickDRSnapshot (service.go)
                // matches it against the "vm:"+name backup tag, never the
                // display-only friendly name a TrueNAS VM shows here.
                <option key={v.libvirtName} value={v.libvirtName}>{v.name}</option>
              ))}
            </select>
          </label>
        </div>
      )}

      <div className="flex flex-col gap-3">
        {domains.map(({ key: domain, label }) => {
          const dKey = `${domain}:drill`;
          const drill = lastDrill[domain];
          const tRes = tamper[domain];
          const tLast = lastTamper[domain];
          return (
            <div key={domain} className="flex flex-col gap-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm text-carbon-textSub w-24 shrink-0">{label}</span>
                {actions.map((a) => {
                  const k = `${domain}:${a.key}`;
                  return (
                    <span key={a.key} className="inline-flex items-center gap-1">
                      <button
                        onClick={() => void run(domain, a.key)}
                        disabled={state[k] === "busy"}
                        title={t(`integrity.${a.key}Hint`)}
                        className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                      >
                        {state[k] === "busy" ? a.busy : a.label}
                      </button>
                      {state[k] === "ok" && <span className="text-sm text-statusOk">{t("integrity.ok")}</span>}
                    </span>
                  );
                })}
              </div>

              {/* Restore-verification drill: its own row + inline result + last drill.
                  The run button + labels follow the selected drill kind. */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="w-24 shrink-0" />
                <button
                  onClick={() => void runDrillFor(domain)}
                  disabled={state[dKey] === "busy"}
                  title={kind === "dr" ? t("drill.drNote") : t("verify.hint")}
                  className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                >
                  {state[dKey] === "busy"
                    ? kind === "dr" ? t("drill.runningDR") : t("verify.running")
                    : kind === "dr" ? t("drill.runDR") : t("verify.now")}
                </button>
                {state[dKey] === "ok" && <span className="text-sm text-statusOk">✓ {t("verify.ok")}</span>}
                {state[dKey] === "fail" && (
                  <span className="text-sm text-statusFail wrap-break-word">✗ {msg[dKey] || t("verify.failed")}</span>
                )}
                {/* Last recorded drill for this domain/source (idle state only).
                    Names WHICH check ran (off-site DR vs local integrity) and,
                    on a stored failure, the scrubbed reason. */}
                {state[dKey] !== "busy" && state[dKey] !== "ok" && state[dKey] !== "fail" && (
                  drill ? (
                    <>
                      <span className="text-xs text-carbon-textMuted">
                        {isOffsiteSource(drill.source) && drill.kind === "dr"
                          ? t("drill.checkOffsiteDr")
                          : t("drill.checkLocal")}
                        {" · "}
                        {t("verify.last").replace("{time}", relativeTime(t, drill.at))} {drill.ok ? "✓" : "✗"}
                      </span>
                      {!drill.ok && drill.detail && (
                        <span className="text-xs text-statusFail wrap-break-word" title={drill.detail}>
                          {t("drill.failReasonPrefix")} {drill.detail}
                        </span>
                      )}
                    </>
                  ) : (
                    <span className="text-xs text-carbon-textMuted">{t("verify.never")}</span>
                  )
                )}
              </div>

              {/* Append-only check (#109): the wizard's tamper test, findable in
                  this card and led by the plainer name. Immutable off-site domains
                  only; always probes the OFF-SITE repo (source-independent). The
                  verdict rendering mirrors the wizard, incl. the glyph as its own
                  node so RTL locales place it correctly. */}
              {appendOnlyEligible[domain] && (
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="w-24 shrink-0" />
                  <button
                    onClick={() => void runTamperFor(domain)}
                    disabled={tRes?.kind === "busy"}
                    title={t("integrity.appendOnlyHint")}
                    className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                  >
                    {tRes?.kind === "busy" ? t("integrity.checking") : t("integrity.appendOnly")}
                  </button>
                  {tRes?.kind === "verdict" && (
                    <span
                      className={`text-sm wrap-break-word ${
                        !tRes.testable ? "text-statusWarn" : tRes.protected ? "text-statusOk" : "text-statusFail"
                      }`}
                    >
                      {tRes.testable && <span aria-hidden="true">{tRes.protected ? "✓" : "✗"}&nbsp;</span>}
                      {!tRes.testable
                        ? t("offsite.tamperUnverifiable")
                        : tRes.protected
                          ? t("offsite.tamperOk")
                          : t("offsite.tamperFail")}
                    </span>
                  )}
                  {tRes?.kind === "error" && (
                    <span className="text-sm text-statusFail wrap-break-word">{tRes.message}</span>
                  )}
                  {/* Idle caption: the last recorded check, mirroring the drill
                      row's "Last verified …" line. */}
                  {!tRes &&
                    (tLast ? (
                      <span className="text-xs text-carbon-textMuted">
                        {t("integrity.appendOnlyLast").replace("{time}", relativeTime(t, tLast.at))} {tLast.ok ? "✓" : "✗"}
                      </span>
                    ) : (
                      <span className="text-xs text-carbon-textMuted">{t("integrity.appendOnlyNever")}</span>
                    ))}
                </div>
              )}

              {actions.map((a) =>
                state[`${domain}:${a.key}`] === "fail" ? (
                  <span key={a.key} className="text-xs text-statusFail wrap-break-word">
                    {a.label}: {msg[`${domain}:${a.key}`] || t("integrity.failed")}
                  </span>
                ) : null
              )}
            </div>
          );
        })}
      </div>
      {confirmDialog}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Schedule editors — migrated verbatim from the retired Plans page (Jobs.tsx).
// The Schedules tab is now the single owner of every backup/off-site/self-backup/
// restore-check cadence. These render their own Cards (same as on the old Plans
// page); behaviour is unchanged.
// ---------------------------------------------------------------------------

/** Convert a cadence string to a human-readable label. */
function cadenceLabel(raw: string, t: ReturnType<typeof useT>["t"]): string {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return t("jobs.notScheduled");

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return t("jobs.cadenceDaily").replace("{time}", dailyM[1]);

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) return t("jobs.cadenceWeekly").replace("{days}", weeklyM[1]).replace("{time}", weeklyM[2]);

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) return t("jobs.cadenceEveryN").replace("{n}", everyNM[1]).replace("{time}", everyNM[2]);

  return s;
}

type ScheduleStatus = "active" | "paused" | "off";

function scheduleStatus(schedule: string): ScheduleStatus {
  if (!schedule || schedule === "off") return "off";
  return "active";
}

// ScheduleBadge → Badge tone mapping (GlimStone form-engine Task 5 follow-up):
// this was its own hand-rolled `px-2 py-0.5 rounded-control text-xs
// font-medium` + tone-lookup pair, byte-for-byte the same shape the shared
// Badge component now owns — a 6th duplicate the migration's audit found
// alongside the five named in the plan. active/paused/off map onto Badge's
// ok/warn/neutral tones (the only three a schedule status ever needs).
const SCHEDULE_BADGE_TONE: Record<ScheduleStatus, BadgeTone> = {
  active: "ok",
  paused: "warn",
  off: "neutral",
};

function ScheduleBadge({
  status,
  label,
}: {
  status: ScheduleStatus;
  label: string;
}) {
  return <Badge tone={SCHEDULE_BADGE_TONE[status]}>{label}</Badge>;
}

// Domain section — Containers (editable schedule + included-containers list)
function ContainersSection({
  settings,
  containers,
  onChange,
  perItem,
  t,
  hueIndex,
}: {
  settings: Settings;
  containers: Container[];
  onChange: (schedule: string) => void;
  /** #121: when on, each included container exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = settings.containersSchedule;
  const status = scheduleStatus(schedule);
  // Exclude BombVault's own container: it can never be backed up, so it must
  // never appear as a schedule member even if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    <Card title={t("jobs.containersSection")} hint={t("containers.scheduleHint")} hueIndex={hueIndex}>
      {/* Cadence row */}
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>

      {/* Editable cadence builder */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
        />
      </div>

      {/* Member list */}
      {included.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noContainersIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {included.map((c) => (
            <div key={c.name} className="flex flex-col gap-2 py-2 text-sm">
              <div className="flex items-center gap-3">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    c.state.toLowerCase() === "running"
                      ? "bg-statusOkSolid"
                      : "bg-carbon-surface3"
                  }`}
                />
                <span className="font-medium text-carbon-text flex-1 truncate">
                  {c.name}
                </span>
                {c.image && (
                  <span className="text-xs text-carbon-textMuted truncate hidden sm:block max-w-xs">
                    {c.image}
                  </span>
                )}
              </div>
              {perItem && (
                <ItemScheduleOverride
                  name={c.name}
                  initial={c.scheduleCadence ?? ""}
                  onSave={(cadence) => setScheduleCadence(c.name, cadence)}
                />
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// Domain section — VMs (editable schedule)
function VMsSection({
  settings,
  syncSchedules,
  onChange,
  vms,
  perItem,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  /** Included VMs, for the per-item override list (#121). */
  vms: VM[];
  /** #121: when on, each included VM exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const status = scheduleStatus(schedule);
  const included = vms.filter((v) => v.includeInSchedule);

  return (
    <Card title={t("jobs.vmsSection")} hint={t("jobs.vmIncludeHint")} hueIndex={hueIndex}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.vmsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
        />
      </div>

      {/* Per-item overrides (#121): an included-VM list with a per-VM cadence,
          shown only when the toggle is on so the section is otherwise unchanged. */}
      {perItem && (
        included.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("jobs.noVMsIncluded")}</p>
        ) : (
          <div className="flex flex-col gap-1 divide-y divide-carbon-border">
            {included.map((v) => (
              <div key={v.libvirtName} className="flex flex-col gap-2 py-2 text-sm">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-2 h-2 rounded-full shrink-0 ${
                      v.state.toLowerCase() === "running" ? "bg-statusOkSolid" : "bg-carbon-surface3"
                    }`}
                  />
                  <span className="font-medium text-carbon-text flex-1 truncate">{v.name}</span>
                </div>
                <ItemScheduleOverride
                  name={v.name}
                  initial={v.scheduleCadence ?? ""}
                  // libvirtName, not name: PATCH /api/vms/{name} resolves the
                  // path segment against the raw name (see vmNameParam),
                  // never the TrueNAS display-only friendly name.
                  onSave={(cadence) => setVMScheduleCadence(v.libvirtName, cadence)}
                />
              </div>
            ))}
          </div>
        )
      )}
    </Card>
  );
}

// Domain section — Flash (editable schedule)
function FlashSection({
  settings,
  syncSchedules,
  onChange,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.flashSchedule;
  const status = scheduleStatus(schedule);

  return (
    <Card title={t("jobs.flashSection")} hueIndex={hueIndex}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
        />
        {/* GlimStone follow-up pass: stays permanent text, NOT bubbled — a
            behavioural caveat ("this control looks live but silently does
            nothing yet") someone hits while confused about why a saved
            Flash schedule never runs, not a one-time "what does this do"
            explainer. Same carve-out category as notify.healthchecksLifecycle
            above (NotifyCard's own header comment). */}
        {!syncSchedules && (
          <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.flashNotImplemented")}</p>
        )}
      </div>
      <div className="flex items-center gap-3 py-2 text-sm border-t border-carbon-border">
        <div className="w-2 h-2 rounded-full bg-carbon-surface3 shrink-0" />
        <span className="font-medium text-carbon-text flex-1">{t("jobs.flashRow")}</span>
        <span className="text-xs text-carbon-textMuted italic">{t("jobs.flashPlanned")}</span>
      </div>
    </Card>
  );
}

// Domain section — Files (editable schedule + per-set include list). Mirrors
// VMsSection for the cadence and ContainersSection for the member list, except
// the per-set "include in schedule" toggles PATCH each file set directly (the
// same {enabled} flag the Files tab edits) — they are not part of the SaveBar.
function FilesSection({
  settings,
  fileSets,
  onChange,
  onSetsChanged,
  t,
  hueIndex,
}: {
  settings: Settings;
  fileSets: FileSetView[];
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set — reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = settings.filesSchedule;
  const status = scheduleStatus(schedule);
  // Per-set toggle busy state, keyed by set id.
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  // GlimStone follow-up pass (v8.0.0): the persistent (never auto-cleared)
  // error paragraph below is now a toast — a toggle failure is a one-shot
  // completion notice like every other migrated site here.
  async function toggle(set: FileSetView) {
    setBusy((b) => ({ ...b, [set.id]: true }));
    try {
      const res = await patchFileSet(set.id, { enabled: !set.enabled });
      if (res.ok) onSetsChanged();
      else push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [set.id]: false }));
    }
  }

  return (
    <Card title={t("jobs.filesSection")} hint={t("jobs.filesIncludeHint")} hueIndex={hueIndex}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.filesSection")}
          value={schedule}
          onChange={onChange}
        />
      </div>

      {/* Member list — every file set with its live include-in-schedule toggle. */}
      {fileSets.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noFileSetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {fileSets.map((s) => (
            <div key={s.id} className="flex items-center gap-3 py-2 text-sm">
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${
                  s.enabled ? "bg-statusOkSolid" : "bg-carbon-surface3"
                }`}
              />
              <span className="font-medium text-carbon-text flex-1 min-w-0 truncate">{s.name}</span>
              {s.path && (
                <span dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate hidden sm:block max-w-xs text-start">
                  {s.path}
                </span>
              )}
              <Toggle
                hideLabel
                label={`${t("files.enabled")}: ${s.name}`}
                checked={s.enabled}
                onChange={() => void toggle(s)}
                disabled={!!busy[s.id]}
              />
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// Domain section — Restore checks (scheduled restore-verification drills).
// The drill schedule sits beside the backup schedules; always visible.
function RestoreChecksSection({
  settings,
  update,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  return (
    <Card title={t("verify.auto")} hint={t("verify.hint")} hueIndex={hueIndex}>
      <ToggleRow
        hideLabel
        label={t("verify.auto")}
        checked={settings.drillsEnabled}
        onChange={(v) => update({ drillsEnabled: v })}
      />
      {/* Sub-toggle: only meaningful while scheduled drills are on. ToggleRow
          itself dims its switch AND its caption/description together — no
          wrapping container opacity needed here. */}
      <ToggleRow
        label={t("settings.offsiteDrills")}
        hint={t("settings.offsiteDrillsHelp")}
        checked={settings.offsiteDrillsEnabled}
        disabled={!settings.drillsEnabled}
        onChange={(v) => update({ offsiteDrillsEnabled: v })}
      />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("settings.schedule")}
          value={settings.drillsSchedule}
          disabled={!settings.drillsEnabled}
          onChange={(v) => update({ drillsSchedule: v })}
        />
      </div>
      <label className="flex flex-col gap-1 max-w-40">
        <span className="text-xs text-carbon-textSub">{t("verify.subsetPct")}</span>
        <input
          type="number"
          min={1}
          max={100}
          value={settings.drillsSubsetPct}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            const clamped = isNaN(n) ? 1 : Math.min(100, Math.max(1, n));
            update({ drillsSubsetPct: clamped });
          }}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
        />
      </label>
    </Card>
  );
}

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
function IconTabGeneral() {
  // Two stacked switches — the domain on/off toggles this tab actually holds.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" className="shrink-0" aria-hidden="true">
      <rect x="1" y="3" width="10" height="4" rx="2" stroke="currentColor" strokeWidth="1.3" />
      <circle cx="8" cy="5" r="1.15" fill="currentColor" />
      <rect x="5" y="9" width="10" height="4" rx="2" stroke="currentColor" strokeWidth="1.3" />
      <circle cx="8" cy="11" r="1.15" fill="currentColor" />
    </svg>
  );
}

function IconTabStorage() {
  // A drive/disk stack — backup storage paths.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="shrink-0" aria-hidden="true">
      <ellipse cx="8" cy="4" rx="6" ry="2.2" />
      <path d="M2 4v8c0 1.2 2.7 2.2 6 2.2s6-1 6-2.2V4" strokeLinecap="round" />
      <path d="M2 8c0 1.2 2.7 2.2 6 2.2s6-1 6-2.2" strokeLinecap="round" />
    </svg>
  );
}

function IconTabSchedules() {
  // A clock — cadence/timing.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="shrink-0" aria-hidden="true">
      <circle cx="8" cy="8" r="6.2" />
      <path d="M8 4.5V8l3 1.8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function IconTabOffsite() {
  // A cloud — the remote/off-site replica target.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="shrink-0" aria-hidden="true">
      <path d="M4.5 12.5A3 3 0 0 1 4 6.53 3.5 3.5 0 0 1 10.9 5.1 2.75 2.75 0 0 1 12.5 10.4v.1a2.25 2.25 0 0 1-2 2h-6Z" strokeLinejoin="round" />
    </svg>
  );
}

function IconTabNotifications() {
  // A bell — alerts.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="shrink-0" aria-hidden="true">
      <path d="M4 6.5a4 4 0 0 1 8 0c0 3 1 3.8 1 3.8H3s1-.8 1-3.8Z" strokeLinejoin="round" />
      <path d="M6.6 12.5a1.5 1.5 0 0 0 2.8 0" strokeLinecap="round" />
    </svg>
  );
}

function IconTabIntegrity() {
  // A checked shield — repo/backup integrity checks.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="shrink-0" aria-hidden="true">
      <path d="M8 2 3 3.8v3.9c0 3.4 2.3 5.6 5 6.5 2.7-.9 5-3.1 5-6.5V3.8L8 2Z" strokeLinejoin="round" />
      <path d="M5.8 8 7.3 9.5l3-3.1" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function IconTabSystem() {
  // Sliders — system/advanced/SSH knobs. Distinct from IconTabGeneral's
  // rounded toggle switches (a discrete on/off pair) — these are inline
  // continuous sliders, matching Sidebar.tsx's own IconConfig-vs-IconSettings
  // "deliberately distinct so the two never read alike" precedent.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="shrink-0" aria-hidden="true">
      <path d="M2 5h5.5M10 5h4M2 11h2.5M7 11h7" strokeLinecap="round" />
      <circle cx="8.5" cy="5" r="1.4" fill="var(--carbon-surface, transparent)" />
      <circle cx="5" cy="11" r="1.4" fill="var(--carbon-surface, transparent)" />
    </svg>
  );
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

// keepRegistryAuths — the pure filtering logic behind the Image Cleanup &
// Registries card's own save (GlimStone follow-up round, merge A auto-save):
// drops untouched blank rows, and marks a freshly typed token as "stored" so
// the field shows the kept-placeholder once the save lands. Pulled out as a
// standalone, exported function (no React, no `save()` side effect) so it's
// directly unit-testable without mounting SettingsPage — same "extract the
// pure decision, test it without a renderer" shape as isRemotePath
// (PathModeSwitch.tsx) and Selector.tsx's own nextFocusIndex/rovedIndex.
// `auths`/`rowIds` are always the SAME length and index-aligned by
// construction (every mutation site keeps them in lockstep) — the caller
// (saveRegistries below) passes the freshly computed pair rather than
// letting this function read component state directly.
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

export function SettingsPage() {
  const { t } = useT();
  const { advanced } = useAdvanced();
  const { push, quiet, setQuiet } = useToast();

  const [tab, setTab] = useState<TabKey>("general");
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
  // German is not the same width in every one of the 26 shipped locales) and
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
  // savedSettings is the server's last-confirmed state. Each card's Save persists
  // its own fields merged onto THIS baseline (not the live, possibly-edited
  // `settings`), so saving one card never silently commits another card's
  // unsaved edits.
  const [savedSettings, setSavedSettings] = useState<Settings | null>(null);
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
  const [authAuthed, setAuthAuthed] = useState(false);
  const [pwNew, setPwNew] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwSaveState, setPwSaveState] = useState<SaveState>("idle");
  const [pwSaveMsg, setPwSaveMsg] = useState<string | null>(null);
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

  // Rainbow state (GlimStone form-engine Phase 2, Task 1) — synced from/to
  // localStorage via appearance.ts, the same pattern as accentHex above.
  // setRainbow() persists + applies + returns the new (validated) state in
  // one call, so this only ever needs updating from that return value, never
  // a second localStorage read.
  const [rainbow, setRainbowLocal] = useState<RainbowState>(() => getRainbow());
  function updateRainbow(patch: Partial<RainbowState>) {
    setRainbowLocal(setRainbow(patch));
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

  const [pathSaveState, setPathSaveState] = useState<SaveState>("idle");
  const [pathSaveError, setPathSaveError] = useState<string | null>(null);
  const [, setExportEncSaveState] = useState<SaveState>("idle");
  const [, setExportEncSaveError] = useState<string | null>(null);
  const [offsiteSaveState, setOffsiteSaveState] = useState<SaveState>("idle");
  const [offsiteSaveError, setOffsiteSaveError] = useState<string | null>(null);
  // Which domain's guided off-site setup wizard is expanded (null = none).
  const [offsiteWizard, setOffsiteWizard] = useState<"containers" | "vms" | "flash" | "files" | null>(null);

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
  type DomainToggleKey =
    | "containersEnabled"
    | "vmsEnabled"
    | "flashEnabled"
    | "filesEnabled"
    | "configEnabled"
    | "receiverEnabled"
    | "fleetEnabled";
  const [domainToggleBusy, setDomainToggleBusy] = useState<Partial<Record<DomainToggleKey, boolean>>>({});
  const [domainToggleShake, setDomainToggleShake] = useState<Partial<Record<DomainToggleKey, number>>>({});

  const [retSaveState, setRetSaveState] = useState<SaveState>("idle");
  const [retSaveError, setRetSaveError] = useState<string | null>(null);

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

  const [cacheSaveState, setCacheSaveState] = useState<SaveState>("idle");
  const [cacheSaveError, setCacheSaveError] = useState<string | null>(null);

  const [offRetSaveState, setOffRetSaveState] = useState<SaveState>("idle");
  const [offRetSaveError, setOffRetSaveError] = useState<string | null>(null);

  const [limSaveState, setLimSaveState] = useState<SaveState>("idle");
  const [limSaveError, setLimSaveError] = useState<string | null>(null);

  const [metricsSaveState, setMetricsSaveState] = useState<SaveState>("idle");
  const [metricsSaveError, setMetricsSaveError] = useState<string | null>(null);

  // Weekly digest (notifications tab) — its own save state, persisted via the
  // shared baseline-merging save().
  const [digestSaveState, setDigestSaveState] = useState<SaveState>("idle");
  const [digestSaveError, setDigestSaveError] = useState<string | null>(null);

  // Overdue-backup watchdog (notifications tab) — its own save state, same
  // baseline-merging save() as the digest card above it.
  const [watchdogSaveState, setWatchdogSaveState] = useState<SaveState>("idle");
  const [watchdogSaveError, setWatchdogSaveError] = useState<string | null>(null);

  // Schedules tab (migrated from the retired Plans page). The container list
  // feeds the Containers schedule section's included-members list; syncSchedules
  // applies the Containers cadence to VMs + Flash; schedSave* drives its SaveBar.
  const [containers, setContainers] = useState<Container[]>([]);
  // VMs feed the VMs schedule section's per-item override list (#121).
  const [vms, setVMs] = useState<VM[]>([]);
  // File sets feed the Files schedule section's member list (live enabled toggles).
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);
  const [syncSchedules, setSyncSchedules] = useState(false);
  const [schedSaveState, setSchedSaveState] = useState<SaveState>("idle");
  const [schedSaveError, setSchedSaveError] = useState<string | null>(null);
  // Health-gated ordered restart (#119) — its own save state, same
  // baseline-merging save() as the other cards on this tab.
  const [restartSaveState, setRestartSaveState] = useState<SaveState>("idle");
  const [restartSaveError, setRestartSaveError] = useState<string | null>(null);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          setSavedSettings(res.settings);
          // Give every loaded registry row a stable client-only id (see
          // registryRowIds' declaration above) — a fresh GET never carries
          // one of its own, so one is minted here, once, per row. randomId()
          // rather than crypto.randomUUID(): the latter is secure-context-only
          // and would throw on BombVault's documented plain-HTTP origin, and a
          // throw HERE lands in this promise's .catch below — killing the whole
          // Settings page, not just this card (see lib/uuid.ts).
          setRegistryRowIds(res.settings.registryAuths.map(() => randomId()));
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
          if (res.platform) setPlatformKind(res.platform);
          // Detect whether the domain schedules are already in sync (Containers ==
          // VMs == Flash, and not off), so the Schedules tab's sync checkbox
          // reflects it on load. Reproduced from the retired Plans page.
          const s = res.settings;
          if (
            s.vmsSchedule === s.containersSchedule &&
            s.flashSchedule === s.containersSchedule &&
            s.containersSchedule !== "off" &&
            s.containersSchedule !== ""
          ) {
            setSyncSchedules(true);
          }
        } else {
          setLoadError("Failed to load settings");
        }
      })
      .catch(() => setLoadError("Failed to load settings"));

    // Load auth status for the Security card.
    getAuth()
      .then((res) => {
        setAuthEnabled(res.enabled);
        setAuthAuthed(res.authed);
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
    const tabs: TabKey[] = [
      "general",
      "storage",
      "schedules",
      "offsite",
      "notifications",
      "integrity",
      "system",
    ];
    const applyHash = () => {
      const h = window.location.hash.replace(/^#/, "");
      if ((tabs as string[]).includes(h)) setTab(h as TabKey);
    };
    applyHash();
    window.addEventListener("hashchange", applyHash);
    return () => window.removeEventListener("hashchange", applyHash);
  }, []);

  // While "sync" is on, mirror the Containers cadence onto VMs + Flash in live
  // state (not just in the save patch), so unchecking sync doesn't snap the
  // VM/Flash editors back to stale pre-sync values. The equality guard stops
  // re-renders from looping. Reproduced verbatim from the retired Plans page.
  useEffect(() => {
    if (!syncSchedules) return;
    setSettings((prev) => {
      if (!prev) return prev;
      if (
        prev.vmsSchedule === prev.containersSchedule &&
        prev.flashSchedule === prev.containersSchedule
      ) {
        return prev;
      }
      return { ...prev, vmsSchedule: prev.containersSchedule, flashSchedule: prev.containersSchedule };
    });
  }, [syncSchedules, settings?.containersSchedule]);

  // ---------------------------------------------------------------------------
  // Generic save helper
  // ---------------------------------------------------------------------------

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
  async function save(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ): Promise<boolean> {
    const base = savedSettings ?? settings;
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
        setSavedSettings(updated);
        setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
        setSaveState("idle");
        // Tell the Layout/Sidebar to refetch so a newly enabled/disabled domain
        // tab appears or vanishes immediately — no page reload needed.
        window.dispatchEvent(new Event("bv:settings-changed"));
        push(t("settings.saved"), "success");
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
  type MergedAutoSaveKey =
    | "pruneImageAfterUpdate"
    | "reconcileUnraidUpdateStatus"
    | "exportEncryptEnabled"
    | "encryptionEnabled";
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
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  const DEBOUNCE_MS = 800;

  function debouncedSave(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, DEBOUNCE_MS);
  }

  function cancelDebounce(key: string) {
    const existing = debounceTimers.current[key];
    if (existing) {
      clearTimeout(existing);
      delete debounceTimers.current[key];
    }
  }

  // A pending debounce must not fire (and call setSettings/save with stale
  // closures) after this page has unmounted — e.g. the user types into the
  // registry host field and navigates away within the 800ms window. The
  // ref's own object identity never changes (only its properties are
  // mutated in place by debouncedSave/cancelDebounce above), but it's still
  // captured into a local first — the plain, lint-satisfying version of the
  // same "don't read .current fresh inside a cleanup" rule, even though this
  // ref never actually goes stale the way a DOM-node ref could.
  useEffect(() => {
    const timers = debounceTimers.current;
    return () => {
      Object.values(timers).forEach(clearTimeout);
    };
  }, []);

  // saveRegistries — the merge A registries sub-section's own save, shared by
  // both the debounced per-field edit path and the immediate Remove-row path
  // (see the Card below). Reproduces the exact "drop untouched blank rows,
  // mark a freshly typed token as stored" logic the old batched Speichern
  // button used to run on click, now against an EXPLICIT (auths, rowIds)
  // pair rather than reading `settings`/`registryRowIds` directly — both
  // callers already have the freshly computed arrays in hand (the state
  // update and this save race the same render otherwise), so passing them
  // in avoids acting on a one-render-stale snapshot.
  function saveRegistries(nextAuths: RegistryAuthEntry[], nextRowIds: string[]) {
    const { auths, rowIds } = keepRegistryAuths(nextAuths, nextRowIds);
    void save(
      { registryAuths: auths },
      setRegistrySaveState,
      setRegistrySaveError
    ).then((ok) => {
      if (ok) setRegistryRowIds(rowIds);
    });
  }

  // buildSchedulePatch collects EVERY schedule field for the Schedules tab's one
  // SaveBar, applying Jobs' exact sync semantics: Containers always; VMs + Flash
  // mirror Containers when synced, else their own value. Persisted via save(),
  // which merges onto the savedSettings baseline (never clobbering other tabs).
  function buildSchedulePatch(): Partial<Settings> {
    if (!settings) return {};
    const patch: Partial<Settings> = {
      containersSchedule: settings.containersSchedule,
    };
    if (syncSchedules) {
      patch.vmsSchedule = settings.containersSchedule;
      patch.flashSchedule = settings.containersSchedule;
    } else {
      patch.vmsSchedule = settings.vmsSchedule;
      patch.flashSchedule = settings.flashSchedule;
    }
    // Files cadence — independent of the sync checkbox (it covers VMs + Flash).
    patch.filesSchedule = settings.filesSchedule;
    // Restore-check (drill) schedule.
    patch.drillsEnabled = settings.drillsEnabled;
    patch.offsiteDrillsEnabled = settings.offsiteDrillsEnabled;
    patch.drillsSchedule = settings.drillsSchedule;
    patch.drillsSubsetPct = settings.drillsSubsetPct;
    // Off-site replication cadences (+ config + files) — sole owner is this tab.
    patch.containersOffsiteSchedule = settings.containersOffsiteSchedule;
    patch.vmsOffsiteSchedule = settings.vmsOffsiteSchedule;
    patch.flashOffsiteSchedule = settings.flashOffsiteSchedule;
    patch.configOffsiteSchedule = settings.configOffsiteSchedule;
    patch.filesOffsiteSchedule = settings.filesOffsiteSchedule;
    // Self-backup cadence + scheduled off-site tamper test.
    patch.configSchedule = settings.configSchedule;
    patch.tamperTestSchedule = settings.tamperTestSchedule;
    // Anacron-style catch-up toggle (Missed schedules card on this tab).
    patch.catchUpMissed = settings.catchUpMissed;
    // Per-item schedules opt-in (#121).
    patch.perItemSchedules = settings.perItemSchedules;
    return patch;
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
  async function handleSetPassword() {
    if (pwNew !== pwConfirm) {
      setPwSaveMsg(t("auth.passwordMismatch"));
      setPwSaveState("error");
      return;
    }
    setPwSaveState("saving");
    setPwSaveMsg(null);
    try {
      const res = await setAuthPassword(pwNew);
      if (res.ok) {
        setAuthEnabled(res.enabled ?? false);
        setPwSaveState("idle");
        push(pwNew === "" ? t("auth.passwordCleared") : t("auth.passwordSaved"), "success");
        setPwNew("");
        setPwConfirm("");
      } else {
        setPwSaveState("idle");
        push(res.error ?? t("auth.saveError"), "fail");
      }
    } catch {
      setPwSaveState("idle");
      push(t("auth.saveError"), "fail");
    }
  }

  async function handleLogout() {
    await logout().catch(() => undefined);
    // Reload so the auth gate re-checks and shows the login screen.
    window.location.reload();
  }

  async function handleLogoutAll() {
    // Rotates the server-side session epoch, revoking EVERY outstanding session
    // cookie (all browsers/devices) — not just clearing this one.
    await logoutAll().catch(() => undefined);
    // Reload so the auth gate re-checks and shows the login screen. Reached via
    // globalThis (cf. downloadRecoveryKit in api.ts): runtime-identical to bare
    // window, but immune to the broken DOM lib resolution.
    const g = globalThis as unknown as { location: { reload(): void } };
    g.location.reload();
  }

  // Tamper-test schedule eligibility (#109): mirrors immutableOffsiteDomains in
  // internal/schedule/schedule.go — the scheduler only wires the scheduled
  // tamper-test job when at least one domain's off-site repo is set AND
  // flagged immutable. Without that, the cadence editor below silently never
  // runs (the same per-domain predicate as appendOnlyEligible in IntegrityCard,
  // widened to "any domain including config").
  const tamperScheduleActive =
    (settings.containersOffsite !== "" && settings.containersOffsiteImmutable) ||
    (settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable) ||
    (settings.flashOffsite !== "" && settings.flashOffsiteImmutable) ||
    (settings.configOffsite !== "" && settings.configOffsiteImmutable) ||
    (settings.filesOffsite !== "" && settings.filesOffsiteImmutable);

  // hueSeq/nextHue (GlimStone follow-up pass, jdp's second live-review round
  // — "Die ganzen... Abschnittsbadges sind nicht in der Farbengine!!"):
  // every Card/section-title notch below now takes an optional `hueIndex`,
  // by its own LIST INDEX among the notches visible on the CURRENTLY ACTIVE
  // tab (see Card's own doc comment and Badge.tsx's tone="heading" section
  // for the full history/reasoning). A hand-counted literal per tab would
  // silently drift the moment a Card is added/removed/reordered — this
  // plain, freshly-reset-every-render counter instead assigns 0,1,2,... in
  // exactly the order the JSX below actually evaluates each call, which for
  // a `{tab === "x" && (<Card hueIndex={nextHue()} .../>)}` gate is ALSO
  // exactly the order those Cards are, or would be, painted: `&&`
  // short-circuits, so an inactive tab's own `nextHue()` calls never run at
  // all (the counter is never incremented for cards that aren't on screen),
  // and a re-render always starts back at 0 (a plain local `let`, not a
  // ref/state — nothing here needs to survive between renders). One
  // exception, documented at its own two call sites below: the four
  // Domain-schedule sections (ContainersSection/VMsSection/FlashSection/
  // FilesSection) and RestoreChecksSection are each passed their OWN
  // `nextHue()` result the same way every inline `<Card>` is — the counter
  // does not care whether the notch it is numbering renders directly here or
  // one function-call away inside a child component.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    // gap-10 (live-review round — "gap between the tab strip and the first
    // card is too small"): was gap-6 (24px), same value the tab-panels
    // wrapper further down used to use for the SAME job before its own
    // gap-10 bump (see that wrapper's own comment). This outer wrapper now
    // has exactly two children — the heading+tab-strip block immediately
    // below, and the tab-panels wrapper — so bumping ITS gap to gap-10 is
    // what actually widens the space between the tab strip and the first
    // Card's top edge to the same 40px rhythm every Card-to-Card gap already
    // uses, without touching the (unrelated, still gap-6) space between the
    // heading and the tab strip itself.
    <div className="flex flex-col gap-10">
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

      {/* ------------------------------------------------------------------ */}
      {/* Tab strip (7 tabs), on the shared Selector component (GlimStone     */}
      {/* form-engine Phase 2, Task 3 — design-language's "top, with an       */}
      {/* icon" rule for a settings-style tab row). `tab` is the single owner */}
      {/* of which card group renders. Each tab still owns a rainbow         */}
      {/* position by its LIST INDEX (never a hash of `key`) — Selector's     */}
      {/* default hue=true carries over exactly the rainbow wiring this strip */}
      {/* had before the migration (see Task 2's own audit comment, now      */}
      {/* removed from here since Selector owns the useRainbow() subscription */}
      {/* itself). Icons are new: the pre-migration hand-rolled strip had     */}
      {/* none — TAB_ICON above is this task's own addition, satisfying the   */}
      {/* "no icon beats the wrong one" rule with a per-section glyph rather  */}
      {/* than a placeholder.                                                */}
      {/*                                                                    */}
      {/* GlimStone follow-up pass, live-review point 7 (real bug, not a     */}
      {/* Selector defect): this strip used to live INSIDE the same           */}
      {/* `max-w-3xl` reading column as the form content below, which capped  */}
      {/* it to 768px — narrower than the ~814px seven icon+label "lg"        */}
      {/* segments need in German (the longest-label locale), so it wrapped   */}
      {/* to two lines (a lone "System" tab stranded on row 2) even on a wide  */}
      {/* desktop window. Selector's own "wraps, it never scrolls" rule       */}
      {/* (design-language.md, "The one horizontal selector") is working      */}
      {/* exactly as designed — the bug was Settings.tsx capping its PRIMARY   */}
      {/* NAVIGATION to the same narrow column as its prose/form content, the  */}
      {/* one page in the app that did (every sibling page's own <h1> above    */}
      {/* renders un-capped too). Moving `max-w-3xl` down onto the tab panels  */}
      {/* wrapper below (and off this strip + the heading above) restores a    */}
      {/* one-line fit at any normal desktop width without touching Selector   */}
      {/* itself, so its wrap-not-scroll fallback still protects every OTHER   */}
      {/* call site (and this one too, on a genuinely narrow viewport) exactly */}
      {/* as before.                                                          */}
      {/*                                                                    */}
      {/* `plain` dropped (live-review point 8): the ten toolbar-chip call    */}
      {/* sites across Containers/VMs/Files/CadenceBuilder give an unselected  */}
      {/* segment a visible bg-carbon-surface2 idle fill, so the WHOLE strip   */}
      {/* reads as a row of badges with one filled/active — `plain` (this     */}
      {/* strip's pre-migration look, preserved verbatim by Task 3 on purpose) */}
      {/* instead rendered unselected tabs as bare text with no badge shape at */}
      {/* all. Matching the dominant convention instead of the one other      */}
      {/* `plain` call site (Dashboard's heatmap toggle, left untouched — out  */}
      {/* of THIS strip's scope) is a deliberate, requested style change, not  */}
      {/* a migration-fidelity slip.                                          */}
      {/*                                                                    */}
      {/* `equalWidth` (GlimStone follow-up pass, live-review round — "make    */}
      {/* the tab strip's badges all equal width, then size the cards to      */}
      {/* match that row"): each of the 7 tabs used to hug its own label       */}
      {/* width ("Allgemein" narrower than "Pfade & Speicher"), so the wrapped */}
      {/* row left a stretch of bare gap after the last tab ("System") even    */}
      {/* though the Card below (its own width cap already removed, see that  */}
      {/* wrapper's own comment below) renders edge-to-edge across the same    */}
      {/* container — a container-width match that still LOOKED mismatched     */}
      {/* because the visible pills never filled it.                          */}
      {/*                                                                    */}
      {/* CORRECTED (jdp, round 2, explicit): a full-row `flex-1` stretch was   */}
      {/* the wrong fix — "Ich wollte die Tab-Badges nur so breit wie sie       */}
      {/* breit sein müssen... alle so breit wie der Benachrichtigungen-        */}
      {/* Badge." Selector's `equalWidth` now measures the widest label's own   */}
      {/* content width and pins every segment to THAT fixed width instead      */}
      {/* (see Selector.tsx's own file header, item 5b, for the full            */}
      {/* corrected mechanism) — the row hugs its own content again, just       */}
      {/* with 7 equal segments instead of 7 ragged ones. That makes this       */}
      {/* strip narrower than the page's full width once more, which is why     */}
      {/* it's now wrapped in its own measured container below (`tabStripEl`/   */}
      {/* `tabStripWidth`, this component's own state block): the Card panels   */}
      {/* wrapper further down reads that SAME measured width back as its own   */}
      {/* max-width, instead of the old "both happen to be full-width, so they  */}
      {/* match automatically" assumption — see that wrapper's own comment for  */}
      {/* why that assumption no longer holds.                                 */}
      {/*                                                                    */}
      {/* `title: label` (equalWidth follow-up): equal-width segments trade    */}
      {/* away content-hugging, so the single longest label at a given         */}
      {/* viewport ("Benachrichtigungen" in German, verified live at 1400px)  */}
      {/* can now truncate where it never did before — the label span's own    */}
      {/* `truncate` class (Selector.tsx) already handles the ellipsis, but    */}
      {/* nothing previously surfaced the untruncated text anywhere, because   */}
      {/* no pre-migration "chip" segment ever needed to (each was always      */}
      {/* exactly as wide as its own content). A native title tooltip is the   */}
      {/* same low-cost fallback Files.tsx's destChip already uses for its own */}
      {/* disabled-hint case — cheap insurance for the one truncation case     */}
      {/* this specific change can newly introduce, at any label length in     */}
      {/* any of the 26 locales, not just the one word measured live today.    */}
      {/* ------------------------------------------------------------------ */}
      {/* self-start (verified live — first pass shipped WITHOUT this and      */}
      {/* silently under-measured): this wrapper's own parent is itself a      */}
      {/* `flex flex-col` column (the gap-6 heading+strip group above), so a   */}
      {/* child here is a genuine FLEX ITEM regardless of what display value   */}
      {/* the child itself specifies — flex items are always "blockified"      */}
      {/* (the CSS Display spec forces a flex child's used display to a        */}
      {/* block-outside value, `inline-flex` included), and the column's own   */}
      {/* default `align-items: stretch` then stretches that blockified item's */}
      {/* CROSS axis (width, since the column's main axis is vertical) to the  */}
      {/* line's full width regardless of content. A plain `inline-flex`       */}
      {/* class alone does NOT opt out of that — it only changes what would    */}
      {/* happen in a normal block-flow parent, which this isn't. `self-start` */}
      {/* is the actual escape hatch (align-self overriding the inherited      */}
      {/* stretch), letting this item's width resolve via ordinary shrink-to-  */}
      {/* fit sizing instead — confirmed live: without it, `tabStripRef`       */}
      {/* measured the STRETCHED (column-width, ~1113px) box while the         */}
      {/* Selector strip inside it kept rendering at its own real, narrower    */}
      {/* content width (~1243px in German at 1400px viewport) and simply      */}
      {/* overflowed the stretched wrapper — capping the Card panels below to  */}
      {/* the WRONG, too-narrow number. `max-w-full` still guards the opposite */}
      {/* edge (a genuinely narrow viewport), and `inline-flex` is kept        */}
      {/* alongside `self-start` for correctness if this wrapper is ever moved */}
      {/* under a non-flex (normal block-flow) parent instead, where the       */}
      {/* inline-level box model would matter again. */}
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
        select="one"
        active={tab}
        onChange={(key) => {
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
      <div className="flex flex-col gap-10" style={{ maxWidth: tabStripWidth ?? undefined }}>

      {/* ------------------------------------------------------------------ */}
      {/* SCHEDULES — the single owner of every cadence (migrated from Plans).  */}
      {/* Backup schedules reuse the proven per-domain sections + sync checkbox; */}
      {/* off-site / self-backup / restore-check cadences are edited here too.   */}
      {/* One SaveBar persists them all via the shared baseline-merging save().  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "schedules" && (
        <>
          {/* Backup schedules (schedulesBackup): Containers + sync + VMs + Flash.
              A group heading (Card-title style) labels the three domain cards,
              matching the single-Card off-site / self-backup / checks groups.
              Task 5 (rule 11): same Badge treatment as Card's own <h2> above,
              since this IS a Card-title-equivalent heading, just labelling
              three sibling Cards instead of sitting inside one.
              GlimStone follow-up pass ("half-overlap card notch"): `relative`
              added directly on this <h2> (no wrapping div otherwise exists
              here) — there's no padding between the heading and the edge it
              straddles, so the h2 itself is the right anchor; see
              Badge.tsx's badgeClassName comment for the positioning math. */}
          <h2 className="relative flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>{t("settings.schedulesBackup")}</Badge>
          </h2>
          {/* Per-item schedules toggle (#121): opt in to per-container/VM overrides.
              Off by default — while off, the member lists below are unchanged. */}
          <label className="flex items-start gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={settings.perItemSchedules}
              onChange={(e) =>
                setSettings((prev) => (prev ? { ...prev, perItemSchedules: e.target.checked } : prev))
              }
              className="mt-0.5 h-4 w-4 rounded-control border-carbon-border bg-carbon-surface2 accent-(--accent)"
            />
            <span className="flex items-center gap-1 text-sm text-carbon-text">
              {t("settings.perItemSchedules")}
              <InfoBubble tip={t("settings.perItemSchedulesHint")} />
            </span>
          </label>
          <ContainersSection
            settings={settings}
            containers={containers}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, containersSchedule: v } : prev))
            }
            perItem={settings.perItemSchedules}
            t={t}
            hueIndex={nextHue()}
          />
          {/* Sync checkbox — applies the Containers cadence to VMs + Flash too. */}
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={syncSchedules}
              onChange={(e) => setSyncSchedules(e.target.checked)}
              className="h-4 w-4 rounded-control border-carbon-border bg-carbon-surface2 accent-(--accent)"
            />
            <span className="text-sm text-carbon-text">{t("jobs.syncSchedules")}</span>
          </label>
          <VMsSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, vmsSchedule: v } : prev))
            }
            vms={vms}
            perItem={settings.perItemSchedules}
            t={t}
            hueIndex={nextHue()}
          />
          <FlashSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, flashSchedule: v } : prev))
            }
            t={t}
            hueIndex={nextHue()}
          />
          <FilesSection
            settings={settings}
            fileSets={fileSets}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, filesSchedule: v } : prev))
            }
            onSetsChanged={loadFileSets}
            t={t}
            hueIndex={nextHue()}
          />

          {/* Off-site replication schedules (schedulesOffsite): one cadence per
              domain (+ config + files). Editors here are the sole owner of these
              fields. */}
          <Card title={t("settings.schedulesOffsite")} hueIndex={nextHue()}>
            {([
              ["containersOffsiteSchedule", "nav.containers"],
              ["vmsOffsiteSchedule", "nav.vms"],
              ["flashOffsiteSchedule", "nav.flash"],
              ["configOffsiteSchedule", "nav.config"],
              ["filesOffsiteSchedule", "nav.files"],
            ] as const).map(([key, label]) => (
              <div key={key} className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">{t(label)}</span>
                <input
                  value={settings[key]}
                  spellCheck={false}
                  onChange={(e) =>
                    setSettings((prev) => (prev ? { ...prev, [key]: e.target.value } : prev))
                  }
                  placeholder={t("offsite.schedulePlaceholder")}
                  dir="ltr"
                  className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus text-start"
                />
              </div>
            ))}
          </Card>

          {/* Self-backup schedule (schedulesSelfBackup): BombVault's own config. */}
          <Card title={t("settings.schedulesSelfBackup")} hint={t("config.scheduleHint")} hueIndex={nextHue()}>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("nav.config")}</span>
              <input
                value={settings.configSchedule}
                spellCheck={false}
                onChange={(e) =>
                  setSettings((prev) => (prev ? { ...prev, configSchedule: e.target.value } : prev))
                }
                placeholder={t("config.schedulePlaceholder")}
                dir="ltr"
                className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus text-start"
              />
            </div>
          </Card>

          {/* Restore-check drills (RestoreChecksSection renders its own Card). */}
          <RestoreChecksSection
            settings={settings}
            update={(patch) =>
              setSettings((prev) => (prev ? { ...prev, ...patch } : prev))
            }
            t={t}
            hueIndex={nextHue()}
          />

          {/* Missed schedules: anacron-style catch-up after start. Backend runs
              the missed domain job ~2 minutes after boot (see internal/schedule
              CatchUpMissed). */}
          <Card title={t("settings.missedSchedulesTitle")} hueIndex={nextHue()}>
            <ToggleRow
              label={t("settings.catchUpMissed")}
              hint={t("settings.catchUpMissedHint")}
              checked={settings.catchUpMissed}
              onChange={(v) =>
                setSettings((prev) => (prev ? { ...prev, catchUpMissed: v } : prev))
              }
            />
          </Card>

          {/* Health-gated ordered restart (#119): after a backup that stopped
              other containers ("Stop other containers during backup"), they
              restart in compose depends_on order and each must report
              healthy/running before its dependents start. The wait also holds
              through the post-backup update recreate (see internal/backup
              orchestrator WhileDependentsStopped). */}
          <Card title={t("settings.restartHealthTitle")} hueIndex={nextHue()}>
            <ToggleRow
              label={t("settings.restartHealthWait")}
              hint={t("settings.restartHealthWaitHint")}
              checked={settings.restartHealthWait}
              onChange={(v) =>
                setSettings((prev) => (prev ? { ...prev, restartHealthWait: v } : prev))
              }
            />
            {settings.restartHealthWait && (
              <label className="flex flex-col gap-1 sm:w-1/2">
                {/* Live-review round 3 sweep: the range explainer used to sit
                    as a permanent caption below the field. Moved beside the
                    field's own label as an InfoBubble, the exact pattern the
                    retention grid further down already uses for a field-
                    level "what does this number mean" note. */}
                <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("settings.restartHealthTimeoutLabel")}
                  <InfoBubble tip={t("settings.restartHealthTimeoutHint")} />
                </span>
                <input
                  type="number"
                  min={5}
                  max={3600}
                  value={settings.restartHealthTimeoutSec}
                  onChange={(e) => {
                    const raw = (e.target as unknown as { value: string }).value;
                    // Clamp to the field minimum (5): never let a transient sub-5
                    // value sit in component state. The server clamps to 5..3600.
                    const n = Math.max(5, parseInt(raw, 10) || 0);
                    setSettings((prev) =>
                      prev ? { ...prev, restartHealthTimeoutSec: n } : prev
                    );
                  }}
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
                />
              </label>
            )}
            <SaveBar
              state={restartSaveState}
              error={restartSaveError}
              onSave={() =>
                void save(
                  {
                    restartHealthWait: settings.restartHealthWait,
                    restartHealthTimeoutSec: settings.restartHealthTimeoutSec,
                  },
                  setRestartSaveState,
                  setRestartSaveError
                )
              }
              t={t}
            />
          </Card>

          {/* Restore-check schedule (schedulesChecks): the scheduled off-site
              append-only tamper test. Previously had no UI editor at all. */}
          <Card title={t("settings.schedulesChecks")} hueIndex={nextHue()}>
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.tamperTestSchedule")}
                value={settings.tamperTestSchedule}
                onChange={(v) =>
                  setSettings((prev) => (prev ? { ...prev, tamperTestSchedule: v } : prev))
                }
              />
              {/* #109: the scheduler stays inert without a qualifying domain — this
                  is the only place that told manilx why Sun 08:00 never ran. */}
              {!tamperScheduleActive && (
                <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                  {t("settings.tamperScheduleInactive")}
                </div>
              )}
            </div>
          </Card>

          {/* One Save persists every schedule field via the shared save(). */}
          <SaveBar
            state={schedSaveState}
            error={schedSaveError}
            onSave={() => void save(buildSchedulePatch(), setSchedSaveState, setSchedSaveError)}
            t={t}
          />
        </>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL — Domains                                                   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "general" && (
      <Card title={t("settings.domains")} hint={t("settings.domainsHint")} hueIndex={nextHue()}>
        {/* Live-review round 3, point 4: all 7 rows here used to show a
            permanent visible caption under the label (rule 8 violation — the
            first 5 were even raw hardcoded English strings, never localized
            at all). Every row now carries its explanation via ToggleRow's
            `hint` prop (an InfoBubble beside the label) instead — receiver/
            fleet already had a real i18n key for their caption and just
            needed the prop swapped; containers/vms/flash/files/config
            needed a NEW *Hint key added (and translated into all 26
            locales) since their old text was never a translation key.

            #142 (jdp, live review): "Bei Domänen der Speichern-Button
            entfernen, es soll automatisch speichern und den Tab live
            einblenden/ausblenden" — no more batched SaveBar. Each row now
            calls toggleDomainEnabled directly: optimistic flip, persist via
            the shared save() (which already broadcasts
            "bv:settings-changed" so Layout/Sidebar re-fetch and the domain's
            nav tab appears/disappears live, no reload), and on a rejected
            save — e.g. enabling VMs with no working SSH connection to the
            libvirt host — revert to the pre-click state and shake. `disabled`
            covers this row's own request still being in flight
            (domainToggleBusy), so a user can't fire a second click at the
            same toggle before the first one resolves. */}
        <ToggleRow
          label={t("settings.containersEnabled")}
          hint={t("settings.containersEnabledHint")}
          checked={settings.containersEnabled}
          onChange={(v) => void toggleDomainEnabled("containersEnabled", v)}
          disabled={domainToggleBusy.containersEnabled}
          shakeNonce={domainToggleShake.containersEnabled}
          hueIndex={0}
        />
        <ToggleRow
          label={t("settings.vmsEnabled")}
          hint={t("settings.vmsEnabledHint")}
          checked={settings.vmsEnabled}
          onChange={(v) => void toggleDomainEnabled("vmsEnabled", v)}
          disabled={domainToggleBusy.vmsEnabled}
          shakeNonce={domainToggleShake.vmsEnabled}
          hueIndex={1}
        />
        <ToggleRow
          label={t("settings.flashEnabled")}
          hint={t("settings.flashEnabledHint")}
          checked={settings.flashEnabled}
          onChange={(v) => void toggleDomainEnabled("flashEnabled", v)}
          disabled={domainToggleBusy.flashEnabled}
          shakeNonce={domainToggleShake.flashEnabled}
          hueIndex={2}
        />
        <ToggleRow
          label={t("settings.filesEnabled")}
          hint={t("settings.filesEnabledHint")}
          checked={settings.filesEnabled}
          onChange={(v) => void toggleDomainEnabled("filesEnabled", v)}
          disabled={domainToggleBusy.filesEnabled}
          shakeNonce={domainToggleShake.filesEnabled}
          hueIndex={3}
        />
        <ToggleRow
          label={t("settings.configEnabled")}
          hint={t("settings.configEnabledHint")}
          checked={settings.configEnabled}
          onChange={(v) => void toggleDomainEnabled("configEnabled", v)}
          disabled={domainToggleBusy.configEnabled}
          shakeNonce={domainToggleShake.configEnabled}
          hueIndex={4}
        />
        <ToggleRow
          label={t("settings.receiverEnabled")}
          hint={t("settings.receiverEnabledHint")}
          checked={settings.receiverEnabled}
          onChange={(v) => void toggleDomainEnabled("receiverEnabled", v)}
          disabled={domainToggleBusy.receiverEnabled}
          shakeNonce={domainToggleShake.receiverEnabled}
          hueIndex={5}
        />
        <ToggleRow
          label={t("settings.fleetEnabled")}
          hint={t("settings.fleetEnabledHint")}
          checked={settings.fleetEnabled}
          onChange={(v) => void toggleDomainEnabled("fleetEnabled", v)}
          disabled={domainToggleBusy.fleetEnabled}
          shakeNonce={domainToggleShake.fleetEnabled}
          hueIndex={6}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Backup paths                                             */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.paths")} hint={t("settings.pathsHint").replace("{root}", hostMountRoot)} hueIndex={nextHue()}>
        <PathModeSwitch
          label={t("settings.containersPath")}
          domain="containers"
          value={settings.containersPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, containersPath: v } : prev)
          }
          settings={settings}
          setSettings={setSettings}
          save={save}
        />
        <PathModeSwitch
          label={t("settings.vmsPath")}
          domain="vms"
          value={settings.vmsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, vmsPath: v } : prev)
          }
          settings={settings}
          setSettings={setSettings}
          save={save}
        />
        <PathModeSwitch
          label={t("settings.flashPath")}
          domain="flash"
          value={settings.flashPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, flashPath: v } : prev)
          }
          settings={settings}
          setSettings={setSettings}
          save={save}
        />
        <PathModeSwitch
          label={t("settings.configPath")}
          domain="config"
          value={settings.configPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, configPath: v } : prev)
          }
          settings={settings}
          setSettings={setSettings}
          save={save}
        />
        <PathModeSwitch
          label={t("settings.filesPath")}
          domain="files"
          value={settings.filesPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, filesPath: v } : prev)
          }
          settings={settings}
          setSettings={setSettings}
          save={save}
        />
        <FolderBrowser
          label={t("settings.restoreFolder")}
          value={settings.restoreFolder}
          hostMountRoot={hostMountRoot}
          hint={t("settings.restoreFolderHint")}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, restoreFolder: v } : prev)
          }
        />
        <SaveBar
          state={pathSaveState}
          error={pathSaveError}
          onSave={() =>
            void save(
              {
                containersPath: settings.containersPath,
                vmsPath: settings.vmsPath,
                flashPath: settings.flashPath,
                configPath: settings.configPath,
                filesPath: settings.filesPath,
                restoreFolder: settings.restoreFolder,
              },
              setPathSaveState,
              setPathSaveError
            )
          }
          t={t}
          hueIndex={nextHue()}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Local snapshot retention (#51 — moved here from Off-site,  */}
      {/* so it sits with the local backup paths it prunes).                   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card
        title={t("settings.retentionTitle")}
        // Live-review round 3, point 4 sweep: this Card's own intro used to
        // sit as a permanent visible <p> below the title instead of going
        // through the Card `hint` mechanism every OTHER Card-level intro in
        // this file already uses — a plain miss, not a documented exception
        // (compare settings.offsiteHint further down, which stayed visible
        // on purpose with its own comment explaining why). retentionHint
        // (what this Card does) and retentionCombineInfo (the OR-combination
        // rule — a "why wasn't this pruned" answer someone re-checks, same
        // category as notify.healthchecksLifecycle's carve-out) both fold
        // into the one title-level bubble rather than leaving the second as
        // an orphaned bare icon once the wrapping <p> it lived in is gone.
        hint={`${t("settings.retentionHint")} ${t("settings.retentionCombineInfo")}`}
        hueIndex={nextHue()}
      >
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["retentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
            ["retentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
            ["retentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
            ["retentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
          ] as const).map(([key, label, info]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <InfoBubble tip={t(info)} />
              </span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ))}
        </div>
        <SaveBar
          state={retSaveState}
          error={retSaveError}
          onSave={() =>
            void save(
              {
                retentionKeepLast: settings.retentionKeepLast,
                retentionKeepDaily: settings.retentionKeepDaily,
                retentionKeepWeekly: settings.retentionKeepWeekly,
                retentionKeepMonthly: settings.retentionKeepMonthly,
              },
              setRetSaveState,
              setRetSaveError
            )
          }
          t={t}
          hueIndex={nextHue()}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Image cleanup and Unraid's own update-status               */}
      {/* reconciliation (GlimStone follow-up round, merge A): both feed the   */}
      {/* SAME post-backup container-update pipeline (#56, #116). Every field  */}
      {/* here auto-saves instead of batching into a Speichern button —        */}
      {/* mirrors the Domains card's own auto-save mechanism (#142): both      */}
      {/* toggles use the exact optimistic-flip + persist + revert-on-failure  */}
      {/* shape toggleDomainEnabled established (see autoSaveField below).     */}
      {/*   Private container registries (#106) USED to be a third             */}
      {/* sub-section merged into this same card. SPLIT BACK OUT into its own  */}
      {/* standalone Card below (jdp, live-review: "Registries: wir machen     */}
      {/* eine eigene Card daraus") — a registry credential is consulted BY    */}
      {/* the update-pull, but isn't itself image cleanup or Unraid's own      */}
      {/* status reconciliation, so the merge was really "three things on the  */}
      {/* same Storage tab," not three parts of one coherent decision; this    */}
      {/* undoes exactly that, not a mechanical revert of merge A as a whole.  */}
      {/* This card's own title/hint (settings.imageMaintenanceTitle/-Hint,    */}
      {/* same keys, retitled values) dropped every registries mention         */}
      {/* accordingly.                                                        */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.imageMaintenanceTitle")} hint={t("settings.imageMaintenanceHint")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.pruneImageAfterUpdate")}
          hint={t("settings.pruneImageAfterUpdateHint")}
          checked={settings.pruneImageAfterUpdate}
          onChange={(v) => void autoSaveField("pruneImageAfterUpdate", v, setPruneSaveState, setPruneSaveError)}
          disabled={mergedFieldBusy.pruneImageAfterUpdate}
          shakeNonce={mergedFieldShake.pruneImageAfterUpdate}
        />
        <ToggleRow
          label={t("settings.reconcileUnraidStatus")}
          hint={t("settings.reconcileUnraidStatusHint")}
          checked={settings.reconcileUnraidUpdateStatus}
          onChange={(v) => void autoSaveField("reconcileUnraidUpdateStatus", v, setReconcileSaveState, setReconcileSaveError)}
          disabled={mergedFieldBusy.reconcileUnraidUpdateStatus}
          shakeNonce={mergedFieldShake.reconcileUnraidUpdateStatus}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — private container registries (#106), its own standalone    */}
      {/* Card again — see the Image Cleanup card's own comment above for why  */}
      {/* it split out. `hint` now carries what used to be a separate          */}
      {/* <h3>+InfoBubble pair right inside the merged card                    */}
      {/* (settings.registriesTitle/-Hint, unchanged keys/values, just         */}
      {/* promoted to the Card's own title/hint slot) — the exact same         */}
      {/* content, through the ONE heading+bubble mechanism every other Card   */}
      {/* on this page already uses instead of a second, bespoke one. No       */}
      {/* `border-t` divider carried over either — that only ever separated    */}
      {/* this sub-section from its two former siblings; a standalone Card     */}
      {/* already has its own surface/edge doing that job, same as every       */}
      {/* other single-purpose Card in this file.                              */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.registriesTitle")} hint={t("settings.registriesHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-3">
          {settings.registryAuths.length === 0 && (
            <p className="text-sm text-carbon-textMuted">
              {t("settings.registriesEmpty")}
            </p>
          )}
          {settings.registryAuths.map((entry, i) => {
            // Fallback only guards a transient/impossible index mismatch (see
            // registryRowIds' declaration) — every mutation site below keeps
            // the two arrays in lockstep, so this should never actually miss.
            const rowId = registryRowIds[i] ?? `registry-row-fallback-${i}`;
            return (
            <div
              key={rowId}
              className="grid grid-cols-1 sm:grid-cols-[1fr_1fr_1fr_auto] gap-2 items-end"
            >
              <label className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.registryHost")}
                </span>
                <input
                  type="text"
                  value={entry.host}
                  placeholder="ghcr.io"
                  onChange={(e) => {
                    const host = e.target.value;
                    const nextAuths = settings.registryAuths.map((a, j) =>
                      j === i ? { ...a, host } : a
                    );
                    setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                    debouncedSave("registryAuths", () => saveRegistries(nextAuths, registryRowIds));
                  }}
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.registryUser")}
                </span>
                <input
                  type="text"
                  value={entry.username}
                  autoComplete="off"
                  onChange={(e) => {
                    const username = e.target.value;
                    const nextAuths = settings.registryAuths.map((a, j) =>
                      j === i ? { ...a, username } : a
                    );
                    setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                    debouncedSave("registryAuths", () => saveRegistries(nextAuths, registryRowIds));
                  }}
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.registryToken")}
                </span>
                <RevealInput
                  visible={!!registryTokenVisible[rowId]}
                  onToggleVisible={() =>
                    setRegistryTokenVisible((p) => ({ ...p, [rowId]: !p[rowId] }))
                  }
                  showLabel={t("common.showValue")}
                  hideLabel={t("common.hideValue")}
                  value={entry.token}
                  autoComplete="new-password"
                  placeholder={
                    entry.tokenSet && entry.token === ""
                      ? t("cloud.secretSet")
                      : ""
                  }
                  onChange={(e) => {
                    const token = e.target.value;
                    const nextAuths = settings.registryAuths.map((a, j) =>
                      j === i ? { ...a, token } : a
                    );
                    setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                    debouncedSave("registryAuths", () => saveRegistries(nextAuths, registryRowIds));
                  }}
                  wrapperClassName="w-full"
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
                />
              </label>
              {/* Square icon-only remove button with a trash-can glyph (jdp,
                  live-review: "Wenn man eine Registry hinzufügt, soll der
                  Entfernen-Button quadratisch sein mit Mülleimer-Icon") — was
                  a bare text `<button>` ("Entfernen"/"Remove"). IconTipButton
                  (components/IconTipButton.tsx) for the same real
                  `.glim-bubble` hover tooltip every other icon-only control on
                  this page already gets, not a native `title=`;
                  `settings.registryRemove`'s existing value moves from
                  visible button text to this tooltip's own content unchanged
                  — same "text moves onto the tip, key stays" move the
                  Registry-add button below already made. Same `h-8 w-8`/
                  `rounded-control`/`bg-carbon-surface3` icon-badge footprint
                  as that Registry-add button and FolderBrowser's own Browse
                  button, not a fresh guess: this row's own three text fields
                  are `text-sm px-3 py-1.5` — the SAME classes already
                  measured live to render at 32px for those other controls
                  (see Selector.tsx's own `iconOnly` doc for that
                  measurement's full writeup) — so 32px is this row's real
                  control height too, confirmed, not assumed from a token
                  used elsewhere. IconTrash (components/Sidebar.tsx) drawn
                  fresh for this — no trash glyph existed in this codebase
                  yet — filled/`currentColor`-only, no `stroke`, matching
                  every other icon in that file's icon-only-badge set. */}
              <IconTipButton
                tip={t("settings.registryRemove")}
                onClick={() => {
                  // Removing a row is a discrete action, not a text edit — it
                  // saves IMMEDIATELY (no debounce), and cancels any pending
                  // debounced save from an edit elsewhere in this section so
                  // a stale pre-removal snapshot can't land after this one.
                  const nextAuths = settings.registryAuths.filter((_, j) => j !== i);
                  const nextRowIds = registryRowIds.filter((_, j) => j !== i);
                  setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                  setRegistryRowIds(nextRowIds);
                  // Drop this row's reveal-state entry too, so neither an id
                  // nor a stray "revealed" flag survives to be picked up by
                  // whatever row slides into this index next.
                  setRegistryTokenVisible((prev) => {
                    if (!(rowId in prev)) return prev;
                    const next = { ...prev };
                    delete next[rowId];
                    return next;
                  });
                  cancelDebounce("registryAuths");
                  saveRegistries(nextAuths, nextRowIds);
                }}
                className="shrink-0 inline-flex items-center justify-center rounded-control bg-carbon-surface3 h-8 w-8 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
              >
                <IconTrash />
              </IconTipButton>
            </div>
            );
          })}
          {/* Icon-only + right-aligned (GlimStone follow-up round, live-review:
              "Registry hinzufügen button soll bündig nach rechts... einen
              Glyph statt Text bekommen, mit Hover-Infobubble") — `flex
              justify-end` is this file's own established idiom for a single
              trailing action in an otherwise block-level row (see e.g.
              Dashboard.tsx's/VMs.tsx's identical `<div className="flex
              justify-end">` wrapper for a lone action). The row's own text
              label moves onto the button's `IconTipButton` tip instead of
              disappearing — an icon-only trigger has no other way to say
              what it does. Same `h-8 w-8`/`rounded-control`/
              `bg-carbon-surface3` footprint as FolderBrowser's own
              "Durchsuchen" icon button (that fix's own comment: the one
              real field/control height already established on this page,
              32px, not a new bracket invented for this call site). */}
          <div className="flex justify-end">
            <IconTipButton
              tip={t("settings.registryAdd")}
              onClick={() => {
                setSettings((prev) => {
                  if (!prev) return prev;
                  const blank: RegistryAuthEntry = {
                    host: "",
                    username: "",
                    token: "",
                    tokenSet: false,
                  };
                  return { ...prev, registryAuths: [...prev.registryAuths, blank] };
                });
                // A brand-new row always starts with its OWN fresh id — never
                // reusing one, so it can't inherit a stale "revealed" flag left
                // behind by a since-removed row that used to sit at this index.
                // Not saved yet — a blank row has nothing worth persisting
                // until a field in it is actually filled in (debouncedSave
                // above then fires, and its own "kept" filter would drop it
                // again anyway if it's abandoned blank).
                setRegistryRowIds((prev) => [...prev, randomId()]);
              }}
              className="shrink-0 inline-flex items-center justify-center rounded-control bg-carbon-surface3 h-8 w-8 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
            >
              <IconAdd />
            </IconTipButton>
          </div>
        </div>
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — restic cache size limit. The persistent cache under        */}
      {/* /config (RESTIC_CACHE_DIR) survives restarts and would otherwise     */}
      {/* grow unbounded; LRU per-repo caches are evicted after scheduled runs.*/}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline (not the <Advanced> wrapper component): the
          wrapper takes children as an ALREADY-BUILT prop, so this Card's own
          hueIndex={nextHue()} would fire every render regardless of whether
          Advanced end up showing it — caught live (Playwright against the
          real container) as a hue slot silently "spent" on a Card that never
          painted, shifting every later Storage-tab heading by one position
          while Advanced was off. Plain `&&` short-circuits properly, exactly
          like every other conditional Card on this page. */}
      {tab === "storage" && advanced && (
      <Card title={t("settings.cacheTitle")} hint={t("settings.cacheHint")} hueIndex={nextHue()}>
        <label className="flex flex-col gap-1 sm:w-1/2">
          <span className="text-xs text-carbon-textSub">{t("settings.cacheLimitLabel")}</span>
          <input
            type="number"
            min={0}
            value={settings.resticCacheMaxMB}
            onChange={(e) => {
              // Structural cast (cf. handleLogoutAll): runtime-identical to
              // e.target.value, but immune to the broken DOM lib resolution.
              const raw = (e.target as unknown as { value: string }).value;
              const n = Math.max(0, parseInt(raw, 10) || 0);
              setSettings((prev) => (prev ? { ...prev, resticCacheMaxMB: n } : prev));
            }}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
          />
        </label>
        <SaveBar
          state={cacheSaveState}
          error={cacheSaveError}
          onSave={() =>
            void save(
              { resticCacheMaxMB: settings.resticCacheMaxMB },
              setCacheSaveState,
              setCacheSaveError
            )
          }
          t={t}
          hueIndex={nextHue()}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Plain-export encryption (age) and the restic repositories'  */}
      {/* own encryption, merged into one card (GlimStone follow-up round,      */}
      {/* merge B). Every field auto-saves instead of batching into a           */}
      {/* Speichern button (#142's own mechanism) — the two toggles use         */}
      {/* autoSaveField (optimistic + revert-on-failure); the recipients field  */}
      {/* debounces instead, same reasoning as the registries fields in the     */}
      {/* Image Cleanup card above.                                            */}
      {/*   Flash-ZIP-Export (#28) used to be a third sub-section in THIS same  */}
      {/* card. Moved out in TWO steps, live-review (jdp): first "trenn bitte   */}
      {/* flash zip export und den rest wieder in zwei separate cards", then    */}
      {/* — superseding that — "soll die flash zip export toggle nicht einfach  */}
      {/* in den flash tab? macht doch mehr sinn." It now lives on the Flash    */}
      {/* page itself (pages/Flash.tsx's own FlashZipExportCard, exported from  */}
      {/* this file the same way AccentCard/ThemeCard/RcloneCard/CloudCard      */}
      {/* already are for cross-page reuse — see that component's own header    */}
      {/* comment for the full move and why it's self-contained rather than     */}
      {/* threaded through SettingsPage's own save()/autoSaveField()). This     */}
      {/* card's own title/hint dropped every flash-zip-export mention          */}
      {/* accordingly — it now only covers what's actually left: plain-export   */}
      {/* encryption and repository encryption, both real "encrypt SOMETHING"   */}
      {/* settings, so `settings.exportsEncryptionTitle`/`Hint` keep their OLD   */}
      {/* key names (an internal identifier, not user-facing) with NEW values.  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.exportsEncryptionTitle")} hint={t("settings.exportsEncryptionHint")} hueIndex={nextHue()}>
        {/* Plain-export encryption (age) -------------------------------------- */}
        {/* No `border-t` divider against Repository encryption below it (jdp,
            live review: "die Linien dazwischen weg") — the Card's own `gap-4`
            between direct children already separates the two sub-sections,
            same spacing-only convention as the Colors Card's own accent/
            rainbow halves and its own rainbow ToggleRow trio
            (settings.rainbow/-Reactive/-Rotate) elsewhere in this file, none
            of which ever had a rule line between their parts either. (A
            THIRD sub-section, Flash-ZIP-Export, used to sit above this one —
            see this Card's own header comment for where it moved.) */}
        <div className="flex flex-col gap-3">
          {/* No more standalone <h3> sub-heading (jdp, live-review: "Export
              und Verschlüsselung: Texte normal formatieren, es sind keine
              Überschriften mehr") — this sub-section is now JUST a single
              ToggleRow with an optional conditional block beneath it, not a
              heading introducing its own block of content, so it shouldn't
              LOOK like one either. `hideLabel` is gone below: the row's own
              native `label` (ToggleRow's plain `text-sm text-carbon-text`
              span, the same normal weight every other row's own caption in
              this app already uses, not the bold/uppercase/tracking-widest
              heading treatment the removed `<h3>` had) is now this
              sub-section's only visible caption. `export.encrypt.title` (the
              old heading's own text, "Encrypt plain exports"/"Plain-Exporte
              verschlüsseln") is retired — the ToggleRow's own
              `export.encrypt.enable` label already names the same action
              ("Encrypt exports with age"/"Exporte mit age verschlüsseln")
              and is the one text a screen reader announces for this switch
              either way, so keeping both would be two competing captions for
              one control. The old heading's own three-sentence InfoBubble
              tip (what age is, what enabling it does) moves onto the
              ToggleRow's own `hint` unchanged — the same content, now
              anchored to the control it actually describes instead of a
              heading standing in front of it. */}
          <ToggleRow
            label={t("export.encrypt.enable")}
            hint={`${t("export.encrypt.hint")} ${t("export.encrypt.ageInfo")} ${t("export.encrypt.enableHint")}`}
            checked={settings.exportEncryptEnabled}
            onChange={(v) => void autoSaveField("exportEncryptEnabled", v, setExportEncSaveState, setExportEncSaveError)}
            disabled={mergedFieldBusy.exportEncryptEnabled}
            shakeNonce={mergedFieldShake.exportEncryptEnabled}
          />
          {settings.exportEncryptEnabled && (
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("export.encrypt.recipients")}</span>
              {/* Live-review round 3 sweep: export.encrypt.recipientsHint below
                  (the "one per line, age1.../SSH key format" caption on the
                  textarea) is left as permanent text on purpose, the same
                  "genuine toss-up" carve-out settings.offsiteHint documents
                  further up this file — it names the exact accepted KEY
                  SYNTAX for a multi-line field someone fills in by pasting one
                  key per line, which reads as reference to consult while
                  composing the list rather than a one-time "what does this
                  toggle do" explainer (that half is already covered by
                  export.encrypt.enableHint, now folded into the sub-heading's
                  own InfoBubble above). Flagged, not force-converted. */}
              <textarea
                value={settings.exportAgeRecipients}
                spellCheck={false}
                rows={3}
                onChange={(e) => {
                  const v = e.target.value;
                  setSettings((prev) => prev ? { ...prev, exportAgeRecipients: v } : prev);
                  debouncedSave("exportAgeRecipients", () =>
                    void save({ exportAgeRecipients: v }, setExportEncSaveState, setExportEncSaveError)
                  );
                }}
                placeholder={t("export.encrypt.recipientsPlaceholder")}
                dir="ltr"
                className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus text-start"
              />
              <span className="text-xs text-carbon-textMuted">{t("export.encrypt.recipientsHint")}</span>
              {!settings.exportAgeRecipients.trim() && (
                <span className="text-xs text-statusFail">{t("export.encrypt.recipientsRequired")}</span>
              )}
            </label>
          )}
        </div>

        {/* Repository encryption ---------------------------------------------- */}
        {/* jdp, live review: "keine Überschrift und nochmal darunter der
            Text. Nur die Überschrift als Text, alles andere in die
            Infobubble." This sub-heading was the one holdout in this card
            still pairing a bare <h3> with a permanent paragraph underneath
            it (settings.encryptionWarning, now settings.encryptionHint) —
            its two siblings above already fold that same kind of one-time
            "here's what this does" text into the heading's own InfoBubble
            (flash.zipExport.hint, export.encrypt.hint+ageInfo). Renamed
            .../Warning -> .../Hint on the move: it's no longer a
            statusWarnBg banner, so it no longer earns the "Warning" name —
            see the still-conditional flash.zipExport.plaintextWarn a few
            lines up for the genuine, actively-risky warning case (only
            rendered while the risk applies) this text never was: it's an
            unconditional, one-time explainer of how the toggle behaves, the
            exact content InfoBubble exists for. No `border-t` here either,
            same reasoning as the Plain-export block above.
              FOLLOW-UP (jdp, live-review, fresh screenshot proved a prior
            round's claim wrong): that earlier pass only bubbled the STATIC
            explainer above — it left the master ToggleRow's own DYNAMIC
            status label ("Aktiviert (Passwort aus APP_KEY)" /
            "Deaktiviert (kein Passwort)") sitting directly under this same
            heading in plain view, which is exactly the line the fresh
            screenshot still showed. `hideLabel` below hides it now, same as
            its two siblings above; the state it used to carry moves into the
            bubble's own tip — computed per render off the live
            `settings.encryptionEnabled` value (the same values
            settings.encryptionOn/Off already translate in every locale, just
            read here instead of handed to the ToggleRow as visible text) —
            rather than a static string, so the bubble still answers "is this
            actually on right now" concretely instead of only explaining the
            feature in the abstract. The switch's own filled/unfilled track
            still shows on/off at a glance without hovering anything. */}
        <div className="flex flex-col gap-3">
          {/* No more standalone <h3> sub-heading here either — same fix, same
              reasoning, as the Plain-export block above (jdp, live-review:
              "Export und Verschlüsselung: Texte normal formatieren, es sind
              keine Überschriften mehr"). `hideLabel` is gone: the ToggleRow's
              own DYNAMIC on/off label ("Enabled (password derived from
              APP_KEY)"/"Disabled (no password)") is now this sub-section's
              only visible caption, at ToggleRow's normal `text-sm
              text-carbon-text` weight, not the retired heading's bold/
              uppercase/tracking-widest treatment. `settings.encryption` (the
              old heading's own generic "Encryption"/"Verschlüsselung" text)
              is retired — the row's own live on/off label already says more
              than that static word did. The bubble's own tip drops the
              on/off-state PREFIX it used to carry (`settings.encryptionOn`/
              `Off` concatenated in front of `settings.encryptionHint`): that
              existed only because the label sitting above it was hidden and
              had nowhere else to show the current state — now that the
              state IS the visible label, repeating it inside the bubble too
              would just be the same sentence twice. */}
          <ToggleRow
            label={
              settings.encryptionEnabled
                ? t("settings.encryptionOn")
                : t("settings.encryptionOff")
            }
            hint={t("settings.encryptionHint")}
            checked={settings.encryptionEnabled}
            onChange={(v) => void autoSaveField("encryptionEnabled", v, setEncSaveState, setEncSaveError)}
            disabled={mergedFieldBusy.encryptionEnabled}
            shakeNonce={mergedFieldShake.encryptionEnabled}
          />
          {settings.encryptionEnabled && (
            <div className="flex flex-col gap-2">
              {/* recovery.why is bubbled, not kept as permanent text, even though
                  it explains a real data-loss risk: the RECURRING "you still
                  haven't saved this" job is already owned by Dashboard.tsx's own
                  separate, more prominent recovery.nagTitle/nagBody banner
                  (dismissed only by recovery.stored) — this paragraph is purely
                  the one-time "here's why, if you're curious" context for the
                  button below it, not the app's only safeguard against
                  forgetting. */}
              <h4 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
                {t("recovery.title")}
                <InfoBubble tip={t("recovery.why")} />
              </h4>
              {/* Icon-only + right-aligned (GlimStone follow-up round,
                  live-review: "...ebenso der Recovery Kit herunterladen
                  button. Beide sollen einen Glyph statt Text bekommen, mit
                  Hover-Infobubble") — the visible "Recovery-Kit
                  herunterladen" label moves onto the IconTipButton's own
                  tip, the only remaining visible text in this sub-section is
                  its heading, same "only heading + a bare bubbled/tooltipped
                  control" shape the encryption toggle right above it now
                  has. `self-end` (not a `flex justify-end` wrapper — this
                  button is already a direct child of the section's own
                  `flex flex-col` above) flips this from the row's start edge
                  to its end edge, RTL-safe, same as every other logical
                  start/end pairing on this page. Same `h-8 w-8`/
                  `rounded-control`/`bg-carbon-surface3` icon-button footprint
                  as FolderBrowser's own Browse button and the Registry-add
                  button above. */}
              <IconTipButton
                tip={t("recovery.download")}
                onClick={() => {
                  setKitError(null);
                  void downloadRecoveryKit().then(setKitError);
                }}
                className="self-end shrink-0 inline-flex items-center justify-center rounded-control bg-carbon-surface3 h-8 w-8 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
              >
                <IconDownload />
              </IconTipButton>
              {kitError && (
                // Backend-provided error text shown verbatim BY DESIGN (e.g. the
                // fail-closed "set a login password" refusal when auth is off) —
                // the API answers English and is not translated client-side.
                <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
              )}
            </div>
          )}
        </div>
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Off-site copy (restic copy replication)                  */}
      {/* Default-mode feature (v4): off-site + ransomware protection is a      */}
      {/* first-class flow, not advanced-only. Deep-linked via /settings#offsite */}
      {/* selects this tab (id kept for back-compat).                          */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && (
      <div id="offsite">
      {/* GlimStone follow-up pass ("half-overlap card notch"): `relative`
          added directly on this <h2> — same bare-heading, no-padding case as
          settings.schedulesBackup above; see Badge.tsx's badgeClassName
          comment. */}
      <h2 className="relative flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>{t("offsite.sectionTitle")}</Badge>
      </h2>
      <Card title={t("settings.offsiteTitle")} hueIndex={nextHue()}>
        {/* GlimStone follow-up pass: the one genuine toss-up in this pass —
            left as permanent text rather than force a call. It names three
            backend URL prefixes (rest:/s3:/b2:), but that's only PARTIALLY
            unique reference: the field's own placeholder already shows a
            rest: example, and offsite.repoLocalHint right below each field
            already documents the relative-path option. What it adds beyond
            those is s3: and b2: as valid prefixes here specifically — real
            but thinner value than RcloneCard's/CloudCard's own hints above
            (the sole documentation of their syntax anywhere). Whether that
            remainder is enough to justify a permanent paragraph, or should
            fold into the placeholder/caption instead, is a real design call,
            not a mechanical one — flagged rather than decided here. */}
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
        {([
          ["containersOffsite", "nav.containers", "containers"],
          ["vmsOffsite", "nav.vms", "vms"],
          ["flashOffsite", "nav.flash", "flash"],
          ["filesOffsite", "nav.files", "files"],
        ] as const).map(([repoKey, label, domain]) => {
          const wizardOpen = offsiteWizard === domain;
          return (
          <div key={repoKey} className="flex flex-col gap-1 border-b border-carbon-border pb-3 last:border-0">
            <div className="flex items-center justify-between">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <span className="inline-flex items-center gap-2">
                {settings[repoKey] && !wizardOpen && (
                  <>
                    <TestConnectionButton domain={domain} t={t} />
                    <ReplicateNowButton domain={domain} t={t} />
                  </>
                )}
                <button
                  type="button"
                  onClick={() => setOffsiteWizard(wizardOpen ? null : domain)}
                  className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover"
                >
                  {wizardOpen ? t("offsite.wizard.close") : t("offsite.wizard.setup")}
                </button>
              </span>
            </div>
            {wizardOpen ? (
              <OffsiteWizard
                domain={domain}
                settings={settings}
                setSettings={setSettings}
                save={save}
                t={t}
              />
            ) : (
              <>
                <input
                  value={settings[repoKey]}
                  spellCheck={false}
                  onChange={(e) =>
                    setSettings((prev) => (prev ? { ...prev, [repoKey]: e.target.value } : prev))
                  }
                  placeholder="rest:http://host:8000/repo"
                  dir="ltr"
                  className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus text-start"
                />
                {/* A mounted share is a perfectly valid off-site target, but the
                    placeholder only ever showed a REST URL — so nothing told the
                    operator a bare relative path works here (issue #138). */}
                <span className="text-xs text-carbon-textMuted">
                  {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
                </span>
              </>
            )}
            {/* Additional off-site targets (multi-off-site): extra copies of this
                domain beyond the primary editor above, managed via the CRUD API. */}
            <OffsiteTargetsSection domain={domain} t={t} />
          </div>
          );
        })}
        <SaveBar
          state={offsiteSaveState}
          error={offsiteSaveError}
          onSave={() =>
            // Repo URLs only — the off-site *cadences* are owned by the Schedules
            // tab now, so this Save no longer writes (or clobbers) them.
            void save(
              {
                containersOffsite: settings.containersOffsite,
                vmsOffsite: settings.vmsOffsite,
                flashOffsite: settings.flashOffsite,
                filesOffsite: settings.filesOffsite,
              },
              setOffsiteSaveState,
              setOffsiteSaveError
            )
          }
          t={t}
        />
      </Card>
      </div>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Retention (off-site repo only; local retention now lives   */}
      {/* in the Storage tab, #51).                                            */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && (
      <Card
        title={t("settings.retentionOffsiteTitle")}
        // Same fix as the local-retention Card above, folding all three
        // sentences (what this Card does, the OR-combination rule, and the
        // immutable-destination override) into the one title-level bubble.
        hint={`${t("settings.retentionOffsiteHint")} ${t("settings.retentionCombineInfo")} ${t("settings.retentionOffsiteImmutableInfo")}`}
        hueIndex={nextHue()}
      >
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["offsiteRetentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
            ["offsiteRetentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
            ["offsiteRetentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
            ["offsiteRetentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
          ] as const).map(([key, label, info]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <InfoBubble tip={t(info)} />
              </span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ))}
        </div>
        <SaveBar
          state={offRetSaveState}
          error={offRetSaveError}
          onSave={() =>
            void save(
              {
                offsiteRetentionKeepLast: settings.offsiteRetentionKeepLast,
                offsiteRetentionKeepDaily: settings.offsiteRetentionKeepDaily,
                offsiteRetentionKeepWeekly: settings.offsiteRetentionKeepWeekly,
                offsiteRetentionKeepMonthly: settings.offsiteRetentionKeepMonthly,
              },
              setOffRetSaveState,
              setOffRetSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Off-site bandwidth                                        */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper — see the Storage
          tab's cacheTitle Card (above) for why: the wrapper's children are
          already built before it decides whether to render them, so a
          hueIndex={nextHue()} inside it fires every render regardless. */}
      {tab === "offsite" && advanced && (
      <Card title={t("settings.offsiteLimits")} hint={t("settings.limitHint")} hueIndex={nextHue()}>
        <div className="grid grid-cols-2 gap-3">
          {([
            ["offsiteLimitUpload", "settings.limitUpload"],
            ["offsiteLimitDownload", "settings.limitDownload"],
          ] as const).map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ))}
        </div>
        <SaveBar
          state={limSaveState}
          error={limSaveError}
          onSave={() =>
            void save(
              {
                offsiteLimitUpload: settings.offsiteLimitUpload,
                offsiteLimitDownload: settings.offsiteLimitDownload,
              },
              setLimSaveState,
              setLimSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Monitoring (Prometheus)                                   */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper — same reason as
          the Storage tab's cacheTitle Card above. */}
      {tab === "system" && advanced && (
      <Card title={t("settings.metrics")} hueIndex={nextHue()}>
        {/* GlimStone follow-up pass: stays permanent text, NOT bubbled — it
            names the exact /metrics path AND the exact
            "Authorization: Bearer <token>" scrape syntax someone pastes into
            Grafana/Uptime Kuma config verbatim, the same "exact syntax to
            copy correctly" carve-out as RcloneCard's/CloudCard's own hints.
            The comment below also documents that the ToggleRow beneath
            deliberately has NO description of its own because THIS paragraph
            already covers it — hiding it behind a hover target would silently
            break that reasoning too. */}
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.metricsHint")}</p>
        {/* No description here: the Card's own hint paragraph above already
            states the /metrics path — a hardcoded "GET /metrics" description
            would just orphan itself once hideLabel hides the row's caption. */}
        <ToggleRow
          hideLabel
          label={t("settings.metricsEnable")}
          checked={settings.metricsEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, metricsEnabled: v } : prev)
          }
        />
        {/* Write-only secret (the GET never echoes it): blank-on-save keeps the
            stored token, so a stored one shows as the same "saved — leave blank
            to keep" placeholder the cloud-credential secrets use. */}
        <label className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.metricsToken")}</span>
          <RevealInput
            {...revealMetricsToken}
            value={settings.metricsToken}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) =>
              setSettings((prev) => prev ? { ...prev, metricsToken: e.target.value } : prev)
            }
            placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
          />
        </label>
        <SaveBar
          state={metricsSaveState}
          error={metricsSaveError}
          onSave={() =>
            void save(
              {
                metricsEnabled: settings.metricsEnabled,
                metricsToken: settings.metricsToken,
                // Keep the is-set flag honest locally: saving a non-blank token
                // stores one; a blank save keeps whatever was stored before.
                metricsTokenSet: settings.metricsToken.trim() !== "" || settings.metricsTokenSet,
              },
              setMetricsSaveState,
              setMetricsSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Dashboard widget (embeddable activity log). Not behind      */}
      {/* Advanced: it is an end-user feature, unlike the ops-y metrics card.  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (
        <>
        <DashboardWidgetCard
          t={t}
          tokenSet={settings.widgetTokenSet}
          onTokenSet={(set) => {
            // Keep BOTH the live and the saved baseline in sync: the token is
            // managed by its own endpoints, so a later card save (which merges
            // onto savedSettings) must not carry a stale widgetTokenSet.
            setSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
            setSavedSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
          }}
          hueIndex={nextHue()}
        />
        <FleetSettingsCard
          t={t}
          settings={settings}
          setSettings={setSettings}
          save={save}
          tokenSet={settings.fleetTokenSet}
          onTokenSet={(set) => {
            setSettings((prev) => (prev ? { ...prev, fleetTokenSet: set } : prev));
            setSavedSettings((prev) => (prev ? { ...prev, fleetTokenSet: set } : prev));
          }}
          hueIndex={nextHue()}
        />
        </>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — VM Backup over SSH                                        */}
      {/* Advanced, OR shown whenever VMs are enabled so the SSH setup you    */}
      {/* need to make VM backups work is never hidden behind Advanced.       */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (advanced || settings.vmsEnabled) && <VMSSHCard t={t} hueIndex={nextHue()} />}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Off-site backends (rclone + cloud credentials). Same     */}
      {/* "not advanced-only" rule as the off-site repo-path Card above: a   */}
      {/* user can't actually USE an rclone:/s3:/rest: off-site URL without  */}
      {/* these credentials, so hiding them behind Advanced silently broke   */}
      {/* off-site setup for Simple-mode users (they'd only find these two   */}
      {/* cards by way of the Recovery page, which never gated them either). */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && <RcloneCard t={t} hueIndex={nextHue()} />}

      {tab === "offsite" && <CloudCard t={t} hueIndex={nextHue()} />}
      {tab === "offsite" && <CloudCredSetsCard t={t} hueIndex={nextHue()} />}

      {/* ------------------------------------------------------------------ */}
      {/* NOTIFICATIONS — NotifyCard (renders always; not re-gated).          */}
      {/* ------------------------------------------------------------------ */}
      {tab === "notifications" && <NotifyCard t={t} platformKind={platformKind} hueIndex={nextHue()} />}

      {/* NOTIFICATIONS — Weekly digest: one summary message per week through
          the channels configured above. Schedule input mirrors the drills/
          tamper cadence editors (CadenceBuilder's own <fieldset disabled>
          handles the dimming — no opacity gate on the wrapping container). */}
      {tab === "notifications" && (
        <Card title={t("settings.digestTitle")} hint={t("settings.digestHint")} hueIndex={nextHue()}>
          <ToggleRow
            hideLabel
            label={t("settings.digestToggle")}
            checked={settings.digestEnabled}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, digestEnabled: v } : prev))
            }
          />
          <div className="rounded-card bg-carbon-surface2 p-4">
            <CadenceBuilder
              label={t("settings.schedule")}
              value={settings.digestSchedule}
              disabled={!settings.digestEnabled}
              onChange={(v) =>
                setSettings((prev) => (prev ? { ...prev, digestSchedule: v } : prev))
              }
            />
          </div>
          <SaveBar
            state={digestSaveState}
            error={digestSaveError}
            onSave={() =>
              void save(
                {
                  digestEnabled: settings.digestEnabled,
                  digestSchedule: settings.digestSchedule,
                },
                setDigestSaveState,
                setDigestSaveError
              )
            }
            t={t}
          />
        </Card>
      )}

      {/* NOTIFICATIONS — Overdue-backup watchdog: a fixed daily check (09:00)
          that pushes ONE notification per overdue episode through the channels
          configured above; a new successful backup re-arms it. */}
      {tab === "notifications" && (
        <Card title={t("settings.watchdogTitle")} hint={t("settings.watchdogHint")} hueIndex={nextHue()}>
          <ToggleRow
            label={t("settings.watchdogToggle")}
            checked={settings.watchdogEnabled}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, watchdogEnabled: v } : prev))
            }
          />
          <SaveBar
            state={watchdogSaveState}
            error={watchdogSaveError}
            onSave={() =>
              void save(
                { watchdogEnabled: settings.watchdogEnabled },
                setWatchdogSaveState,
                setWatchdogSaveError
              )
            }
            t={t}
          />
        </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Spike (host-integration check; KEEP — it is LIVE).         */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper component: the
          wrapper takes `children` as an ALREADY-BUILT prop, so a
          hueIndex={nextHue()} inside it would fire every render regardless
          of whether Advanced ends up showing it — caught live (Playwright
          against the real deployed container: this exact site, plus three
          more of the same shape — cacheTitle/offsiteLimits/metrics above —
          were each silently "spending" a hue slot on a Card that never
          painted, shifting every later heading on that tab by one position
          while Advanced was off). Plain `&&` short-circuits correctly,
          exactly like every other conditional Card on this page — this was
          the one call site that still used the wrapper component instead. */}
      {tab === "system" && advanced && (
        <Card title={t("spike.title")} hueIndex={nextHue()}>
          <SpikePanel t={t} />
        </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* INTEGRITY — Integrity, maintenance & restore drills                 */}
      {/* Default-visible (v4): manual restore drills — including the real     */}
      {/* off-site DR restore — are part of the core ransomware-protection     */}
      {/* flow, alongside the un-gated off-site + retention cards above.       */}
      {/* ------------------------------------------------------------------ */}
      {tab === "integrity" && (
      // No hueIndex: this is the ONLY Card the Integrity tab ever renders
      // (unlike every other tab, nothing here is conditional) — a genuine
      // singleton per design-language's own exclusion ("the only one of its
      // kind on the page keeps the single accent"), not an oversight. A
      // stray nextHue() call was caught live (Playwright against the real
      // container: this heading rendered rainbow position 0 instead of the
      // flat accent) — removed rather than "fixed" by consuming a slot, so
      // no other tab's numbering shifts either.
      <IntegrityCard t={t} settings={settings} setSettings={setSettings} save={save} />
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Security                                                  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (
      <Card title={t("auth.security")} hint={t("auth.passwordHint")} hueIndex={nextHue()}>
        {/* Status badge */}
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {authEnabled ? t("auth.authOn") : t("auth.authOff")}
          </span>
        </div>

        {/* Set / Change password form */}
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {authEnabled ? t("auth.changePassword") : t("auth.setPassword")}
            </label>
            <RevealInput
              {...revealPwNew}
              value={pwNew}
              onChange={(e) => setPwNew(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {t("auth.confirmPassword")}
            </label>
            <RevealInput
              {...revealPwConfirm}
              value={pwConfirm}
              onChange={(e) => setPwConfirm(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
            />
          </div>

          {/* Save / status row */}
          <div className="flex items-center gap-3 pt-1">
            <button
              onClick={() => void handleSetPassword()}
              disabled={pwSaveState === "saving"}
              className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {pwSaveState === "saving" ? (
                <>
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                  {t("auth.saving")}
                </>
              ) : (
                t("settings.save")
              )}
            </button>
            {/* Only the pre-flight mismatch validation error renders here now
                (GlimStone form-engine Task 9) — the post-save success/failure
                notice is a toast instead; see handleSetPassword's own comment. */}
            {pwSaveState === "error" && pwSaveMsg && (
              <span className="text-sm text-statusFail">{pwSaveMsg}</span>
            )}
          </div>
        </div>

        {/* Logout buttons — only shown when currently signed in. Plain sign-out
            clears THIS browser's cookie; "sign out everywhere" rotates the
            server-side session epoch, revoking every outstanding session. */}
        {authEnabled && authAuthed && (
          <div className="pt-2 border-t border-carbon-border flex items-center gap-3">
            <button
              onClick={() => void handleLogout()}
              className="rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("auth.logout")}
            </button>
            <button
              onClick={() => void handleLogoutAll()}
              className="rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("settings.logoutAll")}
            </button>
          </div>
        )}
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL — Language (GlimStone follow-up pass, live-review point 9). */}
      {/* Moved out of Sidebar.tsx's footer — see LanguageCard's own header    */}
      {/* comment above for the full move rationale. Sits right after Domains */}
      {/* (a fundamental, whole-app setting, same register) and right before  */}
      {/* the purely-cosmetic Appearance cluster below.                       */}
      {/* ------------------------------------------------------------------ */}
      {tab === "general" && <LanguageCard t={t} hueIndex={nextHue()} />}

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL — Theme (GlimStone follow-up pass, later live-review round). */}
      {/* Moved out of Sidebar.tsx's footer — see ThemeCard's own header       */}
      {/* comment above. Same register and immediately below Language: both   */}
      {/* are fundamental, whole-app identity settings (not one of the purely */}
      {/* cosmetic Appearance sub-topics below), so this Card sits right      */}
      {/* after Language and right before Accent colour.                      */}
      {/* ------------------------------------------------------------------ */}
      {tab === "general" && <ThemeCard t={t} hueIndex={nextHue()} />}

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL — Appearance                                               */}
      {/* GlimStone follow-up pass, live-review point 5: this used to be ONE  */}
      {/* shared Card with four sub-topics (accent / shape / rainbow / quiet  */}
      {/* toasts) separated by `border-t border-carbon-border` divider lines  */}
      {/* — a real, previously-unnoticed violation of this app's own "never a */}
      {/* border line, only shade/shadow" house rule (see index.css's shape-  */}
      {/* token comments and Badge.tsx's file header: every OTHER visual      */}
      {/* separation in this app comes from a surface's own elevation, not a  */}
      {/* rule). Each of the four became its OWN Card that round — same       */}
      {/* bg-carbon-surface + rounded-card + shadow every other Settings      */}
      {/* topic already renders through, no divider needed because there's   */}
      {/* no longer a shared surface to divide.                              */}
      {/*   LATER live-review round (jdp: "Die card von Akzentfarbe und       */}
      {/* Regenbogenmodus in eine mergen. Gehört ja zusammen"): accent and    */}
      {/* rainbow are back to ONE Card below — see that Card's own header     */}
      {/* comment for the merge, the hint relocation, and the hue-integration  */}
      {/* fixes that landed in the same pass. Shape and Quiet toasts stay     */}
      {/* their own separate Cards; `settings.appearance` (the old umbrella   */}
      {/* title from the FOUR-way split) still has no call site and stays     */}
      {/* removed from every locale rather than kept as a dead key. Same      */}
      {/* "general" tab condition repeated per Card — the pattern every OTHER */}
      {/* multi-Card tab on this page already uses (e.g. the "system" tab's   */}
      {/* Settings Portability Card + AboutFooter further down), not a        */}
      {/* wrapping Fragment introduced just for this section.                 */}
      {/* ------------------------------------------------------------------ */}

      {/* Shape (GlimStone form-engine — shape engine; design-language.md's
          "The user-owned axes": data-shape on <html>, round/soft/square,
          one radius token set driving every rounded corner). lib/shape.ts is
          the JS half (read/write/persist which of the three is chosen, stamp
          the attribute), index.css already carries the matching
          [data-shape="soft"|"square"] radius-token overrides. Lives directly
          above the merged Colors Card below: same kind of setting
          (client-only, applied at the app root — shape.ts's own header
          comment), same "one picker, no Save step" shape.
            Selector, not a bespoke button row: this IS "three mutually
          exclusive options" (design-language.md's "The one horizontal
          selector"), the exact shape Dashboard.tsx's heatmap-domain toggle
          already uses this component for.
            REVERSED (jdp, live-review, extremely emphatic standing rule —
          "Der horizontale Selektor der Ecken ist nicht im Regenbogen-Modus
          integriert... Es soll immer alles in die Farb- und Formengine
          integriert werden!! IMMER!!"): this used to carry `hue={false}`,
          reasoned at the time as "round/soft/square are a form choice, not a
          position in a list, and tinting the segments would compete with
          the choice itself." That is exactly the kind of self-authored
          aesthetic exception jdp has now ruled out categorically — a
          plausible-sounding taste judgement is never grounds to unilaterally
          exclude a control from the colour engine. `hue` now stays on its
          plain `true` default, so this Selector's three segments read
          RAINBOW[0]/[1]/[2] like any other hue-enabled Selector in the app —
          see Selector.tsx's own file header item 1 for the full reversal
          note (Dashboard's heatmap toggle got the identical fix in the same
          pass).
            `size="lg"` (GlimStone follow-up pass, live-review point 1 —
          up from the original "sm"): this is a full, standalone Settings
          decision in its own right, the same visual register as the page's
          OWN 7-tab Selector strip further up this file (also `size="lg"`),
          not a tight toolbar chip like Dashboard's heatmap toggle or
          CadenceBuilder's weekday pills — "sm" undersold it next to
          everything else in this Card.
            No `icon` per item anymore (live-review point 2): the original
          per-option glyph (a small outlined square drawn at a SCALED-DOWN
          6px/2px/0 preview radius — deliberately not the real 10px/5px/0
          --radius-control values, for legibility at 14px) turned out to
          undercut its own point live: a smaller-than-real preview sitting
          right next to the label read as "round isn't very round," the
          opposite of what it was meant to show. Text-only avoids that
          entirely — the real Selector segment the user is looking at IS the
          shape preview, at its own true radius, with no scaled-down stand-in
          competing with it.
            `variant="well"` (GlimStone follow-up pass, live-review point 7 —
          "turn the shape picker into a horizontal selector styled like the
          one in TrickWork"): the one call site exercising Selector's new
          well track (components/Selector.tsx's own file header, item 5) —
          TrickWork's shared padded background with flush, crossfade-only
          segments, no sliding pill. Picked for the FIRST try of this variant
          specifically because it's already icon-free (no glyph competing
          with the track's own look) and already the page's most "three
          mutually exclusive settings, read together as one control"
          Selector on this page — the shape it suits best. A LATER round
          gave the Theme Card's own light/dark picker (above) this exact same
          treatment. Every other Selector on this page (the 7-tab strip
          above, the drill-type toggle further down) stays on the default
          `variant="chip"`, unchanged. */}
      {tab === "general" && (
      <Card title={t("settings.shape")} hint={t("settings.shapeHint")} hueIndex={nextHue()}>
        {/* `inline-flex self-start max-w-full` (jdp, live-review: "Die
            horizontalen Selektoren sollen nicht auf die ganze Card-Breite
            gestreckt werden, sondern eine standardisierte Breite bekommen")
            — same standardized "don't stretch" wrapper as the Theme Card's
            own identical Selector above and the Settings tab strip's
            `tabStripEl` wrapper further down this file; see either one's
            own comment for the full root cause (this Card's `flex flex-col`
            root stretches an un-wrapped direct child to its own full
            content width by default) and why `self-start`, not just
            `inline-flex` alone, is the part that actually opts out of it. */}
        <div className="inline-flex self-start max-w-full">
        <Selector
          items={SHAPES.map((s) => ({
            id: s,
            label: t(`settings.shape.${s}` as TranslationKey),
          }))}
          label={t("settings.shape")}
          select="one"
          active={shape}
          onChange={(id) => {
            setShapeLocal(id as Shape);
            setShape(id as Shape);
          }}
          size="lg"
          variant="well"
        />
        </div>
      </Card>
      )}

      {/* Colors (GlimStone form-engine Phase 2, Task 1; the accent Card and
          the Rainbow Card, MERGED — jdp, live-review: "Die card von
          Akzentfarbe und Regenbogenmodus in eine mergen. Gehört ja
          zusammen"). AccentCard above now returns just its own body (no
          Card wrapper of its own — see its header comment), composed here
          alongside the Rainbow controls this Card used to hold on its own.
          One heading, `settings.colors` ("Colours"/"Farben") — new key, not
          a repurposed `settings.accentColor`/`settings.rainbow`: those two
          stay in use as the sub-topics' own row labels below, so the Card's
          own title needed a THIRD string that reads as "colour, broadly"
          without clashing with either. No `hint` on the Card itself any
          more (see the master toggle below for where Rainbow's own hint
          moved). No divider between the two halves — spacing only, this
          app's established "cards separate sections, never a rule line"
          convention (see the Shape/Rainbow split's own comment above for
          the fuller house-rule writeup); AccentCard's body and the rainbow
          `<div>` below it are simply two direct children of this Card's own
          `flex flex-col gap-4`, the same "adjacent flex children, no divider"
          shape the (now-relocated) Flash-zip-export/Plain-export/Repository
          trio in the Storage tab's encryption Card already established.
            hueIndex: merging two Cards into one Card means one FEWER
          `nextHue()` call in the sequence than before — removed here rather
          than left as a dead call, since `hueSeq++` would otherwise burn a
          position nothing renders. Every Card below this one (Quiet toasts,
          the "system"/"storage"/etc. tabs' own Cards) is still numbered
          correctly with no manual re-numbering: `nextHue()` is a plain
          `hueSeq++` evaluated in JSX order at render time (see this
          function's own `hueSeq`/`nextHue` comment above), so removing one
          call site automatically shifts every LATER one down by one — the
          exact self-correcting behaviour that comment already documents.
            This switch genuinely repaints the app: every hue-enabled
          Selector segment (components/Selector.tsx, its own default —
          twelve call sites across seven files, including the Settings tab
          strip above and the drill-type toggle further down) and the
          container/VM/file-set list rows all read a rainbow position, so
          turning this on sets data-rainbow + --rb-0..--rb-7 on <html> AND
          immediately recolours those real call sites. The sidebar nav is
          deliberately NOT a consumer (Sidebar.tsx carries the reasoning), so
          flipping this switch never changes the rail's own colours.
            The master toggle's own hueIndex/hint fixes are documented right
          on that ToggleRow below — see its own comment for both. */}
      {tab === "general" && (
      <Card title={t("settings.colors")} hueIndex={nextHue()}>
        <AccentCard t={t} />
        <div className="flex flex-col gap-3">
          {/* hueIndex 0/1/2 (jdp, live-review, extremely emphatic — "auch
              nicht die Toggles der Regenbogen-Card! ... Es soll immer alles
              in die Farb- und Formengine integriert werden!! IMMER!!"):
              these three ToggleRows used to carry NO hueIndex at all,
              reasoned in ToggleRow's own doc comment as "not members of an
              equal, trackable list the way seven independent domain toggles
              are, so they correctly keep the flat single accent." That
              exclusion — like the Shape Selector's own former `hue={false}`
              right above — is exactly the self-authored design exception jdp
              has now ruled out: three toggles rendered together, one per
              row, are a list by construction regardless of whether they're
              logically independent or a master-plus-two-sub-options group.
              Given the SAME `.glim-hue`/`hueVars(rainbowAt(i))` treatment
              the Domains Card's seven rows already use (own local 0-based
              index, unrelated to this Card's own `nextHue()` sequence — see
              ToggleRow's `hueIndex` doc). ToggleRow's own comment excluding
              this exact trio by name has been corrected to match. */}
          <ToggleRow
            label={t("settings.rainbow")}
            // Moved DOWN from the Card's own `hint` (jdp, live-review: "die
            // infobubble des regenbogenmodus ist unverständlich und sie soll
            // von titelbadge runterwandern in die toggle zeile") — same
            // `hint` prop mechanism the Reactive/Rotation rows below already
            // use for their own explanations. Text rewritten for this same
            // move: see settings.rainbowHint's own value for the rewrite
            // rationale (a fresh, concrete one-pass explanation, not the old
            // abstract "handed out by position" phrasing jdp found unclear).
            hint={t("settings.rainbowHint")}
            checked={rainbow.on}
            onChange={(v) => updateRainbow({ on: v })}
            hueIndex={0}
          />

          {/* Dimmed via each control's OWN `disabled` — ToggleRow dims its
              switch AND its caption together (rule 15, and the exact fix
              this branch's own ToggleRow carries from Phase 1 Task 4 — see
              its own header comment above). "Switched off, not hidden":
              these stay visible and reachable even while off, so nobody has
              to guess what the mode does. */}
          <ToggleRow
            label={t("settings.rainbowReactive")}
            hint={t("settings.rainbowReactiveHint")}
            checked={rainbow.reactive}
            disabled={!rainbow.on}
            onChange={(v) => updateRainbow({ reactive: v })}
            hueIndex={1}
          />
          <ToggleRow
            label={t("settings.rainbowRotate")}
            hint={t("settings.rainbowRotateHint")}
            checked={rainbow.rotate}
            disabled={!rainbow.on}
            onChange={(v) =>
              // Turning rotation on draws a fresh offset immediately, so
              // the switch does something visible instead of silently
              // re-applying whatever rotation the palette already had.
              updateRainbow({
                rotate: v,
                seed: v ? 1 + Math.floor(Math.random() * (RAINBOW.length - 1)) : 0,
              })
            }
            hueIndex={2}
          />

          {/* The very same row shape as the accent swatches in the Accent
              Card above, because it is the very same job: pick colours. Each
              of the 8 is independently editable; setRainbow()/
              isValidPalette() enforce all-or-nothing validation on the
              resulting palette before it ever reaches
              document.documentElement.style — see lib/appearance.ts.
                Live-review round 3, point 2: the "Palettenfarbe:"/"Palette
              colour" caption in front of the swatches read as noise once you
              can already see eight colour swatches sitting there — removed.
              settings.rainbowPalette itself is NOT orphaned: PaletteSwatch
              still reads it (see that component's own `label` line above)
              for each swatch's title/aria-label ("Palette colour 1", "...2",
              …), so the key stays in every locale unchanged. */}
          <div className="flex items-center gap-2 flex-wrap">
            {rainbow.palette.map((hex, i) => (
              <PaletteSwatch
                key={i}
                hex={hex}
                index={i}
                disabled={!rainbow.on}
                t={t}
                onChange={(v) => {
                  const next = rainbow.palette.slice();
                  next[i] = v;
                  updateRainbow({ palette: next });
                }}
              />
            ))}
            {/* Live-review round 3, point 3: was a plain text "Reset" button.
                Now a Badge, matching this row's own established "everything
                clickable is a badge" convention (Task 5 rule 13) — the first
                live use of shape="circle" (previously type-only; see
                Badge.tsx's BadgeShape comment), sized via the new `icon`
                stage to land on the exact same 28px (h-7 w-7) footprint as
                the PaletteSwatch circles it sits beside, so it reads as part
                of the same row of controls rather than a mismatched
                afterthought.
                  `ms-auto` (pushing it to the row's own far right, flush
                with the ToggleRow switches above) is GONE again — a later
                live-review round (this one) asked it back next to swatch 8
                instead: jdp reviewed the far-right placement live and wants
                the reset control reading as "the next control in the same
                row of colours," not stranded at the row's opposite edge with
                a gap nothing else explains. With no `ms-auto` (and no other
                margin/justify override), this Badge is just the next child
                in the same `flex items-center gap-2` row as the 8 swatches
                above, so the shared `gap-2` places it immediately after the
                8th swatch — the same spacing every swatch already keeps from
                its own neighbour, no special-cased gap needed. Icon-only, so
                `title`+`ariaLabel` both carry common.reset ("Reset"/
                "Zurücksetzen"/…) as the accessible name and native hover
                tooltip — same generic reset wording the Accent Card's own
                (text) reset button above already uses for the identical
                action on a sibling swatch row. */}
            <Badge
              as="button"
              shape="circle"
              size="icon"
              tone="neutral"
              disabled={!rainbow.on}
              onClick={() => updateRainbow({ palette: RAINBOW })}
              title={t("common.reset")}
              ariaLabel={t("common.reset")}
            >
              <svg viewBox="0 0 16 16" width="16" height="16" fill="none" aria-hidden="true">
                <path
                  d="M0.67 2.67 L0.67 6.67 L4.67 6.67 M2.34 10 a6 6 0 1 0 1.42 -6.24 L0.67 6.67"
                  stroke="currentColor"
                  strokeWidth="1.3"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </Badge>
          </div>
        </div>
      </Card>
      )}

      {/* Quiet toasts (GlimStone form-engine Task 9) — the toast system's
          severity-based quiet mode. Its own Card now (previously the last,
          divider-less sub-topic tacked onto the shared Appearance Card), next
          to the other purely client-side display preferences, rather than
          being bolted onto NotifyConfig's server-side "on" field above (that
          one gates external webhook/Matrix/email notifications — a different
          axis entirely; muting a toast in THIS browser must never silently
          change what a webhook receives elsewhere). `hideLabel` because the
          Card's own title already says "Quiet toasts" — the same single-
          purpose-Card pattern this Card kept even after the merged Colors
          Card above went the OTHER way (its own master "Regenbogen-Modus"
          toggle keeps a visible label alongside the Card's title — jdp
          asked for that back explicitly; see that ToggleRow's own comment);
          the `description` (unaffected by this pass) still renders under
          the hidden label. */}
      {tab === "general" && (
      <Card title={t("settings.quietToasts")} hueIndex={nextHue()}>
        <ToggleRow
          hideLabel
          label={t("settings.quietToasts")}
          description={t("settings.quietToastsHint")}
          checked={quiet}
          onChange={setQuiet}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Export / import settings                                   */}
      {/* Portable config file: move this instance's settings + off-site      */}
      {/* destinations (and, opt-in, credentials) to another install. Backups, */}
      {/* snapshots and history are never touched.                            */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && <SettingsPortabilityCard t={t} hueIndex={nextHue()} />}

      {/* SYSTEM — Version + report-a-bug (kept out of the sidebar for a clean UI). */}
      {tab === "system" && <AboutFooter />}
      </div>
    </div>
  );
}
