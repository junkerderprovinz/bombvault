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
import { CheckDraw } from "../components/CheckDraw";
import { Badge } from "../components/Badge";
import { ScheduleRow, scheduleStatus } from "../components/ScheduleBadge";
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
import { DropdownListbox } from "../components/DropdownListbox";
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
import { MOTION_INTENSITIES, getMotionIntensity, setMotionIntensity, type MotionIntensity } from "../lib/motion";
import { Selector } from "../components/Selector";
import { relativeTime } from "../lib/reltime";
import { Flag, IconAdd, IconDownload, IconTrash, IconCheckCircle, IconSync, IconGear, IconClose, IconCopy } from "../components/Sidebar";
import { getResolvedTheme, getTheme, onSystemThemeChange, setTheme, type ResolvedTheme } from "../lib/theme";

// GlimStone version this UI is built against — bump by hand whenever
// index.css / lib/appearance.ts / lib/shape.ts / lib/motion.ts are re-copied
// from a newer github.com/junkerderprovinz/glimstone release (same
// convention KnightLoader's own Settings.tsx uses for its GLIMSTONE_VERSION
// constant — see that file for the precedent). Check that repo's
// CHANGELOG.md for the current number before bumping this by hand.
const GLIMSTONE_VERSION = "1.2.0";

// AboutFooter shows the running BombVault version (+ the GlimStone version
// this UI targets, both linking to BombVault's releases page) and a
// "Report a bug" link, at the bottom of the Settings page so it reads on
// every Settings tab without being wired into each tab body individually
// (jdp, live review: "Die Versionsnummer soll in allen Tabs ganz unten
// stehen, auch die GlimStone-Version soll da stehen" — moved here from a
// single Card on the System tab, which only ever showed it on that one tab).
// See GlimStone's own docs/design-language.md, "The version footer", for
// the full cross-app rationale (first built for KnightLoader).
//
// A genuine sticky FOOTER now (jdp, live review, follow-up round — "Die
// Versionsnummer soll ganz unten auf der Seite stehen, unterhalb der
// untersten Card, oder am unteren Fensterrand wenn die Cards nicht bis
// ganz nach unten reichen — jetzt fahren die Cards hinten durch"): a
// PRIOR pass made this `position: fixed` (rendered through a portal onto
// document.body — see git history) specifically to dodge the tab-slide
// wrapper's `transform` animation, which per the CSS spec briefly turns an
// animated ancestor into the containing block for any `fixed` descendant
// (exactly the bug KnightLoader's own VersionFooter comment documents
// hitting first). That fix traded one bug for another: pinned to the
// viewport regardless of scroll position, it made every Settings tab long
// enough to scroll (Storage, Schedules) scroll its OWN Cards visually
// behind/through the footer instead of the footer sitting after them.
//
// The actual fix is the classic flexbox "sticky footer" page shell, not a
// fixed overlay: SettingsPage's own root (below, its return's top-level
// div) is now itself a flex column that FILLS the scrollable viewport
// (`main` in app/Layout.tsx, whose own child chain was given `flex-1 flex
// flex-col` for exactly this — see that file's own comments), and the one
// `key={tab}` tab-panels wrapper between the heading and this footer is
// `flex-1` (SettingsPage's return, that wrapper's own className) — it
// absorbs whatever spare height is left over, pushing THIS footer, a
// perfectly ordinary last child with no special positioning, down to the
// bottom of that column: right after the last Card when there are enough
// of them to fill (or exceed, and scroll) the window, or flush with the
// window's bottom edge when there are only a few. No portal, no `fixed`,
// no transform-containing-block edge case to dodge in the first place —
// the tab-slide wrapper's animated `transform` only ever mattered for a
// `position: fixed` descendant; a normal-flow sibling rendered AFTER that
// wrapper (never inside it) was never subject to that rule to begin with,
// confirmed live by switching tabs rapidly: the footer holds still at the
// bottom of the window while only the tab content slides.
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
    <div className="flex justify-center">
      {/* Task 5 (rule 13, "everything clickable is a badge — including
          links"): both footer links are real anchors (as="a", not "button")
          — right-click "copy link", middle-click to open in a new tab, the
          browser's own status-bar URL preview — none of which a synthetic
          onClick reproduces. No `pointer-events-none`/`-auto` split needed
          any more (unlike the old fixed-overlay version): a normal-flow row
          only ever occupies its own content box, so there is nothing behind
          it for stray pointer-events to accidentally block. */}
      <div className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
        {version && (
          // tone="muted" (GlimStone follow-up round, jdp's live review: "Die
          // Versionsnummern sollen keinen hellen Hintergrund haben" — this was
          // tone="neutral", i.e. a bg-carbon-surface2 pill, a visibly pale
          // #e8e8e8 chip against the near-white page ground in light theme.
          // See Badge.tsx's own `muted` tone comment for the full reasoning —
          // a version number is a plain metadata caption, the same register
          // as Fleet.tsx's peer-version text or this Card's own Import-preview
          // `<dd>` rows, not a status chip that needs a tinted surface.
          //
          // GlimStone version appended to the SAME badge/link, "BombVault
          // {v} · GlimStone {v}" (KnightLoader's own VersionFooter separator
          // — see that file's precedent — over jdp's other suggested comma;
          // picked for cross-app consistency with the actual shipped
          // reference implementation, not a new delimiter invented here):
          // one quiet line, not a second badge competing for the same
          // corner of the window.
          <Badge
            as="a"
            href="https://github.com/junkerderprovinz/bombvault/releases"
            target="_blank"
            rel="noopener noreferrer"
            tone="muted"
            size="small"
            title={`BombVault ${version} · GlimStone ${GLIMSTONE_VERSION}`}
          >
            BombVault {version} · GlimStone {GLIMSTONE_VERSION}
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
  nested,
}: {
  /** GlimStone follow-up pass (live-review, "Bei allen Zeitplanpicker Cards
   *  soll der Name raus... das ist redundant"): optional, not required —
   *  the four Schedules-tab domain-schedule Cards (Containers/VMs/Flash/
   *  Folders) drop their own title entirely because CadenceBuilder's own
   *  `<legend>{label}</legend>` right inside them already names the same
   *  domain, one level down. Omitting `title` while still passing `hint`
   *  (see below) still renders the heading Badge — just without the now-
   *  redundant text — so the Card keeps its rainbow-hue notch instead of
   *  silently losing it the moment a title goes away (never regress a
   *  control's hue wiring, see this repo's own standing rule). A Card with
   *  neither `title` nor `hint` renders no heading at all. */
  title?: string;
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
  /** This Card is being rendered INSIDE another card that already provides
   *  the surface and the padding — drop both of mine.
   *
   *  Two of this file's exported Cards (CloudCard, RcloneCard) are reused
   *  verbatim inside Recovery's step 3, where the step is itself a `p-4`
   *  `bg-carbon-surface` card. Stacked, that produced a card-inside-a-card
   *  whose background was the LITERAL SAME token as its parent's — invisible
   *  as a card, but very visible as 20px of unexplained indentation: measured
   *  live, the step's own heading notch and its FolderBrowser labels sat at
   *  x=264 while these two cards' notches and fields sat at x=284. jdp, live
   *  review: "Jetzt ist der ganze Bereich auch zu weit rechts angeordnet."
   *
   *  `nested` drops `bg-carbon-surface` (it was painting the parent's own
   *  colour over the parent) and drops the horizontal padding (it was pure
   *  indentation with no visible edge to justify it), so everything lines up
   *  on the parent's content edge. It keeps `pt-5` — the same top padding
   *  `p-5` gave — because that space is not decoration: the heading notch
   *  straddles this box's top edge and pokes half its height DOWN into it, so
   *  removing it would drop the badge straight onto the first field. It keeps
   *  `relative` (the notch resolves against this box), `glim-notch-card` (the
   *  reactive card-wide hover zone) and the whole hue wiring untouched.
   *
   *  Default false: every one of this file's own ~50 Card call sites is a
   *  real, top-level card on the page ground and keeps its surface and its
   *  padding exactly as before. */
  nested?: boolean;
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
    //
    // `.glim-hue` ALSO added directly on this shared wrapper (rainbow-mode
    // completeness sweep, jdp live review, repeated escalation: "Es sind
    // nicht alle Buttons in den Regenbogen-Modus eingepflegt"):
    // `glim-notch-card` alone never redefines --accent/--focus-ring, only
    // the reactive-mode hover reveal. Every individual button inside a Card
    // was until now expected to re-derive its OWN `hueOn`/`hueStyle` from
    // the SAME `hueIndex` this component already receives and re-apply
    // `.glim-hue` + that style by hand (see e.g. the widget/fleet token
    // Disable buttons above) — correct where every call site actually did
    // it, but a manual, easy-to-miss-once-per-button convention across ~50
    // Card() call sites is exactly the shape of gap this sweep exists to
    // close for good. Redefining --accent/--focus-ring ONCE here instead
    // means every descendant inherits it via ordinary CSS custom-property
    // cascade whether or not its own call site remembered the manual tag —
    // harmless where a button already carries its own identical `.glim-hue`
    // (same hueIndex, same computed colour, purely redundant), a genuine fix
    // wherever one didn't.
    <div
      className={`relative glim-notch-card flex flex-col gap-4 ${
        nested ? "pt-5" : "bg-carbon-surface rounded-card p-5"
      }${hueIndex !== undefined ? " glim-hue" : ""}`}
      style={hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      {/* Task 5 (design-language.md rule 11, "every heading is a filled
          section badge") resolution, for whoever finds this next: the <h2>
          tag stays (screen readers still get a real heading, e.g. "heading
          level 2: Off-site Copy"), but its VISIBLE content is now a Badge
          (tone="heading" size="heading" — see Badge.tsx's file header for
          the full colour/size reasoning). */}
      {(title || hint) && (
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
            {title}
            {hint && <InfoBubble tip={hint} onAccent />}
          </Badge>
        </h2>
      )}
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
  shakeNonce,
  pulseNonce,
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
   *  its `title`/`hint` pair. */
  hint?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  // No `hideLabel` prop on this component (REMOVED, not just unused — jdp
  // audit, "es soll nie leere Toggles geben"): every real call site that ever
  // set it was the identical anti-pattern — "the Card's title directly above
  // already says the same thing, so suppress the row's own caption" — and
  // that exact reasoning got built, then reversed, THREE separate times in
  // this one file (the merged Colors Card's master "Regenbogen-Modus" row,
  // RestoreChecksSection's "Automatische Restore-Prüfungen" row, and the
  // notifications tab's weekly-digest row) before this pass finally deleted
  // the capability outright instead of patching a fourth instance. A row's
  // own visible caption must never depend on what text happens to sit in a
  // Card above it — see design-language.md's Toggles/Switches section for
  // the standing rule this now enforces structurally: the prop simply isn't
  // there to reach for anymore. (The underlying Toggle's OWN `hideLabel`
  // stays a real, legitimate prop — a caller that renders this row's label
  // itself, in its own layout, e.g. a hand-rolled row like Containers.tsx's
  // UpdateAfterBackupRow or VMs.tsx's VMIncludeToggle, still needs to tell
  // the bare switch not to print a second, literally duplicate copy right
  // next to the one the caller already drew. What's gone is only the
  // ToggleRow-level shortcut that let a caller skip drawing any caption at
  // all on the reasoning that a Card title elsewhere covers it.)
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
  /** Bump this (any new, truthy number) to replay `.glim-pulse` — the
   *  GlimStone motion-engine's animation 2 (confirmation-pulse), glim-
   *  shake's gentler SUCCESS sibling: a brief glow + a small scale pop on
   *  the switch that a just-completed auto-save actually touched, layered
   *  on top of the toast save() already pushes rather than replacing it.
   *  Same "bump a per-field nonce on the outcome, key the Toggle on it"
   *  shape as shakeNonce above, just the opposite outcome — see save()'s
   *  own `fieldPulse` comment in SettingsPage for where these values come
   *  from (one shared map, bumped for every key in a successful patch, so
   *  every autoSaveField/autoSaveToggle/autoSaveScheduleField/
   *  toggleDomainEnabled call site gets this for free without its own
   *  separate wiring). Undefined/0 (never saved yet, or the LAST outcome on
   *  this row was a failure) renders no `.glim-pulse` class — a fresh page
   *  load never pulses, and a row currently mid-`.glim-shake` never also
   *  pulses (see the combined `key` below for why shake always wins when a
   *  caller somehow has both truthy at once, which no real call site does:
   *  a save either fails or succeeds, never both). */
  pulseNonce?: number;
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
  // Combined remount key (confirmation-pulse, GlimStone motion-engine
  // animation 2): shakeNonce and pulseNonce are two INDEPENDENT counters
  // (a save either fails, bumping the first, or succeeds, bumping the
  // second — never both), so neither alone can safely key the Toggle on its
  // own once both exist — `shakeNonce ?? 0` frozen after one failure would
  // never change again on a later SUCCESS, silently killing the pulse for
  // that row forever. Only build this combined string once either has
  // actually fired at least once; while both are still undefined/0 (a row
  // that has never saved, successfully or not) `key` stays `undefined`
  // exactly like before this animation existed, so a normal page load keys
  // nothing and remounts nothing extra.
  const feedbackKey = shakeNonce || pulseNonce ? `${shakeNonce ?? 0}:${pulseNonce ?? 0}` : undefined;
  return (
    <div
      className={`flex items-start justify-between gap-4${hueOn ? " glim-hue" : ""}`}
      style={hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      <div className="flex flex-col gap-0.5">
        <span className={`flex items-center gap-1.5 text-sm text-carbon-text${dim}`}>
          {label}
          {hint && <InfoBubble tip={hint} />}
        </span>
        {description && (
          <span className={`text-xs text-carbon-textMuted${dim}`}>{description}</span>
        )}
      </div>
      <Toggle
        key={feedbackKey}
        hideLabel
        label={label}
        checked={checked}
        onChange={onChange}
        disabled={disabled}
        className={`mt-0.5${shakeNonce ? " glim-shake" : pulseNonce ? " glim-pulse" : ""}`}
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
//
// Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
// Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
// speichern. Nur dort sollen Speicher-Buttons bleiben, wo es unbedingt sein
// muss."): every one of this file's own call sites converted to auto-save —
// the four genuine hold-outs left on this page (RcloneCard, CloudCredSetsCard,
// OffsiteTargetsSection's own draft editor, SettingsPage's own
// handleSetPassword form) each render a plain `<button>` of their own rather
// than this shared component, for reasons documented at each site. That
// leaves ZERO call sites of THIS component anywhere in the app (Config.tsx's
// own comment above referencing it is a precedent citation, not an import).
// `hueIndex` — added in an earlier round for exactly this component, but
// (per this same sweep's own audit) never actually wired from a real call
// site, the flagged bug this pass was asked to close out — is removed below
// rather than fixed: fixing dead plumbing nothing renders would just be a
// different kind of not-genuinely-integrated. The component itself stays,
// exported and still directly testable, as a plain reusable primitive for
// whichever FUTURE control turns out to have as hard a reason to keep a
// manual Save as the four exceptions above — should one arrive, wire a real
// hueIndex through then, the same way every other hue-aware control on this
// page already does, rather than pre-emptively re-adding unused plumbing now.
// ---------------------------------------------------------------------------

type SaveState = "idle" | "saving" | "saved" | "error";

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
      <button
        onClick={onSave}
        disabled={disabled || state === "saving"}
        className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
  // h-8 w-8 (32px outer, 28px visible disc inside the 2px ring), not the
  // h-7 w-7 this swatch used to be: the reset badge sharing this row is a
  // square icon badge, and every square icon badge in the app is 32px
  // (Badge.tsx, "ONE SIZE FOR SQUARE ICON BADGES"). The swatch follows the
  // badge, not the other way round — jdp has twice reported this row when the
  // two disagreed ("der Reset-Badge ist größer als die Farbfelder"), so they
  // are kept equal by moving whichever side is NOT bound by the app-wide rule.
  return (
    <ColorPickerSwatch
      value={hex}
      onChange={onChange}
      label={label}
      disabled={disabled}
      className="h-8 w-8 shrink-0 rounded-pill border-2 border-carbon-border transition-transform hover:scale-110 disabled:cursor-not-allowed disabled:opacity-50"
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
  // Inner disc w-7 h-7 (28px) inside the wrapper's own 2px ring = a 32px outer
  // box, matching PaletteSwatch above and, more to the point, matching the
  // square reset badge that shares this row: every square icon badge in the
  // app is 32px (Badge.tsx, "ONE SIZE FOR SQUARE ICON BADGES") and the
  // swatches follow it. Was w-6 h-6 (24px disc, 28px outer) back when the
  // reset badge was 28px.
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
        className="w-7 h-7 rounded-pill"
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
// matched here for the same granularity, the same Badge shape (now
// shape="square"), the same SVG glyph.
//
// ONE reset, not two (jdp, live-review, re-reporting a third time after two
// prior rounds each declared this fixed: "Der Resetbutton ist da, hat aber
// keine Funktion und der Zurücksetzen-Text ist immer noch da"). Both halves
// of that sentence had the same underlying cause — this row was rendering
// TWO different, competing reset controls side by side:
//   1. an icon Badge gated `disabled={presetsAreDefault}`, resetting only the
//      eight PRESET swatches, and
//   2. a plain TEXT button gated `accentHex !== DEFAULT_ACCENT`, resetting
//      only the ACTIVE ACCENT.
// A previous round converted (1) to a badge and simply overlooked (2), so the
// row ended up with a badge that is dead in the overwhelmingly common case
// (presets are at their shipped defaults for anyone who has never opened a
// preset's editor popover — i.e. almost everyone) sitting right next to a
// text link that appears in exactly the state where the badge is dead. The
// user-visible result is precisely what jdp reported: a permanently greyed
// badge plus a stray "Zurücksetzen" text. Neither control was individually
// broken; the SPLIT was the defect. They are now a single Badge that resets
// the active accent AND the presets, enabled whenever EITHER has drifted.
//   The lesson worth keeping: "the element exists and has the right classes"
// was true for the badge in every prior round's live check, and the control
// was still useless. A reset control's contract is that it is reachable and
// does something in the state a real user is actually in — that has to be
// exercised, not merely rendered.
//
// ALWAYS RENDERED now, disabled (not hidden) once nothing is left to reset
// (jdp, live-review, re-reporting after a prior round claimed this was
// already fixed: "Bei der Akzentfarbe ist das Zurücksetzen immer noch kein
// Badge mit Glyph" — fresh live inspection of the DEFAULT/untouched state,
// the one nearly every real visitor and every prior review actually sees,
// found this control was NOT a badge with a glyph at all — because it wasn't
// rendered AT ALL: the previous `{!presetsAreDefault && (...)}` conditional
// unmounts the entire Badge the moment every preset matches its shipped
// default, which is true for anyone who has only ever clicked a preset to
// select it, without also editing one via the popover — the far MORE common
// path than the rarer "edit a preset's own colour" one this condition
// actually tracks. A conditionally-UNMOUNTED control cannot be measured,
// screenshotted, or recognised as "a badge" by a reviewer who hasn't first
// performed the one specific drift-inducing action — exactly how a
// genuinely-fixed badge kept getting re-reported as broken. Switched to the
// SAME pattern the rainbow-palette reset (a few hundred lines below) already
// used — `disabled={presetsAreDefault}` on an unconditionally-rendered
// Badge, dimmed via Badge's own built-in `disabled:opacity-50` exactly like
// every other disabled control in this app, never truly gone — which also
// makes the "these two mirror each other" pairing this file already claimed
// in prose (see below) actually TRUE for visibility too, not just for shape/
// tone/size/glyph.
//
// No own `<Card>` wrapper any more (jdp, live-review: "Die card von
// Akzentfarbe und Regenbogenmodus in eine mergen. Gehört ja zusammen") —
// this now returns just its own body content, composed inside the shared
// "settings.colors" Card alongside the Rainbow controls at that Card's own
// call site in SettingsPage.
//
// `hueIndex` REMOVED from the signature again (GlimStone follow-up round,
// jdp's neutral-reset-badge fix below): it existed for exactly one reason —
// feeding the preset-reset Badge's own `hueIndex` so its fill matched the
// enclosing Card's rainbow position — and that Badge is now deliberately
// `tone="neutral"` (see its own comment below for the full "why a reset
// control must NOT join the rainbow" reasoning), so the prop had no
// remaining reader anywhere in this component. Left in place it would be
// dead plumbing that `noUnusedParameters` would flag outright. The Card's
// own heading notch keeps its hue independently at its own call site (it
// was never threaded through here — see that call site's own `hueIdx`
// comment). Settings.accentCard.dom.test.tsx's own harness dropped the now-
// nonexistent prop in the same round.
// ---------------------------------------------------------------------------
export function AccentCard({
  t,
}: {
  t: ReturnType<typeof useT>["t"];
}) {
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

  // ONE reset, TWO things reset (see this component's own header comment for
  // the "two competing resets" root cause). Both halves of the row's state
  // get their own predicate so the single control can be enabled whenever
  // EITHER has drifted, and the click can restore both unconditionally.
  const accentIsDefault = accentHex.toLowerCase() === DEFAULT_ACCENT.toLowerCase();
  const presetsAreDefault = presets.every(
    (hex, i) => hex.toLowerCase() === DEFAULT_ACCENT_PRESETS[i]?.toLowerCase()
  );
  const nothingToReset = accentIsDefault && presetsAreDefault;

  return (
    <div className="flex items-center gap-3 flex-wrap">
      {/* Row label — bare now, no trailing colon (jdp, live-review: "Der
          Doppelpunkt nach Akzentfarbe und Farbpalette weg"). The colon was
          only ever appended here in JSX, never baked into the
          settings.accentColor string itself (that string is ALSO read bare,
          with no colon, as the swatch's own aria-label/title below — see
          this key's history a few rounds back) — checked across all 42
          locales, none bakes a colon into the translated text either, so
          dropping the literal ":" here is the complete, single-source fix. */}
      <span className="text-sm text-carbon-text">{t("settings.accentColor")}</span>
      {/* EVERYTHING else on this row — custom-colour swatch, presets, both
          resets — is now ONE right-aligned group (jdp, live-review: "Das
          Akzentfarbeauswahlfeld auch nach rechts"). A prior round's `ms-auto`
          only wrapped the presets+reset sub-group, leaving this
          ColorPickerSwatch behind at the row's start next to the label; jdp
          reviewed that live and wants the swatch pulled into the same
          right-aligned cluster, not left as the one holdout. Same `ms-auto`
          idiom this app already uses everywhere else for "push this to the
          row's own far right" (Containers.tsx/Fleet.tsx's own trailing
          metadata, and the rainbow-reset-badge row a few hundred lines
          below) — the wrapper just moved up to include one more child,
          nothing new invented. */}
      <div className="flex items-center gap-2 flex-wrap ms-auto">
        {/* Custom-colour trigger — a flat swatch, same size/shape as the
            preset swatches beside it (design-language.md, "The user-owned
            axes" > Accent: every custom colour value gets the SAME
            trigger). Opens the shared GlimStone picker popover instead of
            a native <input type="color"> — see ColorPickerPopover.tsx's
            own header comment for why (jdp: "kein eigenes Fenster welches
            sich öffnet" — no separate window opening).
              w-8 h-8 (32px outer, 28px disc inside the 2px ring): the doc
            above says "same size as the preset swatches beside it", but the
            code did not deliver that — this was `w-6 h-6`, which with
            border-box sizing is a 24px OUTER box against the presets' own 28px
            (measured live: "Akzentfarbe" 24x24, "Voreinstellung 1..8" 28x28).
            A pre-existing 4px mismatch, now closed on the 32px the whole row
            moved to when every square icon badge in the app was unified — see
            Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES". */}
        <ColorPickerSwatch
          value={accentHex}
          onChange={selectAccent}
          label={t("settings.accentColor")}
          className="w-8 h-8 rounded-pill border-2 border-carbon-border transition-transform hover:scale-110"
        />
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
        {/* THE reset for this whole row — ONE control that restores BOTH the
            active accent (accentHex → DEFAULT_ACCENT) and the eight preset
            swatches (presets → DEFAULT_ACCENT_PRESETS). Previously these
            were two separate, competing controls sitting side by side; see
            this component's own header comment for the full root cause and
            why merging them is the fix rather than repairing either one.
              `disabled={nothingToReset}` — enabled the moment EITHER half has
            drifted, which is the normal case for anyone who has picked any
            non-default accent at all (one click on any preset but the first
            already gets you there). The old `disabled={presetsAreDefault}`
            tracked only the RARER half — editing a preset's own colour via
            the popover — so in every ordinary session this badge sat
            permanently greyed out: present, correctly shaped, and completely
            inert (jdp: "Der Resetbutton ist da, hat aber keine Funktion").
            An always-disabled control is not a fixed control.
              SQUARE (jdp, live-review: "Die Zurücksetzen-Option soll ein
            quadratischer Badge mit Glyph sein"). `shape="square"` still
            resolves through `rounded-control` (the shape engine's own live
            token — see Badge.tsx's BadgeShape doc), so this genuinely reads
            round/soft/square per the user's own shape-engine choice, not a
            hardcoded corner — under the "Rund" shape-engine setting this
            renders as a full circle, same as every other square badge in
            the app, which is expected, not a regression.
              NEUTRAL now, not hue-tinted (jdp, re-reporting: "Der
            Reset-Badge soll ... nicht farbig sein, damit er sich besser von
            den Farbflächen abhebt" — it must NOT blend in as one more colour
            option in a row whose entire job is presenting colour options).
            DELIBERATE EXCEPTION to this app's standing "every icon-only
            badge gets real colour-engine integration" rule: a reset control
            sitting directly beside the very colour swatches it resets is
            the one role where taking a rainbow/accent hue is actively
            counter-productive — a coloured reset button reads as just
            another swatch to click through, not as a distinct utility
            action set apart from the row it acts on. `tone="neutral"`
            (Badge's own default, spelled out explicitly here so this
            exception is visible at the call site, not an accident of
            omission) resolves to `bg-carbon-surface2 text-carbon-textSub` —
            the same flat, hue-immune grey chip every other "no real status"
            badge in the app already uses. `border-2 border-carbon-border`
            (NEW) matches AccentPresetSwatch's own wrapping border exactly
            (see that component's own JSX a few dozen lines up) — without
            it, this Badge's solid fill reaches the full 28×28 box edge to
            edge while every neighbouring swatch's visible colour disc is
            actually only 24×24 (28px border-box minus its own 2px ring),
            so even at an IDENTICAL measured bounding box the reset control
            read as visibly BIGGER (jdp: "der Reset-Badge ist größer als die
            Farbfelder") — an optical-weight mismatch a bounding-box
            measurement alone can't catch. The border closes that gap: same
            28×28 outer box, same 24×24 inner content area as every swatch
            beside it.
              IconResetArrow (redesigned — see that icon's own header
            comment for why the old thinner ring/arrowhead is gone) — the
            established "counter-clockwise arrow = reset" convention already
            used by Sidebar.tsx's IconRecovery/IconRestore, redrawn bolder
            specifically for this small-badge-in-a-busy-row role.
              ALWAYS rendered, disabled (never conditionally unmounted) — see
            this component's own header comment for the earlier "isn't a badge
            at all" root cause that established this pattern.
              size="icon" — the app's ONE square-icon-badge size (32px), not
            a number this control derives from its own row. It used to be
            28px, chosen to match the swatches beside it; when every square
            icon badge in the app was unified on 32px, the swatch row moved
            WITH the badge (AccentPresetSwatch's inner disc w-6 → w-7, so its
            bordered wrapper measures 32px, and the custom-accent swatch
            w-6 → w-8, which also closes a stray 24-vs-28px gap it had against
            its own presets). Badges set the size, swatches follow — a swatch
            is a colour disc, not an icon badge, so it is the free variable
            here. Same treatment on the rainbow-palette reset below.
              tip (not title/ariaLabel) — same convention: an icon-only
            trigger gets IconTipButton's real hover/focus bubble, not a silent
            native title balloon. Text is settings.accentReset, which NAMES
            BOTH halves of what one click now does ("Reset accent color and
            presets") — the old settings.accentPresetsReset ("Reset presets")
            described only half the action and would have been an outright lie
            on a control that also resets the live accent. Kept identical to
            the rainbow-palette reset badge below in shape/tone/size/glyph/
            border — the established "these two mirror each other" pairing. */}
        <Badge
          as="button"
          shape="square"
          size="icon"
          tone="neutral"
          className="border-2 border-carbon-border"
          disabled={nothingToReset}
          onClick={() => {
            selectAccent(DEFAULT_ACCENT);
            setPresets(setAccentPresets(DEFAULT_ACCENT_PRESETS));
          }}
          tip={t("settings.accentReset")}
        >
          <IconResetArrow />
        </Badge>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Language Card (GlimStone follow-up pass, live-review point 9) — the app's
// UI-language switcher, MOVED here out of Sidebar.tsx's own footer, not
// duplicated (jdp: "verschieb den Sprachschalter... auch als eigene card ins
// allgemein setting"). Same picker mechanism as before: useT()'s
// lang/setLanguage/languages (lib/i18n.ts — a flat 42-locale list, persisted
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
  // The BUTTON itself, not its wrapper — DropdownListbox sizes the portalled
  // panel to what this ref measures, and an `inline-block` wrapper inside a
  // flex column is blockified and stretched to the Card's full width. See
  // StopContainersEditor's own copy of this note in Containers.tsx.
  const ref = useRef<HTMLButtonElement>(null);

  const current = languages.find((l) => l.code === lang) ?? languages[0];

  // Outside-click / Escape / scroll dismissal lives in DropdownListbox now,
  // together with the panel it dismisses — this card's own two hand-rolled
  // listener effects are gone. They were also the pattern Containers.tsx's
  // multi-select copied wholesale, and that copy sat under an
  // `overflow-hidden` card that hard-clipped the panel (see
  // DropdownListbox.tsx's header). Fixing the copy and leaving the original
  // behind as a second, subtly different implementation of the same control
  // is exactly the sibling drift this repo keeps out; both call sites now
  // render the one shared, portalled panel.
  //   Not merely cosmetic here either: this list is 42 locales deep and
  // always opened straight downward with no viewport awareness at all, so on
  // a short window it ran off the bottom edge. The shared panel clamps and
  // flips above the trigger when there is no room below.

  return (
    <Card title={t("settings.language")} hueIndex={hueIndex}>
      <div className="inline-block">
        {/* w-48 (GlimStone follow-up pass, live-review round — "widen the
            Language button, then match the Theme button to it"): was
            content-hugging (only as wide as the current flag+label pair),
            which read as too narrow/incidental for a deliberate settings
            control. w-48 (192px) isn't an arbitrary new number — it was the
            SAME width this button's own dropdown listbox already hard-coded,
            so the trigger sits flush above the exact footprint of the menu it
            opens, rather than a narrower button popping open a visibly wider
            list. That number is no longer restated on the listbox at all:
            DropdownListbox sizes the portalled panel to THIS trigger's own
            measured width, so the two can no longer drift apart.
            `truncate`/`min-w-0` on
            the label span below keeps a genuinely long locale name (this
            list has 42) from overflowing the now-fixed width instead of
            just growing the button the way it used to. */}
        <button
          ref={ref}
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
        <DropdownListbox
          open={open}
          onClose={() => setOpen(false)}
          triggerRef={ref}
          label={t("language.label")}
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
        </DropdownListbox>
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
// decision register, not a tight toolbar chip) and `variant="well"
// equalWidth` (TrickWork's shared padded groove, flush crossfade-only
// segments, at the big pinned scale — see Selector.tsx's file header items 5
// and 6). No `hue={false}` — see that Card's own
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
  // FILLED rays (design-language.md "Icon glyphs", rule 220 — "a structural
  // detail that has to stay a thin line... renders as a thin filled shape,
  // never stroke"): the disc itself was already filled (see the fixed-hex
  // colour note above, unrelated to this fix); only the 8 sunburst rays were
  // still stroked lines. Each ray is now a thin filled rounded rect — the
  // four cardinal ones axis-aligned, the four diagonal ones a `<rect>`
  // rotated to its own 45°, same start/end points as the old stroke
  // segments — same `#FACC15` fixed fill as the disc (still deliberately
  // not `currentColor`/the accent, per this Card's own header comment).
  const sunIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="#FACC15" className="shrink-0" aria-hidden="true">
      <circle cx="10" cy="10" r="4.25" />
      <rect x="9.15" y="2" width="1.7" height="2" rx="0.85" />
      <rect x="9.15" y="16" width="1.7" height="2" rx="0.85" />
      <rect x="2" y="9.15" width="2" height="1.7" rx="0.85" />
      <rect x="16" y="9.15" width="2" height="1.7" rx="0.85" />
      <rect x="4.435" y="4.785" width="2.4" height="1.7" rx="0.85" transform="rotate(45 5.635 5.635)" />
      <rect x="13.165" y="13.515" width="2.4" height="1.7" rx="0.85" transform="rotate(45 14.365 14.365)" />
      <rect x="4.435" y="13.515" width="2.4" height="1.7" rx="0.85" transform="rotate(-45 5.635 14.365)" />
      <rect x="13.165" y="4.785" width="2.4" height="1.7" rx="0.85" transform="rotate(-45 14.365 5.635)" />
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
      {/* NO `inline-flex self-start max-w-full` wrapper any more (jdp,
          live-review: "Die horizontalen Selektoren sollen nicht auf die ganze
          Card-Breite gestreckt werden, sondern eine standardisierte Breite
          bekommen"). That ask is still honoured — this Card's own root is
          `flex flex-col` (Card's own comment above), whose default
          `align-items: stretch` would otherwise blockify this row to the
          Card's full content width — but round 8 moved the mechanism INTO
          the variant: `variant="well"` now carries `w-fit max-w-full` itself,
          and `width: fit-content` is not `auto`, so a stretch alignment no
          longer applies to it. Three call sites hand-rolling the identical
          wrapper div was the same "one control, two mechanisms" drift that
          round's whole change is about; see Selector.tsx's file header item
          6. (The Settings tab strip further down this file KEEPS its own
          `tabStripEl` wrapper — it is `variant="chip"`, not a grooved well,
          so nothing in the variant hugs for it.) */}
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
        // The BIG scale (Selector.tsx file header item 6): every segment
        // pinned to the widest one's own measured width, floored at
        // MIN_PINNED_WIDTH, at the fixed --badge-md height. Round 8 made
        // this an explicit opt-in — "well" alone is now the small,
        // content-hugging scale the CadenceBuilder mode pickers use.
        equalWidth
      />
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
  // Confirmation-pulse (GlimStone motion-engine animation 2) — this Card
  // owns its own local `persist()` rather than SettingsPage's shared save()
  // (see that function's own comment), so it needs its own pulse nonces
  // too, same per-toggle shape as shakeEnabled/shakeKeep above, bumped on
  // the OPPOSITE (`ok`, not `!ok`) branch of each toggle handler below.
  const [pulseEnabled, setPulseEnabled] = useState(0);
  const [pulseKeep, setPulseKeep] = useState(0);
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
    } else {
      setPulseEnabled((n) => n + 1);
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
    } else {
      setPulseKeep((n) => n + 1);
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
      {/* No-empty-toggles audit (jdp): this row used to `hideLabel` on the
          reasoning that the Card's own title/hint above already carry the
          same explanation, verbatim — the exact pattern jdp has now ruled
          out categorically ("es soll nie leere Toggles geben"), the same
          reversal already applied to the merged Colors Card's master
          "Regenbogen-Modus" row and RestoreChecksSection's "Automatische
          Restore-Prüfungen" row. The row's own label is visible again. */}
      <ToggleRow
        label={t("flash.zipExport.enable")}
        checked={enabled}
        onChange={(v) => void toggleEnabled(v)}
        disabled={busyEnabled}
        shakeNonce={shakeEnabled}
        pulseNonce={pulseEnabled}
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
            pulseNonce={pulseKeep}
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
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST, never as permanent page text, and
  // the button that triggered it replays `.glim-shake` once — the exact
  // pattern this component's own handleCopy/handleCopyCmd a few lines below
  // already use, and IntegrityCard's own run()/runTamperFor() established
  // for the file (see its `bumpShake` doc comment). Replaces the old
  // permanent `testMsg` paragraph next to the Test button with `shake` (a
  // bumped nonce — see ToggleRow's own shakeNonce doc comment for why).
  const [shake, setShake] = useState(0);

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
    try {
      const r = await testVMSSH();
      if (r.ok) {
        setTestState("ok");
      } else {
        setTestState("fail");
        push(r.error ?? t("vm.ssh.testFail"), "fail");
        setShake((n) => n + 1);
      }
    } catch {
      setTestState("fail");
      push(t("vm.ssh.testFail"), "fail");
      setShake((n) => n + 1);
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

  // GlimStone follow-up round (jdp, live review, five-escalations-deep
  // standing rule — "IMMER alles in die Farb- und Formengine integrieren"):
  // this Card's own two copy badges and its Test button were plain
  // `bg-carbon-surface2/3` controls with zero tie to this Card's own
  // `hueIndex` — flat regardless of rainbow mode, exactly the gap rule 1
  // exists to close. Same mechanism ToggleRow/Selector/TimePicker/Badge
  // already use: `.glim-hue` + `hueVars(rainbowAt(hueIndex))` inline,
  // computed once here and reused by every control below rather than three
  // separate near-identical blocks. On a neutral `bg-carbon-surface*`
  // control this doesn't repaint the fill (design-language.md's own
  // "icons/neutral surfaces carry no colour of their own" rule, restated at
  // [data-rainbow] .glim-hue-icon's own header in index.css) — it wires the
  // one thing that DOES apply to a neutral control, the same `--focus-ring`
  // redefinition every other `.glim-hue` element already gets, a real,
  // live-verifiable per-item colour a keyboard user actually sees on
  // Tab/click.
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

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
            {/* GlimStone follow-up round (jdp, live review: "beide
                Kopieren-Buttons sollen ein quadratischer Badge mit Glyph
                sein"): was a short-text button (`px-3 py-2 text-xs`) — the
                text is gone, an icon-only trigger needs the real
                `.glim-bubble` tooltip IconTipButton gives, not a bare
                `title=`. h-8 w-8 (32px), NOT guessed: verified live via
                getComputedStyle against this exact row's own `<code>`
                sibling (`p-2 text-xs`, renders 32px tall) — the same
                measured-not-reused discipline, and the identical 32px
                result, FolderBrowser.tsx's own "Durchsuchen" icon button
                already established for its own `px-3 py-1.5` field
                neighbour (see that file's own comment); `rounded-control`
                for the same reason given there, this button sits flush
                beside a `rounded-control` sibling.
                  COLOUR-ENGINE ROUND: both copy badges were still flat
                `bg-carbon-surface3` grey with only `.glim-hue`'s focus-ring
                redefinition tying them to this Card's position — the fill
                itself never moved, which is precisely the gap jdp's standing
                rule exists to close, and the reason the delete badge's own
                grey special-casing was removed a round earlier. Both are real
                `Badge`s now (`as="button" tone="active" shape="square"
                size="icon"`), the same construction ReplicateNowButton/
                TestConnectionButton in this file already use — an icon-only
                `tone="active"` Badge resolves to the full solid
                `bg-accent`/`text-accentContrast` fill, not the pale wash jdp
                rejected as "halb abgedunkelt". `hueIndex` is passed
                explicitly here (rather than left to inherit from the Card's
                own `.glim-hue` subtree, which would compute the identical
                colour) purely because this component already HAS the value in
                scope, matching every other hue-aware control in it. The
                hand-rolled `hueOn`/`hueStyle` pair stays — the Test button
                below still uses it. `size="icon"` is the app's one square-
                icon-badge size and is the same 32px these buttons already
                measured to, so the footprint is unchanged. */}
            <Badge
              as="button"
              tone="active"
              shape="square"
              size="icon"
              hueIndex={hueIndex}
              className="shrink-0"
              onClick={() => void handleCopy()}
              disabled={!pub}
              tip={t("vm.ssh.copy")}
            >
              <IconCopy />
            </Badge>
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
            {/* Same conversion, same measured 32px square, and the same
                colour-engine round — see the pub-key copy button above for the
                full reasoning on both. This `<pre>` sibling wraps to several
                lines (unlike the `<code>` above), but the row stays
                `items-start` so the badge never stretches to match it — it
                keeps its own fixed 32px regardless, exactly as it already did
                as a plain button before either round. */}
            <Badge
              as="button"
              tone="active"
              shape="square"
              size="icon"
              hueIndex={hueIndex}
              className="shrink-0"
              onClick={() => void handleCopyCmd()}
              disabled={!pub}
              tip={t("vm.ssh.copyCmd")}
            >
              <IconCopy />
            </Badge>
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
          {/* Button-size sweep (jdp, live review: "Die vielen Buttons sind
              unterschiedlich groß"): was `px-3 py-2` — measured live at 36px,
              taller than every sibling secondary button on this tab (Export/
              Choose file/Cancel/Logout, all `px-4 py-1.5` = 32px, the
              dominant control height this whole page already standardizes
              on — see SettingsPortabilityCard's own Export button for the
              same 32px recipe). Now matches that convention exactly, plus
              `.glim-hue` per this Card's own hueOn/hueStyle above. */}
          <button
            key={shake || 0}
            onClick={handleTest}
            disabled={testState === "testing"}
            className={`rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
              shake ? " glim-shake" : ""
            }${hueOn ? " glim-hue" : ""}`}
            style={hueStyle}
          >
            {testState === "testing" ? t("vm.ssh.testing") : t("vm.ssh.test")}
          </button>
          {testState === "ok" && (
            <span className="text-sm text-statusOk">{t("vm.ssh.testOk")}</span>
          )}
          {/* Minimal fixed glyph, matching IntegrityCard's own "ok"/"fail"
              indicator weight — the actual error text now lives in the toast
              the failure just pushed above, not a permanent inline sentence. */}
          {testState === "fail" && (
            <span className="text-sm text-statusFail">{t("vm.ssh.testFail")}</span>
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
  const { push } = useToast();
  const [includeCreds, setIncludeCreds] = useState(true);
  const [exporting, setExporting] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST, never as permanent page text, and
  // the button that triggered it replays `.glim-shake` once — the exact same
  // migration IntegrityCard/FleetSettingsCard/DashboardWidgetCard already
  // made elsewhere in this file (their own comments: "the persistent ✗
  // {error} banner... is now a toast"). Replaces the old permanent
  // exportError/importError paragraphs below each button. Export/Choose-
  // file/Import each get their own shake key so a failure in one never
  // shakes an unrelated button (same per-key nonce shape as IntegrityCard's
  // own `shake` state — see its `bumpShake` doc comment).
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  // Import is a two-step flow: pick a file → preview summary + confirm → apply.
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [importBusy, setImportBusy] = useState<"idle" | "reading" | "applying">("idle");
  const [importDone, setImportDone] = useState(false);
  // The parsed preview and the raw file text held for the confirmed apply.
  const [preview, setPreview] = useState<ImportSettingsSummary | null>(null);
  const [pendingText, setPendingText] = useState<string | null>(null);

  function resetImport() {
    setPreview(null);
    setPendingText(null);
    setImportDone(false);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  async function handleExport() {
    setExporting(true);
    // Backend-provided error text (if any) is shown verbatim BY DESIGN — the API
    // answers English and is not translated client-side.
    const err = await exportSettings(includeCreds);
    setExporting(false);
    if (err) {
      push(err, "fail");
      bumpShake("export");
    }
  }

  async function handleFilePicked(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
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
        push(res.error ?? t("settingsIO.importFailed"), "fail");
        bumpShake("chooseFile");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      bumpShake("chooseFile");
    } finally {
      setImportBusy("idle");
    }
  }

  async function handleConfirmImport() {
    if (!pendingText) return;
    setImportBusy("applying");
    try {
      const res = await importSettingsApply(pendingText);
      if (res.ok) {
        setImportDone(true);
        setPreview(null);
        setPendingText(null);
        if (fileInputRef.current) fileInputRef.current.value = "";
      } else {
        push(res.error ?? t("settingsIO.importFailed"), "fail");
        bumpShake("import");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      bumpShake("import");
    } finally {
      setImportBusy("idle");
    }
  }

  const busy = importBusy !== "idle" || exporting;

  // Button-size/colour-engine sweep (jdp, live review — see VMSSHCard's own
  // identical comment for the full reasoning): Export/Choose file/Confirm/
  // Cancel were already at this page's dominant 32px control height, but
  // none of the four carried this Card's own `hueIndex` — flat regardless of
  // rainbow mode. Same `.glim-hue` + `hueVars(rainbowAt(hueIndex))` fix.
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

  return (
    <Card title={t("settingsIO.title")} hint={t("settingsIO.desc")} hueIndex={hueIndex}>
      {/* EXPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.exportHeading")}
        </h3>
        {/* Was a raw hand-rolled <input type="checkbox"> + <label> — the last
            remaining one in this file (every earlier instance of the same
            anti-pattern was already converted to ToggleRow elsewhere on this
            page, e.g. NotifyCard's scheduledSummary/notifyOnUpdate/unraid
            rows above): a native checkbox never picks up the shape engine's
            --radius-control/--radius-pill tokens (stays the browser's native
            box shape in every round/soft/square mode) and never gets
            ToggleRow's shake/hue plumbing. Converted to the shared
            ToggleRow, matching every other toggle on this page. No
            `hueIndex` — this is a genuinely LONE toggle in this Card with no
            siblings of its own kind (ToggleRow's own hueIndex doc comment
            carves out exactly this case). */}
        <ToggleRow
          label={t("settingsIO.includeCreds")}
          checked={includeCreds}
          onChange={setIncludeCreds}
        />
        {includeCreds && (
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settingsIO.credsWarning")}
          </div>
        )}
        <button
          type="button"
          key={shake.export || 0}
          onClick={() => void handleExport()}
          disabled={busy}
          className={`self-start rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50${
            shake.export ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
          style={hueStyle}
        >
          {exporting ? t("settingsIO.exporting") : t("settingsIO.exportButton")}
        </button>
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
          key={shake.chooseFile || 0}
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          className={`self-start rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50${
            shake.chooseFile ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
          style={hueStyle}
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
                key={shake.import || 0}
                onClick={() => void handleConfirmImport()}
                disabled={busy}
                className={`rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
                  shake.import ? " glim-shake" : ""
                }${hueOn ? " glim-hue" : ""}`}
                style={hueStyle}
              >
                {importBusy === "applying" ? t("settingsIO.importing") : t("settingsIO.confirmButton")}
              </button>
              <button
                type="button"
                onClick={resetImport}
                disabled={busy}
                className={`rounded-control bg-carbon-surface3 hover:bg-carbon-border px-4 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50${hueOn ? " glim-hue" : ""}`}
                style={hueStyle}
              >
                {t("settingsIO.cancel")}
              </button>
            </div>
          </div>
        )}

        {importDone && (
          <span className="inline-flex items-center gap-1 text-xs text-statusOk">
            <CheckDraw />
            {t("settingsIO.importSuccess")}
          </span>
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
  // `output` moved to UnraidTileSection's own local `runErr` (see its doc
  // comment): this "error" kind now means only "the status CHECK itself
  // failed" (refresh()'s own catch), which never carried command output —
  // only a failed install/remove action ever did, and that no longer flows
  // through `status` at all.
  | { kind: "error"; message: string };

// UnraidTileSection — the "Unraid dashboard tile" block inside the Dashboard
// widget card: one-click install/remove of the companion bombvaultwidget plugin
// over the existing host SSH connection. Without SSH it degrades to manual
// instructions (the copyable .plg URL + a CA hint).
function UnraidTileSection({
  t,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  /** GlimStone follow-up round (jdp, live review, five-escalations-deep
   *  standing rule): this section's own Install/Remove/Retry/copy-URL
   *  buttons had no tie to the enclosing DashboardWidgetCard's hue at all —
   *  threaded straight through from that Card's own single `nextHue()` call
   *  (see its own call site below), the same "one Card, several hue-aware
   *  children share the SAME position" shape CadenceBuilder/TimePicker pairs
   *  elsewhere in this file already use, not a second independent
   *  `nextHue()` call that would land a different position for one visually-
   *  grouped Card. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;
  // `status` stays exactly as it was — GlimStone follow-up pass (v8.0.0)
  // audit note: this is a PERSISTENT "is the tile currently installed" fact
  // (plus, on failure, a possibly multi-line command `output` block), not a
  // one-shot completion notice — a poor fit for a 4s, w-80 toast, so it stays
  // as inline status rather than being replaced by one. Toast-on-save sweep
  // (jdp, live review, "a toast every time something auto-saves"): install
  // already pushed a success toast alongside this persistent status, but
  // remove's success and BOTH ops' failures didn't — an asymmetry with no
  // real justification (a failed install/remove is exactly the "auto-save
  // error" case that toast-on-save exists to always surface, quiet mode or
  // not), now fixed to match. The toast is the ephemeral "it worked/it
  // didn't" ping; `status`'s inline banner still separately carries the full
  // multi-line command `output` a toast has no room for.
  const [status, setStatus] = useState<DashPluginStatus>({ kind: "loading" });
  const [busy, setBusy] = useState<"idle" | "install" | "remove">("idle");
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): run()
  // below already pushed a fail toast (per this file's own comment above),
  // but never bumped a shake nonce. Naively bumping one alongside the
  // EXISTING `setStatus({kind:"error", ...})` call would be a no-op live —
  // that call switches the whole section to the dedicated "error" branch
  // (a plain Retry button, not Install/Remove) in the SAME React commit as
  // the shake, so the Install/Remove button the shake targets would never
  // actually paint with the class on it (caught live: a code-read alone
  // missed this, a real Playwright render against the deployed container
  // did not). `runErr` decouples "the install/remove ACTION just failed"
  // (we already know the install/absent state — that didn't change, the
  // button should stay put, shake, and show a companion error line) from
  // `status`'s own "error" kind, which now means only "the status CHECK
  // itself failed" (refresh()'s own catch, below — we genuinely don't know
  // if it's installed, hence the full-section takeover + Retry). Same
  // "keep the message off the toast's own persistent multi-line output"
  // shape this component already established for `status.output`.
  const [runErr, setRunErr] = useState<{ message: string; output?: string } | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

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
    setRunErr(null);
    try {
      const r = await (op === "install" ? installDashboardPlugin() : removeDashboardPlugin());
      if (r.ok) {
        push(op === "install" ? t("settings.dashTileInstallOk") : t("settings.dashTileRemoveOk"), "success");
        refresh();
      } else {
        const message = r.error ?? t("settings.error");
        setRunErr({ message, output: r.output });
        push(message, "fail");
        bumpShake(op);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("settings.error");
      setRunErr({ message });
      push(message, "fail");
      bumpShake(op);
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
              className={`shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast${hueOn ? " glim-hue" : ""}`}
              style={hueStyle}
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
            key={shake.install || 0}
            onClick={() => void run("install")}
            disabled={busy !== "idle"}
            className={`self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shake.install ? " glim-shake" : ""
            }${hueOn ? " glim-hue" : ""}`}
            style={hueStyle}
          >
            {busy === "install" ? t("settings.dashTileInstalling") : t("settings.dashTileInstall")}
          </button>
          {/* A failed install stays right here (the button never disappears —
              see this component's own `runErr` doc comment above for why) —
              the toast already carries the one-line message; only the
              possibly multi-line command output, when present, needs a
              spot that outlives the toast's few seconds. */}
          {runErr?.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {runErr.output}
            </pre>
          )}
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
            key={shake.remove || 0}
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            className={`self-start rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50${
              shake.remove ? " glim-shake" : ""
            }${hueOn ? " glim-hue" : ""}`}
            style={hueStyle}
          >
            {busy === "remove" ? t("settings.dashTileRemoving") : t("settings.dashTileRemove")}
          </button>
          {/* Same "stays put, shows the multi-line output only" treatment as
              the install button above. */}
          {runErr?.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {runErr.output}
            </pre>
          )}
        </div>
      )}

      {status.kind === "error" && (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-statusFail wrap-break-word">✗ {status.message}</span>
          <button
            type="button"
            onClick={() => {
              setStatus({ kind: "loading" });
              refresh();
            }}
            className={`self-start rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover${hueOn ? " glim-hue" : ""}`}
            style={hueStyle}
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
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): the
  // toast below already fired on every failure, but nothing ever bumped a
  // shake nonce, so the Generate/Regenerate/Disable buttons never played
  // `.glim-shake` — same per-key nonce shape as IntegrityCard's own `shake`
  // state (see its `bumpShake` doc comment). Generate and Disable render as
  // mutually-exclusive buttons (tokenSet ? ... : ...), so one shared key per
  // action is enough — whichever of Generate/Regenerate is on screen when
  // `generate` fails is the one that shakes.
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
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
        bumpShake("generate");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("generate");
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
        bumpShake("disable");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("disable");
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

  // Button-size/colour-engine sweep (jdp, live review — see VMSSHCard's own
  // identical comment for the full reasoning): Generate/Regenerate/Disable
  // were already at this page's dominant 32px control height but had no tie
  // to this Card's own hueIndex. Computed once, reused below AND threaded
  // into UnraidTileSection (its own children live inside THIS SAME Card, so
  // they share this Card's one rainbow position, not a second independent
  // `nextHue()` call).
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

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
              key={shake.generate || 0}
              onClick={() => void handleGenerate()}
              disabled={busy}
              className={`shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                shake.generate ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              style={hueStyle}
            >
              {t("settings.widgetRegenerate")}
            </button>
            <button
              type="button"
              key={shake.disable || 0}
              onClick={() => void handleDisable()}
              disabled={busy}
              className={`shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                shake.disable ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              style={hueStyle}
            >
              {t("settings.widgetDisable")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          key={shake.generate || 0}
          onClick={() => void handleGenerate()}
          disabled={busy}
          className={`self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake.generate ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
          style={hueStyle}
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
              {/* GlimStone follow-up round (jdp, live review: "Der Widget-
                  URL-kopieren-Button soll ein quadratischer Badge mit Glyph
                  sein"): was a short-text `bg-accent` button — reuses
                  IconCopy (Sidebar.tsx) verbatim, the same glyph VMSSHCard's
                  own two copy badges in this file already use for the
                  identical "copy this value" role. Standing rule (icon
                  badges get engine+tooltip automatically): `.glim-hue` +
                  this Card's own hueStyle wires real colour-engine
                  integration, `tip` carries the exact text the button used
                  to show (`vm.ssh.copy`, unchanged key — the same generic
                  "Kopieren" action every other copy control in this file
                  already uses, not a new one-off string). h-8 w-8 (32px) —
                  the app's ONE square-icon-badge size, identical to Badge's
                  own `size="icon"` stage and to every other square icon
                  control in the app. (This literal predates that stage and
                  happens to already be the right number; it is kept as a
                  literal only because this call site renders IconTipButton
                  directly rather than through Badge. Do not re-derive it from
                  this row's own `<code>` sibling — per-neighbour sizing is the
                  rejected split, see Badge.tsx's "ONE SIZE FOR SQUARE ICON
                  BADGES" block.) bg-accent/text-accentContrast
                  preserved from the button it replaces (this control read as
                  a primary action, unlike VMSSHCard's neutral grey copy
                  badges) — `.glim-hue` recolours that fill to this Card's
                  own rainbow position exactly like every other bg-accent
                  control in this file. */}
              <IconTipButton
                onClick={() => void handleCopy()}
                tip={t("vm.ssh.copy")}
                className={`shrink-0 inline-flex items-center justify-center rounded-control bg-accent h-8 w-8 text-accentContrast hover:opacity-90 transition-opacity${hueOn ? " glim-hue" : ""}`}
                style={hueStyle}
              >
                <IconCopy />
              </IconTipButton>
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

      {/* Companion Unraid dashboard-tile plugin (one-click install over SSH).
          hueIndex threaded straight through — its own buttons live inside
          THIS SAME Card, so they share this Card's one rainbow position, not
          a second independent nextHue() call (see its own prop doc). */}
      <UnraidTileSection t={t} hueIndex={hueIndex} />
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
  // Only the setters are needed post-conversion (see instanceName's own
  // onChange below) — save()'s toast already reports the outcome, same
  // "only setters needed" shape as SettingsPage's own setDomSaveState/
  // setDomSaveError.
  const [, setNameSaveState] = useState<SaveState>("idle");
  const [, setNameSaveError] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): the
  // toast below already fired on every failure, but nothing ever bumped a
  // shake nonce, so the Generate/Regenerate/Disable buttons never played
  // `.glim-shake` — same gap, same fix, as the sibling DashboardWidgetCard
  // above (see its own comment for the "mutually-exclusive buttons share one
  // key" reasoning, identical here).
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  const reveal = useReveal();
  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): instanceName used to batch into its own bottom SaveBar.
  // This Card is a standalone component with no access to SettingsPage's own
  // shared `debouncedSave` (only the generic `save` prop crosses that
  // boundary), so it gets its own local debounce timer — the exact same
  // local mechanism FlashZipExportCard already established for its own
  // path/keep-count fields, for the identical reason (a self-contained Card
  // that can't reach the page's own debounce helper).
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

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
        bumpShake("generate");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("generate");
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
        bumpShake("disable");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("disable");
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

  // Button-size/colour-engine sweep (jdp, live review — see VMSSHCard's own
  // identical comment for the full reasoning): Generate/Regenerate/Disable/
  // copy were already at this page's dominant 32px control height but had no
  // tie to this Card's own hueIndex.
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

  return (
    <Card title={t("settings.fleet")} hint={t("settings.fleetHint")} hueIndex={hueIndex}>

      <div className="flex flex-col gap-1.5">
        <label className="text-xs text-carbon-textSub">{t("settings.instanceName")}</label>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={settings.instanceName}
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => (prev ? { ...prev, instanceName: v } : prev));
              debounced("instanceName", () => void save({ instanceName: v }, setNameSaveState, setNameSaveError));
            }}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower"
            className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
          />
        </div>
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
              key={shake.generate || 0}
              onClick={() => void handleGenerate()}
              disabled={busy}
              className={`shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                shake.generate ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              style={hueStyle}
            >
              {t("settings.fleetRegenerate")}
            </button>
            <button
              type="button"
              key={shake.disable || 0}
              onClick={() => void handleDisable()}
              disabled={busy}
              className={`shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                shake.disable ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              style={hueStyle}
            >
              {t("settings.fleetDisable")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          key={shake.generate || 0}
          onClick={() => void handleGenerate()}
          disabled={busy}
          className={`self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake.generate ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
          style={hueStyle}
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
              className={`shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast${hueOn ? " glim-hue" : ""}`}
              style={hueStyle}
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
//
// GENUINE EXCEPTION to the full-page Speichern-Button sweep (jdp, live
// review, emphatic: "Die Speicher-Buttons sollen in allen Tabs weg. Überall
// soll es automatisch speichern. Nur dort sollen Speicher-Buttons bleiben,
// wo es unbedingt sein muss."). Every other manual Save on this page got
// converted; this one stays, for a reason specific to this field's shape,
// not "it's a text field" (debounced auto-save already handles those fine
// everywhere else): the textarea below is a WRITE-ONLY one-shot paste — it
// never round-trips the actual stored rclone.conf (`conf` starts blank on
// every load and is blanked again after a successful save; `remotes` above
// is a SEPARATE read-only summary fetched fresh from the server), so there
// is no live "current value" here to keep in sync the way autoSaveField's
// contract assumes. setRclone() also REPLACES the whole config wholesale,
// not a partial PATCH — auto-saving mid-paste/mid-edit would push a
// momentarily incomplete or invalid TOML blob live, and a scheduled off-site
// job that happens to run in that exact window would see a broken config
// instead of the working one it had a moment ago. This matches the "a
// multi-step DRAFT of something not meant to take effect until deliberately
// applied" exception named in the sweep's own criteria — it's the one
// genuine case of that shape actually present in this file.
export function RcloneCard({
  t,
  hueIndex,
  nested,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** Passed straight through to Card — see its own `nested` doc. Set by
   *  Recovery's step 3, which renders this Card inside its own step card. */
  nested?: boolean;
}) {
  const { push } = useToast();
  const [remotes, setRemotes] = useState<string[]>([]);
  const [conf, setConf] = useState("");
  const [state, setState] = useState<SaveState>("idle");
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"), found by
  // this same pass's own proactive sweep (not on the original finding list):
  // this is one of the FOUR documented genuine hold-outs on manual Save (this
  // Card's own file-header comment), the identical shape as CloudCredSetsCard's
  // save() — a fail toast already fired here, but nothing ever bumped a shake
  // nonce for the Save button. Same fix, same mechanism.
  const [shake, setShake] = useState(0);

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
        setShake((n) => n + 1);
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    }
  }

  return (
    <Card title={t("rclone.title")} hint={t("rclone.hint")} hueIndex={hueIndex} nested={nested}>
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
          key={shake || 0}
          onClick={() => void handleSave()}
          disabled={state === "saving" || conf.trim() === ""}
          className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake ? " glim-shake" : ""
          }`}
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
export function CloudCard({
  t,
  hueIndex,
  nested,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** Passed straight through to Card — see its own `nested` doc. Set by
   *  Recovery's step 3, which renders this Card inside its own step card. */
  nested?: boolean;
}) {
  const { push } = useToast();
  const [c, setC] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
  const [secretSet, setSecretSet] = useState(false);
  const [pwSet, setPwSet] = useState(false);
  // No SaveBar/button reads this back anymore post-conversion (see
  // persistPatch below) — only the setter is needed, same "only the setters
  // are needed" shape as Settings.tsx's own setDomSaveState.
  const [, setState] = useState<SaveState>("idle");
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();
  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): unlike RcloneCard right above (kept as a genuine exception
  // — see that Card's own header comment), every field here already
  // round-trips a real persisted value (getCloud below), the two secrets
  // included via the exact same "blank = keep the stored one" contract
  // Settings.tsx's own metricsToken/exportAgeRecipients fields already use
  // safely — there is no "draft not meant to take effect" shape to protect
  // here, just plain settings fields that happen to live in this Card's own
  // local state instead of the page-wide `settings` object. Same local-
  // debounce mechanism as FleetSettingsCard's own instanceName conversion
  // (this Card has no access to SettingsPage's shared debouncedSave either).
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

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

  // persistPatch merges one field's freshly-typed value onto the CURRENT `c`
  // snapshot (closed over at the point the caller scheduled it, same
  // "correct as long as one field is edited at a time" reasoning
  // debouncedSave's own callers rely on elsewhere in this file) and POSTs the
  // whole object — setCloud has no partial-patch form, unlike SettingsPage's
  // own save(). Blanking a just-saved secret mirrors handleSave's old
  // behaviour exactly, so a saved secret immediately shows as "already set"
  // without waiting for a refresh().
  async function persistPatch(patch: Partial<typeof c>) {
    setState("saving");
    const merged = { ...c, ...patch };
    try {
      const r = await setCloud(merged);
      if (r.ok) {
        setState("idle");
        if (patch.s3Secret !== undefined) {
          setC((p) => ({ ...p, s3Secret: "" }));
          if (patch.s3Secret !== "") setSecretSet(true);
        }
        if (patch.restPassword !== undefined) {
          setC((p) => ({ ...p, restPassword: "" }));
          if (patch.restPassword !== "") setPwSet(true);
        }
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

  // set — continuously-typed fields (the four text inputs): optimistic local
  // update + debounce, same shape as every other free-text field on this
  // page.
  function set<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
    debounced(String(k), () => void persistPatch({ [k]: v } as Partial<typeof c>));
  }

  // A <select> fires once per discrete pick, not per keystroke — same
  // "single discrete choice, not continuous typing" reasoning
  // autoSaveScheduleField's own header comment gives for immediate (not
  // debounced) saves, so this one saves right away instead.
  function setImmediate<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
    void persistPatch({ [k]: v } as Partial<typeof c>);
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.title")} hueIndex={hueIndex} nested={nested}>
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
          <select value={c.s3StorageClass} onChange={(e) => setImmediate("s3StorageClass", e.target.value)} className={inputCls}>
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
//
// GENUINE EXCEPTION to the full-page Speichern-Button sweep (jdp, live
// review, emphatic: "Die Speicher-Buttons sollen in allen Tabs weg... Nur
// dort sollen Speicher-Buttons bleiben, wo es unbedingt sein muss."): the
// `editing` form's own Save button stays, unlike every plain settings field
// converted elsewhere in this file. This is exactly the "multi-step DRAFT of
// something not meant to take effect until deliberately applied" shape the
// sweep's own criteria calls out: `editing` is a scratch draft (openNew()
// mints one with a random id that exists NOWHERE in `sets` yet), sitting
// beside an explicit "Close" button that discards it — auto-saving per
// keystroke would create a half-filled, visibly-listed credential set the
// instant the FIRST character of its name is typed, and would silently
// defeat Close's own "discard my edits" contract (the whole point of a
// cancel affordance). Every other row-level action here (Edit/Remove) is
// already immediate, unbatched CRUD against the same API — only this one
// scratch-draft form keeps a manual commit step.
export function CloudCredSetsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [sets, setSets] = useState<CloudCredSetInfo[]>([]);
  const [editing, setEditing] = useState<CloudCredSet | null>(null);
  const [state, setState] = useState<SaveState>("idle");
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): save()
  // and remove() below already push a fail toast but never bumped a shake
  // nonce, leaving the failure feedback toast-only instead of the toast+
  // shake pairing this session's own system-wide shake sweep (commit
  // b48fe30) established everywhere else — that commit's own file list did
  // not include Settings.tsx. Same per-key nonce shape as IntegrityCard's
  // own `shake` state (see its `bumpShake` doc comment); "save" for the
  // single shared editor panel, and `remove:${id}` per row (there can be
  // MULTIPLE saved cred sets rendered at once, unlike the single-editor
  // Save button, so a shared "remove" key would incorrectly shake every
  // row's button when only one row's delete actually failed — same
  // per-row-id keying IntegrityCard's own domain actions use).
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
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
        bumpShake("save");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("save");
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
        bumpShake(`remove:${id}`);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake(`remove:${id}`);
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
                key={shake[`remove:${s.id}`] || 0}
                onClick={() => void remove(s.id)}
                disabled={removingId === s.id}
                className={`rounded-control bg-statusFailBg px-2.5 py-1 text-xs font-medium text-statusFail hover:bg-statusFailBgHover disabled:opacity-50${
                  shake[`remove:${s.id}`] ? " glim-shake" : ""
                }`}
              >
                {removingId === s.id ? t("offsite.targets.removing") : t("offsite.targets.confirmRemove")}
              </button>
            ) : (
              <button
                type="button"
                key={shake[`remove:${s.id}`] || 0}
                onClick={() => setConfirmRemove(s.id)}
                className={`rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-statusFail hover:bg-carbon-hover${
                  shake[`remove:${s.id}`] ? " glim-shake" : ""
                }`}
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
          {/* `bg-carbon-surface3` — was `bg-carbon-surface3/40`, a 40%-alpha
              wash of the surface3 token and the ONLY place in web/src that
              ever used it (every other nested panel on this page, e.g. this
              same Card's own outer `bg-carbon-surface2` a moment ago, uses a
              plain solid surface token). A translucent wash over the Card's
              own solid surface2 background just reads as a slightly darker
              grey, not as "surface3" in any theme — the exact "translucent
              wash reads as darkened, not as the intended colour" failure
              this app's own design language already flags elsewhere. Solid
              surface3 is the correct next step up from this panel's
              surface2 parent, same nesting depth as every sibling panel. */}
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface3 p-3">
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
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface3 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
            <label className={fieldCls}>RESTIC_REST_USERNAME
              <input value={editing.restUser} onChange={(e) => setField("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
            <label className={fieldCls}>RESTIC_REST_PASSWORD
              <RevealInput {...revealRestPassword} value={editing.restPassword} onChange={(e) => setField("restPassword", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.restPasswordSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
          </div>
          <div className="flex items-center gap-3">
            <button
              key={shake.save || 0}
              onClick={() => void save()}
              disabled={state === "saving"}
              className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
                shake.save ? " glim-shake" : ""
              }`}
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
  webhookEnabled: false,
  webhookUrl: "",
  webhookFormat: "generic",
  matrixEnabled: false,
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
  appriseEnabled: false,
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
// Two hints in THIS card were ORIGINALLY left as permanent text, not
// bubbled, reasoned as reference a user consults again later rather than a
// one-time "what does this do" explainer (the spec's own test):
// notify.unraidHint names the EXACT error string ("libvirt not reachable")
// to ignore when VMs aren't backed up; notify.healthchecksLifecycle
// documents a non-obvious cross-setting interaction (Healthchecks pings
// regardless of the "notify on" policy above it). BOTH are now InfoBubbles —
// jdp's live-review ask, "Info-Texte immer in eine Infobubble," is explicit
// and unconditional, and supersedes both per-instance carve-outs the same
// way: notify.unraidHint moved in an earlier round (see its own ToggleRow
// call site's comment below for that override's full reasoning);
// notify.healthchecksLifecycle moved in the round that also added the
// per-channel enable toggles (see the Healthchecks label's own call site
// below) — each override is a one-line revert back to permanent text if the
// debugging-findability concern turns out to matter once live, not a
// redesign.
//
// GlimStone follow-up pass (v8.0.0): closed out the rest of the file's
// remainder Task 4 flagged above — every other tab's Card-level and
// field-level permanent hint <p>s are now bubbled too, on the exact same
// mechanism (Card's `hint` prop; FolderBrowser gained the identical optional
// `hint` prop for its two Settings.tsx call sites that had one). A small
// family of sites earned the SAME "reference, not a one-time explainer"
// carve-out THIS card's own two originally did (both later overridden — see
// the paragraph above): RcloneCard's pathHint and CloudCard's
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
  channelsHueIndex,
  healthchecksHueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  // The detected/overridden platform.Kind ("unraid" | "generic" | "truenas"),
  // sourced from GET /api/settings' sibling "platform" field. Drives the
  // mismatch banner below the Unraid toggle (code-review fix: a c.Unraid=true
  // + Kind()!=KindUnraid mismatch used to silently disable the feature with
  // no UI trace — see unraidGate's doc comment in internal/api/service.go for
  // why the backend gate itself stays hard rather than trusting the toggle).
  platformKind: string;
  // This function now returns THREE Cards (jdp, live-review: "Können wir die
  // Einstellungen für die Benachrichtigungen und die Kanäle für
  // Benachrichtigungen (Matrix, Apprise, Email) in zwei eigene Cards
  // auftrennen?", then a FOLLOW-UP round splitting Healthchecks out again:
  // "Sollen wir Healthchecks.io-Ping-URL und Prüfungen pro Domäne (erweitert)
  // nicht in eine eigene Card machen?") — the settings Card (trigger
  // condition, the three toggles, the platform-mismatch banner, Test), the
  // channels Card (webhook/Apprise/Matrix/Email), and the Healthchecks Card
  // (global ping URL + per-domain overrides). One shared cfg/persistNotify/
  // debounce underneath (all three Cards edit the same NotifyConfig object),
  // so this stays ONE component/one set of hooks — only the JSX splits,
  // hence three distinct hueIndex props rather than three components each
  // taking their own.
  hueIndex?: number;
  // The channels Card only renders while `advanced` is true, so THIS value
  // must come from a `nextHue()` call made INSIDE the same `tab ===
  // "notifications" && advanced &&` gate the call site guards the Card
  // itself with — never an unconditional call whose result just happens not
  // to get used. See this file's own "hueIndex={nextHue()} inside it fires
  // every render regardless" comment (the SYSTEM tab's Spike Card, and the
  // three Storage-tab Cards it references) for the exact silent-hue-shift
  // bug this guards against: an eager `nextHue()` at the call site burns a
  // slot even with Advanced off, shifting every later tab heading by one.
  channelsHueIndex?: number;
  // The Healthchecks Card (card-split follow-up) is gated identically to the
  // channels Card above (`advanced` only) — same "must come from a nextHue()
  // call made INSIDE the advanced gate" rule, same reasoning, its own
  // independent slot.
  healthchecksHueIndex?: number;
}) {
  const { push } = useToast();
  // Simple mode still gets notify-on-failure via Unraid; the extra channels
  // (webhook/Matrix/Healthchecks/SMTP) are power-user features, so gate those.
  const { advanced } = useAdvanced();
  const [cfg, setCfg] = useState<NotifyConfig>(emptyNotify);
  // Only the setter is needed post-conversion (see persistNotify below) — no
  // Speichern button reads this back anymore, same "only the setters are
  // needed" shape as Settings.tsx's own setDomSaveState.
  const [, setState] = useState<SaveState>("idle");
  // The SMTP password / Matrix token are never sent to the browser; track whether
  // one is stored so the field shows "configured" and a blank submit keeps it.
  const [secretSet, setSecretSet] = useState({ smtp: false, matrix: false });
  const revealMatrixToken = useReveal();
  const revealSmtpPassword = useReveal();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide: "Wenn
  // etwas fehlschlägt soll der Toggle/Button kurz zittern"), closing a gap
  // that survived even this Card's own full-page Speichern-Button sweep
  // (see that comment below): persistNotify already pushed a "fail" toast
  // on a rejected save but never reverted the optimistically-updated field
  // back to what the server actually still holds, nor shook it — an
  // un-reverted field showing a value that never actually saved is worse
  // than a silent failure (it reads as "saved", not "failed"). lastGoodRef
  // tracks the last CONFIRMED-persisted snapshot (the initial getNotify
  // load below, then re-stamped to the exact object `setNotify` just
  // accepted on every successful persistNotify — see that function's own
  // comment for why the WHOLE merged object, not just the touched patch
  // key, is the correct "last good" snapshot here). set()/setImmediate()
  // below both revert their own key to `lastGoodRef.current[k]` and bump
  // `fieldShake[key]` on failure — ONE shared mechanism so every one of
  // this Card's ~25 auto-save fields (8 toggles, the "on" Selector, 2
  // selects, ~15 text/number fields including the 5 per-domain Healthchecks
  // overrides) gets revert+shake for free, the same "fix the shared helper
  // once, every caller benefits" shape autoSaveField/autoSaveToggle already
  // established in SettingsPage's own save() further down this file.
  const lastGoodRef = useRef<NotifyConfig>(emptyNotify);
  const [fieldShake, setFieldShake] = useState<Partial<Record<string, number>>>({});
  function bumpShake(key: string) {
    setFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): every field below already round-trips a real persisted
  // value (getNotify below), the two secrets (matrixToken/smtpPassword)
  // included via the SAME "blank = keep the stored one" contract every other
  // secret field on this page already uses safely — no draft/cancel shape to
  // protect here, unlike RcloneCard right above. Local debounce mirrors
  // CloudCard's own mechanism (this Card is fully self-contained, no access
  // to SettingsPage's shared debouncedSave).
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  useEffect(() => {
    getNotify()
      .then((r) => {
        if (r.ok && r.notify) {
          const merged = { ...emptyNotify, ...r.notify };
          setCfg(merged);
          // The freshly-loaded server value IS the last-known-good snapshot —
          // a revert before any edit has ever been attempted rolls back to
          // exactly this, same as a fresh page load never shaking anything.
          lastGoodRef.current = merged;
        }
        setSecretSet({ smtp: !!r.smtpPasswordSet, matrix: !!r.matrixTokenSet });
      })
      .catch(() => undefined);
  }, []);

  // persistNotify merges patch onto the CURRENT cfg snapshot and POSTs the
  // whole object — setNotify has no partial-patch form, same "merge onto the
  // freshest local state" shape as CloudCard's own persistPatch (this Card
  // has no OTHER card's concurrent edits to protect against, unlike
  // SettingsPage's own baseline-merging save()). Returns whether the save
  // actually succeeded so set()/setImmediate() below can revert+shake their
  // OWN field on failure — merges the WHOLE accepted `merged` object into
  // lastGoodRef on success, not just the patch's own key: setNotify always
  // POSTs a full replace, so a successful call really did persist every
  // field `merged` carried at that moment (including another field's own
  // not-yet-separately-saved edit, if one was mid-debounce when THIS call
  // fired) — the same "the whole object is now the server's truth" fact
  // toggleDomainEnabled/autoSaveField don't need to reason about, since
  // SettingsPage's own PATCH-shaped save() never sends untouched sibling
  // fields in the first place.
  async function persistNotify(patch: Partial<NotifyConfig>): Promise<boolean> {
    setState("saving");
    const merged = { ...cfg, ...patch };
    try {
      const r = await setNotify(merged);
      if (r.ok) {
        setState("idle");
        push(t("settings.saved"), "success");
        lastGoodRef.current = merged;
        return true;
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
        return false;
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  // set — continuously-typed text/number fields: optimistic update + debounce,
  // same shape as every other free-text field on this page. Reverts THIS
  // field to its last-known-good value and bumps its own shake nonce on a
  // failed persist (see lastGoodRef's own doc comment above) — the value the
  // user sees snaps back to what the server actually still holds instead of
  // silently keeping an unsaved edit that only a toast (now gone the moment
  // it auto-dismisses) ever said didn't take.
  function set<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
    debounced(String(k), () => {
      void persistNotify({ [k]: v } as Partial<NotifyConfig>).then((ok) => {
        if (!ok) {
          setCfg((c) => ({ ...c, [k]: lastGoodRef.current[k] }));
          bumpShake(String(k));
        }
      });
    });
  }

  // setImmediate — discrete clicks (checkboxes/selects): no debounce, same
  // "single discrete choice, not continuous typing" reasoning
  // autoSaveScheduleField's own header comment gives elsewhere on this page.
  // Same revert+shake on failure as set() above — every toggle/select in
  // this Card routes through here, so this one fix covers all of them.
  function setImmediate<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
    void persistNotify({ [k]: v } as Partial<NotifyConfig>).then((ok) => {
      if (!ok) {
        setCfg((c) => ({ ...c, [k]: lastGoodRef.current[k] }));
        bumpShake(String(k));
      }
    });
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
  // selectCardCls (the Card-level sibling of selectCls above, for a select
  // sitting directly on the Card rather than a surface2 panel) was REMOVED
  // here — its one call site, the "on" select, became a Selector (task 1 of
  // this round; see that call site's own comment). No other field in this
  // Card sits directly on the Card surface rather than inside one of the
  // rounded-card bg-carbon-surface2 panels below, so nothing else needs it.
  const labelCls = "flex flex-col gap-1 text-xs text-carbon-textSub";

  return (
    <>
    <Card title={t("notify.title")} hint={t("notify.hint")} hueIndex={hueIndex}>
      {/* "on" select → Selector (jdp, live-review: "Benachrichtigen: nie, nur
          bei Fehler, bei Fehler und Erfolg soll bitte ein horizontaler
          Selektor sein"). `variant="well"` with no `equalWidth` — the SMALL
          scale of the app's one grooved selector (round 7 escalation, jdp,
          live-review: "Du hast keinen richtigen horizontalen Selektor
          gemacht!" — compared live, side by side, against the page's big
          pickers, the earlier plain `variant="chip"` default read as three
          loose separate buttons, not one real Selector control; round 8 then
          folded that round's separate "track" variant back into "well", so
          the two scales cannot drift apart again — see Selector.tsx's own
          file header item 6). Same `size="md"` scale as before — this stays
          a small 3-item control inside a settings form, literally the SAME
          variant+scale as the Integrity Card's own drill-kind Selector
          (search "drill.kindLabel" in this file, converted alongside this
          one in the same round). No `equalWidth`, so it does not take the
          Theme/Shape/Motion pickers' standardized MIN_PINNED_WIDTH box — see
          Selector.tsx's own file header for why that pinning is scoped to
          those larger pickers only.
            A plain `<span>` caption OUTSIDE the Selector, NOT the `<label>`
          this field used to be (Selector.tsx's own header is explicit: a
          `<label>` wrapping a multi-segment control hands its click to the
          first segment and announces that segment's name as the label's own
          name to a screen reader — the same trap the drill-kind Selector
          below in this file already avoids).
            Follow-up (jdp, live-review: "Benachrichtigen bei nie, Fehler,
          Erfolg und Fehler soll ein horizontaler Selektor sein (in gleicher
          Zeile wie 'Benachrichtigung')"): the Selector itself was already
          horizontal (the `select="one"` chip row above), but the wrapping
          div still used `labelCls`'s `flex flex-col gap-1` — that stacks the
          caption ABOVE the control, not beside it, which is a different ask
          than jdp meant. Swapped the wrapper to the exact
          `flex items-center gap-2 flex-wrap` shape the drill-kind Selector
          above already uses for the identical label-then-Selector-in-one-row
          layout (same file, search "drill.kindLabel") — label and Selector
          now sit on one line, wrapping only if the viewport is too narrow to
          fit both. Not `labelCls` anymore, so `text-carbon-textSub` is
          spelled out explicitly on the span to keep this field's caption the
          same color it always had (the drill-kind precedent uses
          `text-carbon-textMuted` instead, a difference already present
          between those two Cards before this change — not something to
          unify here as a drive-by). */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textSub">{t("notify.on")}</span>
        {/* key + conditional .glim-shake (revert+shake sweep, this Card's own
            `set`/`setImmediate` doc comment above): the same remount-to-
            replay mechanism every other failing control in this session
            uses, just on Selector's own `className` (which lands on its
            outer tablist wrapper, Selector.tsx's own `className` prop —
            there is no per-segment target since "on" is one control, not
            three independent ones). */}
        <Selector
          key={fieldShake.on}
          items={[
            { id: "never", label: t("notify.onNever") },
            { id: "failure", label: t("notify.onFailure") },
            { id: "always", label: t("notify.onAlways") },
          ]}
          label={t("notify.on")}
          select="one"
          // `cfg.on || "never"`, NOT bare `cfg.on` (caught live while
          // verifying the round-7 variant change above, on the running
          // test container — a stored config predating this field's
          // introduction round-trips as the empty string, not "never").
          // The backend's own Config.shouldSend (internal/notify/notify.go)
          // already documents "" as functionally identical to "never" (its
          // switch's `default:` case is literally commented "// 'never' or
          // unset") — this was purely a DISPLAY gap: the Selector had no
          // segment to light up for that legacy empty value, so all three
          // read as unselected even though the backend was correctly
          // behaving as "never" the whole time. Inside a real enclosing
          // groove, "nothing selected" reads far more like a broken control
          // than it did under the old faint plain-chip idle fill (an empty
          // groove with no accent segment anywhere in it), so this was worth
          // fixing alongside rather than shipping the more visible
          // regression. Display-only: does not touch the stored `cfg.on`
          // value or `persistNotify`'s own merge — a genuine explicit
          // "never" click still round-trips as the literal string "never",
          // this only supplies a fallback for RENDERING an already-legacy
          // empty value, never for anything this session itself writes.
          active={cfg.on || "never"}
          onChange={(id) => setImmediate("on", id)}
          variant="well"
          className={fieldShake.on ? "glim-shake" : undefined}
        />
      </div>

      {/* #56/#106 toggle sweep (jdp, live-review: "Geplante Läufe
          zusammenfassen, Bei Container-Update benachrichtigen, Unraid-
          Benachrichtigungen sollen Toggles sein"): these three used to be
          raw hand-rolled <input type="checkbox"> + <label> blocks, NOT the
          shared ToggleRow component — so they never picked up ToggleRow's
          own shake/disabled/hue plumbing the rest of this page's toggles
          get for free. Converted to ToggleRow here, matching every other
          toggle on this page. hueIndex 0/1/2: three ToggleRows rendered
          together, one per row, in the same Card — a list by construction
          regardless of whether the switches are logically independent (the
          merged Colors Card's own master/Reactive/Rotate trio and the
          Domains Card's seven rows are the identical shape; see ToggleRow's
          own `hueIndex` doc comment for the full "own local 0-based index
          per group" rule this follows). */}
      <ToggleRow
        label={t("notify.scheduledSummary")}
        hint={t("notify.scheduledSummaryHint")}
        checked={cfg.scheduledSummary}
        onChange={(v) => setImmediate("scheduledSummary", v)}
        hueIndex={0}
        shakeNonce={fieldShake.scheduledSummary}
      />
      <ToggleRow
        label={t("notify.notifyOnUpdate")}
        hint={t("notify.notifyOnUpdateHint")}
        checked={cfg.notifyOnUpdate}
        onChange={(v) => setImmediate("notifyOnUpdate", v)}
        hueIndex={1}
        shakeNonce={fieldShake.notifyOnUpdate}
      />
      {/* Unraid native notifications (delivered over the host SSH connection).
          notify.unraidHint USED to stay permanent text instead of an
          InfoBubble, reasoned as "it names the exact 'libvirt not
          reachable' error string to ignore, which needs to stay findable on
          the page for someone debugging that message, not hidden behind a
          hover target." jdp's live-review ask on this same pass was
          explicit and unconditional — "Info-Texte immer in eine
          Infobubble" — which supersedes that per-instance exception; moved
          into ToggleRow's own `hint` bubble like the two rows above. If
          that debugging-findability concern turns out to matter once this
          is live, it's a one-line revert back to permanent text, not a
          redesign. */}
      <ToggleRow
        label={t("notify.unraid")}
        hint={t("notify.unraidHint")}
        checked={cfg.unraid}
        onChange={(v) => setImmediate("unraid", v)}
        hueIndex={2}
        shakeNonce={fieldShake.unraid}
      />

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

      {/* Full-page Speichern-Button sweep: the "Speichern" button that used to
          sit beside Test is gone — every field on both this Card and the
          channels Card below now auto-saves itself (debounced text/number,
          immediate checkboxes/selects/toggles). Test keeps working exactly
          as before: it always sends the CURRENT form values (from the SAME
          shared `cfg`, whichever Card each field actually lives in), whether
          or not this Card's own 800ms debounce has already flushed them.
          Stays on THIS Card, not the channels one below, so testing (and
          Unraid notifications generally) keeps working with Advanced off —
          see this function's own header comment on `channelsHueIndex` and
          the "Simple mode still gets notify-on-failure via Unraid" comment
          on `advanced` above for why the channels Card is allowed to
          disappear but this button never is. */}
      <div className="flex items-center gap-3 pt-1 flex-wrap">
        <button onClick={() => void handleTest()}
          className="rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors">
          {t("notify.test")}
        </button>
      </div>
    </Card>

    {advanced && (
      <Card title={t("notify.channelsTitle")} hint={t("notify.channelsHint")} hueIndex={channelsHueIndex}>
      {/* Webhook (generic JSON / Discord / Slack / Gotify / ntfy). Gets a real
          enable/disable toggle (jdp, live-review: "Matrix, Apprise, Webhook-
          URL sollen alle ein Toggle bekommen wie E-Mail") — same shape as
          Email/SMTP's own smtpEnabled further below: a persisted
          `webhookEnabled` boolean that ALSO gates the backend send
          (internal/notify.Config's WebhookEnabled + its own webhookReady(),
          mirroring smtpReady()'s enabled-AND-fields-set gate exactly).
          `hueIndex={0}` (GlimStone follow-up round, jdp's live review of the
          just-hued Webhook/Matrix/Apprise toggles, explicitly OVERRIDING
          this section's prior "sole toggle in its own single-purpose
          subsection" reasoning: "Ebenso sind die Toggles bei Apprise etc.
          nicht im Regenbogenmodus") - treat Webhook/Apprise/Matrix/SMTP as
          one related GROUP (own local 0-based index per group, ToggleRow's
          own hueIndex doc), the same shape as the Rainbow Card's own three
          toggles (master/Reactive/Rotate - individually hueIndex'd despite
          sitting in visually separate blocks too). Fields hide while off,
          matching SMTP's own established pattern.
            notify.webhookChannel ("Webhook") is a NEW key distinct from
          notify.webhook ("Webhook URL", kept unchanged below on the URL
          field itself) — Matrix and Apprise already had their own
          channel-identity label (notify.matrix/notify.apprise) separate from
          their field labels; Webhook never did before this toggle needed
          one. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.webhookChannel")}
          checked={cfg.webhookEnabled}
          onChange={(v) => setImmediate("webhookEnabled", v)}
          hueIndex={0}
          shakeNonce={fieldShake.webhookEnabled}
        />
        {cfg.webhookEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.webhook")}
              <input key={fieldShake.webhookUrl} value={cfg.webhookUrl} onChange={(e) => set("webhookUrl", e.target.value)} spellCheck={false}
                placeholder="https://discord.com/api/webhooks/..." dir="ltr"
                className={`${inputCls} text-start${fieldShake.webhookUrl ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.webhookFormat")}
              <select key={fieldShake.webhookFormat} value={cfg.webhookFormat} onChange={(e) => setImmediate("webhookFormat", e.target.value)}
                className={`${selectCls}${fieldShake.webhookFormat ? " glim-shake" : ""}`}>
                <option value="generic">Generic JSON</option>
                <option value="discord">Discord</option>
                <option value="slack">Slack</option>
                <option value="gotify">Gotify</option>
                <option value="ntfy">ntfy</option>
              </select>
            </label>
          </>
        )}
      </div>

      {/* Apprise API: posts to a user-run apprise-api server, unlocking Apprise's
          100+ services without bundling Python. Auto-saves + shares the card's
          Test button like the other channels (full-page Speichern-Button
          sweep — see this Card's own header comment).
            Same real enable/disable toggle as Webhook above (`appriseEnabled`
          — internal/notify.Config's AppriseEnabled + appriseReady()). The
          section's own former header `<span>` + InfoBubble is GONE, folded
          into this ToggleRow's own `hint` prop instead — the exact mechanism
          notify.unraidHint's own override already uses (see this function's
          header comment) — rather than showing "Apprise" twice (once as a
          plain caption, once as the toggle's own required visible label).
          `hueIndex={1}` - see Webhook's own comment above for jdp's
          explicit override of the old "sole toggle in its own subsection"
          reasoning; same Webhook/Apprise/Matrix/SMTP group, next slot. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.apprise")}
          hint={t("notify.appriseHint")}
          checked={cfg.appriseEnabled}
          onChange={(v) => setImmediate("appriseEnabled", v)}
          hueIndex={1}
          shakeNonce={fieldShake.appriseEnabled}
        />
        {cfg.appriseEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.appriseUrl")}
              <input key={fieldShake.appriseUrl} value={cfg.appriseUrl} onChange={(e) => set("appriseUrl", e.target.value)} spellCheck={false}
                placeholder="http://apprise:8000/notify/bombvault" dir="ltr"
                className={`${inputCls} text-start${fieldShake.appriseUrl ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.appriseTags")}
              <input key={fieldShake.appriseTags} value={cfg.appriseTags} onChange={(e) => set("appriseTags", e.target.value)} spellCheck={false}
                placeholder="backups,homelab" className={`${inputCls}${fieldShake.appriseTags ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>

      {/* Matrix room push. Same real enable/disable toggle as Webhook/Apprise
          above (`matrixEnabled` — internal/notify.Config's MatrixEnabled,
          folded into matrixReady()'s existing homeserver/token/room gate).
          The section's former plain `<span>` header is gone, replaced by
          this ToggleRow's own required visible label (no hint text existed
          for Matrix to fold in, unlike Apprise above). `hueIndex={2}` - see
          Webhook's own comment above for jdp's explicit override; same
          Webhook/Apprise/Matrix/SMTP group, next slot. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.matrix")}
          checked={cfg.matrixEnabled}
          onChange={(v) => setImmediate("matrixEnabled", v)}
          hueIndex={2}
          shakeNonce={fieldShake.matrixEnabled}
        />
        {cfg.matrixEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.matrixHomeserver")}
              <input key={fieldShake.matrixHomeserver} value={cfg.matrixHomeserver} onChange={(e) => set("matrixHomeserver", e.target.value)} spellCheck={false}
                placeholder="https://matrix.org" dir="ltr"
                className={`${inputCls} text-start${fieldShake.matrixHomeserver ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.matrixToken")}
              <RevealInput {...revealMatrixToken} key={fieldShake.matrixToken} value={cfg.matrixToken} onChange={(e) => set("matrixToken", e.target.value)} spellCheck={false}
                placeholder={secretSet.matrix ? t("cloud.secretSet") : ""} wrapperClassName="w-full"
                className={`${inputCls}${fieldShake.matrixToken ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.matrixRoom")}
              <input key={fieldShake.matrixRoom} value={cfg.matrixRoom} onChange={(e) => set("matrixRoom", e.target.value)} spellCheck={false}
                placeholder="!abcdef:matrix.org" dir="ltr"
                className={`${inputCls} text-start${fieldShake.matrixRoom ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>

      {/* Email (SMTP), sent via the configured mail server. Same "same
          component/area, same raw-checkbox anti-pattern" gap as the three
          settings-Card toggles converted in an earlier round (standing rule:
          fix every existing caller of the gap being fixed, not just the
          reported instance) — this was still a hand-rolled
          <input type="checkbox">, the fourth one in this Card.
          `hueIndex={3}` (GlimStone follow-up round, jdp's explicit override of
          the "sole toggle in its own subsection" reasoning that used to sit
          here — see Webhook's own comment above for the full history):
          Webhook/Apprise/Matrix/SMTP are one related GROUP now, four
          independent channel subsections each getting their own stable
          rainbow position (0/1/2/3), not four un-hued singletons.
            MOVED directly above Healthchecks below (jdp, live-review: "E-Mail
          soll über Healthchecks-Ping-URL verschoben werden") — was the LAST
          section in this Card; the new order is Webhook/Apprise/Matrix/
          Email, with Healthchecks(-global+per-domain) now its OWN separate
          Card below (card-split follow-up) rather than a trailing section
          here. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.smtp")}
          checked={cfg.smtpEnabled}
          onChange={(v) => setImmediate("smtpEnabled", v)}
          hueIndex={3}
          shakeNonce={fieldShake.smtpEnabled}
        />
        {cfg.smtpEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.smtpHost")}
              <input key={fieldShake.smtpHost} value={cfg.smtpHost} onChange={(e) => set("smtpHost", e.target.value)} spellCheck={false}
                placeholder="smtp.example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpHost ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPort")}
              <input key={fieldShake.smtpPort} value={cfg.smtpPort} onChange={(e) => set("smtpPort", Number(e.target.value) || 0)} spellCheck={false}
                type="number" placeholder="587" className={`${inputCls}${fieldShake.smtpPort ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTls")}
              <select key={fieldShake.smtpTls} value={cfg.smtpTls} onChange={(e) => setImmediate("smtpTls", e.target.value)}
                className={`${selectCls}${fieldShake.smtpTls ? " glim-shake" : ""}`}>
                <option value="starttls">STARTTLS</option>
                <option value="tls">TLS (implicit)</option>
                <option value="none">None</option>
              </select>
            </label>
            <label className={labelCls}>
              {t("notify.smtpUser")}
              <input key={fieldShake.smtpUsername} value={cfg.smtpUsername} onChange={(e) => set("smtpUsername", e.target.value)} spellCheck={false}
                dir="ltr" className={`${inputCls} text-start${fieldShake.smtpUsername ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPass")}
              <RevealInput {...revealSmtpPassword} key={fieldShake.smtpPassword} value={cfg.smtpPassword} onChange={(e) => set("smtpPassword", e.target.value)} spellCheck={false}
                placeholder={secretSet.smtp ? t("cloud.secretSet") : ""} wrapperClassName="w-full"
                className={`${inputCls}${fieldShake.smtpPassword ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpFrom")}
              <input key={fieldShake.smtpFrom} value={cfg.smtpFrom} onChange={(e) => set("smtpFrom", e.target.value)} spellCheck={false}
                placeholder="bombvault@example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpFrom ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTo")}
              <input key={fieldShake.smtpTo} value={cfg.smtpTo} onChange={(e) => set("smtpTo", e.target.value)} spellCheck={false}
                placeholder="admin@example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpTo ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>
      </Card>
    )}

    {/* Healthchecks Card (card-split follow-up, jdp: "Sollen wir
        Healthchecks.io-Ping-URL und Prüfungen pro Domäne (erweitert) nicht in
        eine eigene Card machen?") — SPLIT OUT of the channels Card above into
        its own standalone Card, own heading, own hueIndex, matching this
        file's established per-topic Card-splitting pattern (the same move
        NotifyCard's own header comment documents for the settings/channels
        split one round earlier). Same `advanced &&` gate as the channels
        Card (Healthchecks is itself an advanced-only feature, same as
        Webhook/Matrix/Apprise/SMTP above it), same shared cfg/persistNotify.
          Neither section's own JSX changed beyond the wrapping Card — global
        URL + per-domain overrides render exactly as before. */}
    {advanced && (
      <Card title={t("notify.healthchecksTitle")} hueIndex={healthchecksHueIndex}>
      {/* Healthchecks global ping URL.
            notify.healthchecksLifecycle moved from a permanent <p> into an
          InfoBubble attached to this label (jdp, live-review, explicit and
          unconditional: "der Infotext dort in eine Infobubble" — overriding
          the prior "must stay findable on-page for someone debugging an
          unexpected check status" reasoning this Card's own header comment
          used to document verbatim; see that comment for the full
          before/after). One-line revert back to a permanent `<p>` if that
          debugging-findability concern turns out to matter once live.
            InfoBubble nested INSIDE the `<label>`, wrapped in its own
          `<span className="flex items-center gap-1">` — the SAME established
          pattern this file already uses at "cloud.storageClass.label" and
          "flash.zipExport.keepN" (an InfoBubble is a real interactive
          `<button>`, so clicking it activates the bubble, not the label's
          associated `<input>` — verified live against those existing call
          sites, not a new mechanism invented for this one). */}
      <label className={labelCls}>
        <span className="flex items-center gap-1">
          {t("notify.healthchecks")}
          <InfoBubble tip={t("notify.healthchecksLifecycle")} />
        </span>
        <input key={fieldShake.healthchecksUrl} value={cfg.healthchecksUrl} onChange={(e) => set("healthchecksUrl", e.target.value)} spellCheck={false}
          placeholder="https://hc-ping.com/your-uuid" className={`${inputCls}${fieldShake.healthchecksUrl ? " glim-shake" : ""}`} />
      </label>

      {/* Per-domain Healthchecks overrides (advanced). A blank field falls back to the global URL above.
          Revert+shake sweep: this field bypasses set()/setImmediate() (it PATCHes
          one entry of a nested Record, not a top-level NotifyConfig key), so it
          gets its own bespoke copy of the SAME lastGoodRef-revert + bumpShake
          shape those two helpers share — reverting only THIS domain's own map
          entry, not the whole healthchecksByDomain object, so an unrelated
          domain's already-good override never gets clobbered by this one
          failing. Shake keyed on the SAME `hc-${key}` string already used for
          the debounce above, not a second name for the identical field. */}
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
              key={fieldShake[`hc-${key}`]}
              value={cfg.healthchecksByDomain?.[key] ?? ""}
              onChange={(e) => {
                const v = e.target.value;
                const nextMap = { ...cfg.healthchecksByDomain, [key]: v };
                setCfg((c) => ({ ...c, healthchecksByDomain: nextMap }));
                debounced(`hc-${key}`, () => {
                  void persistNotify({ healthchecksByDomain: nextMap }).then((ok) => {
                    if (!ok) {
                      setCfg((c) => ({
                        ...c,
                        healthchecksByDomain: {
                          ...c.healthchecksByDomain,
                          [key]: lastGoodRef.current.healthchecksByDomain?.[key] ?? "",
                        },
                      }));
                      bumpShake(`hc-${key}`);
                    }
                  });
                });
              }}
              spellCheck={false}
              placeholder="https://hc-ping.com/your-uuid"
              dir="ltr"
              className={`${inputCls} text-start${fieldShake[`hc-${key}`] ? " glim-shake" : ""}`}
            />
          </label>
        ))}
      </div>
      </Card>
    )}
    </>
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
//
// GlimStone follow-up round (jdp, live review of the just-hued text badges:
// "Die Buttons Verbindung testen, Jetzt replizieren, Einrichten, Ziel
// hinzufügen haben farbige Schrift. Können wir die Buttons in quadratische
// Badges mit Glyphen umwandeln?"): the visible "Jetzt replizieren…"/
// "Replizierend…" text is gone, replaced by IconSync (Sidebar.tsx) inside a
// square (`shape="square"`), icon-only (`tip` set — see Badge.tsx's own `tip`
// doc) Badge at `size="icon"` — the app's ONE square-icon-badge size (32px).
// These four off-site badges were `size="field"` (36px), pinned to the
// repo-url `<input>`'s own measured height; sizing each icon badge to its own
// nearest neighbour is exactly the role-based split jdp rejected, so the
// per-neighbour number is gone (see Badge.tsx's "ONE SIZE FOR SQUARE ICON
// BADGES" block). The old busy/idle text swap survives
// as the tooltip's own content instead of the button's visible label — same
// two i18n keys, same swap condition, just read by `tip` instead of
// `children` now. GlimStone follow-up round (jdp's next live review, on
// this same badge: "die sind falsch eingefaerbt, so halb abgedunkelt")
// replaced the `hueIndex`-driven background from a 14%-alpha wash to a full
// solid hue-derived fill — Badge.tsx's own `isIconOnly && tone==="active"`
// branch now swaps the glyph's ink to the computed-contrast
// `text-accentContrast` riding on that solid fill: still a neutral
// (hue-of-its-own-free) ink per "icons carry no colour of their own, only
// the badge does", just no longer the flat `text-carbon-textSub` token that
// was only safe against the old pale wash. See Badge.tsx's own `toneClasses`
// comment for the full before/after and the measured contrast numbers.
function ReplicateNowButton({
  domain,
  t,
  hueIndex,
}: {
  domain: "containers" | "vms" | "flash" | "files";
  t: ReturnType<typeof useT>["t"];
  /** Offsite-tab card-split follow-up (jdp: "Die Buttons Verbindung testen,
   *  Jetzt replizieren, Einrichten, Ziel hinzufügen in die Farbengine
   *  aufnehmen"): this button's own enclosing per-domain offsite Card's hue
   *  position, the SAME value that Card's own `hueIndex` already got — not a
   *  second independent value, matching every other "thread the enclosing
   *  Card's own hueIndex straight through" call site in this file (e.g.
   *  ContainersSection's CadenceBuilder). `tone="active"` below (not
   *  "neutral", this button's old plain-grey identity) is what makes a
   *  passed hueIndex actually visible — see Badge.tsx's own `hueOn` comment
   *  for why "active" is the one non-heading tone `hueIndex` drives. Still
   *  true after the icon-badge conversion above: `tone` only ever governed
   *  the background wash, never the (now-neutral) glyph ink. */
  hueIndex?: number;
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
    <Badge
      as="button"
      tone="active"
      shape="square"
      size="icon"
      hueIndex={hueIndex}
      onClick={() => void go()}
      disabled={busy}
      tip={busy ? t("offsite.replicating") : t("offsite.replicateNow")}
    >
      <IconSync />
    </Badge>
  );
}

// TestConnectionButton probes a domain's off-site repo (reachable / initialised)
// without modifying it, showing the verdict inline — so the user can verify the
// configured location before relying on it.
// GlimStone follow-up round: converted to a square icon-only badge (IconCheckCircle)
// the same way as ReplicateNowButton above — see that function's own comment for
// the full "coloured text -> neutral glyph, wash -> solid fill" writeup;
// the multiTarget-dependent "Test connection"/"Test PRIMARY connection" swap
// survives unchanged, just as `tip` content instead of visible text.
function TestConnectionButton({
  domain,
  t,
  hueIndex,
}: {
  domain: "containers" | "vms" | "flash" | "files";
  t: ReturnType<typeof useT>["t"];
  /** See ReplicateNowButton's own doc above — identical offsite-tab
   *  card-split follow-up, same enclosing Card's hueIndex threaded through,
   *  same tone="active" reasoning. */
  hueIndex?: number;
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
    <Badge
      as="button"
      tone="active"
      shape="square"
      size="icon"
      hueIndex={hueIndex}
      onClick={() => void go()}
      disabled={busy}
      tip={multiTarget ? t("offsite.testPrimary") : t("offsite.test")}
    >
      <IconCheckCircle />
    </Badge>
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
  const { push } = useToast();
  type ActState = "idle" | "busy" | "ok" | "fail";
  type DrillKind = "subset" | "dr";
  const [state, setState] = useState<Record<string, ActState>>({});
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST (push below), never as permanent
  // page text, and the button that triggered it replays `.glim-shake` once.
  // `msg` (a per-key error STRING kept around to render inline forever) is
  // gone — the toast already carries the message the instant the action
  // fails, so nothing needs to remember it past that point. `shake` replaces
  // it: a per-key nonce, same shape, bumped on every failure so a fresh
  // `.glim-shake` class + key remount fires even for the SAME button failing
  // twice in a row (see ToggleRow's own shakeNonce doc comment for why a
  // bumped number, not a boolean, is required here).
  const [shake, setShake] = useState<Record<string, number>>({});
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

  // bumpShake replays `.glim-shake` on the button keyed `key` (see the
  // `shake` state's own doc comment above) — called from every failure branch
  // below alongside the toast `push()` that now carries the message instead
  // of a permanent inline sentence.
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

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
        // NOTE: a decisive "not protected" verdict (testable && !protected) is
        // NOT a failed action — the test ran fine and truthfully reported bad
        // news, exactly like a red health badge. That stays a persistent inline
        // verdict (rendered below), not a toast — only the "error" branches
        // below (the test itself couldn't run) are the "you clicked it, it
        // didn't work" case the toast+shake standard targets.
      } else {
        const message = r.error ?? t("offsite.tamperError");
        setTamper((m) => ({ ...m, [domain]: { kind: "error", message } }));
        push(message, "fail");
        bumpShake(`${domain}:tamper`);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("offsite.tamperError");
      setTamper((m) => ({ ...m, [domain]: { kind: "error", message } }));
      push(message, "fail");
      bumpShake(`${domain}:tamper`);
    }
  }

  async function run(domain: Domain, action: Action) {
    if (action === "prune" && !(await confirm(t("integrity.pruneConfirm")))) return;
    const key = `${domain}:${action}`;
    setState((s) => ({ ...s, [key]: "busy" }));
    try {
      const r =
        action === "verify" ? await checkDomain(domain, source)
        : action === "unlock" ? await unlockDomain(domain, source)
        : await pruneDomain(domain, source);
      if (r.ok) {
        setState((s) => ({ ...s, [key]: "ok" }));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        push(r.error ?? t("integrity.failed"), "fail");
        bumpShake(key);
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      push(err instanceof Error ? err.message : t("integrity.failed"), "fail");
      bumpShake(key);
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
    try {
      const r = await runDrill(domain, kind === "dr" ? "offsite" : source, kind);
      if (r.ok && r.drill) {
        const drill = r.drill;
        setLastDrill((m) => ({ ...m, [domain]: drill }));
        setState((s) => ({ ...s, [key]: drill.ok ? "ok" : "fail" }));
        if (!drill.ok) {
          push(drill.detail || t("verify.failed"), "fail");
          bumpShake(key);
        }
        // A recorded drill (pass OR fail) changes the shared /api/status the
        // dashboard scorecard reads. Broadcast so the Dashboard refetches its
        // drill / "proven restorable" pills without a page reload — mirrors how
        // saving settings signals the app to refresh.
        window.dispatchEvent(new Event("bv:settings-changed"));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        push(r.error ?? t("verify.failed"), "fail");
        bumpShake(key);
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      push(err instanceof Error ? err.message : t("verify.failed"), "fail");
      bumpShake(key);
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
            setLastDrill({});
          }}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* Drill-type toggle: subset integrity check vs a real off-site DR
          restore — on the shared Selector component (GlimStone form-engine
          Phase 2, Task 3; found only by re-grepping the current codebase,
          not on the original Phase 1 audit's own 11-site list).
          `variant="well"` with no `equalWidth` — the small scale of the app's
          one grooved selector (round 7 escalation) — converted alongside
          NotifyCard's "on" Selector in the same round; see that call site's
          own comment for the full root-cause writeup. Both are the same
          "small in-card single-choice" role at the same scale, so both share
          the literal same variant+props, not just an eyeballed match. */}
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
          }}
          variant="well"
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
              {/* Domain actions + the restore-verification drill on ONE row
                  (jdp, live-review: "Die Buttons bei Container, VMs, etc
                  sollen alle in einer Zeile stehen") — was two separate flex
                  rows (verify/unlock/prune, then a second row for the drill/
                  DR-run button behind an empty w-24 spacer just to align
                  under the first row's buttons). The label alone already
                  anchors the whole row's left edge now that everything sits
                  on it, so that spacer is gone along with the second row;
                  every button below keeps its exact behavior, disabled state
                  and inline busy/ok/fail feedback — this is a pure layout
                  merge, no logic changed. */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm text-carbon-textSub w-24 shrink-0">{label}</span>
                {actions.map((a) => {
                  const k = `${domain}:${a.key}`;
                  return (
                    <span key={a.key} className="inline-flex items-center gap-1">
                      <button
                        key={shake[k] || 0}
                        onClick={() => void run(domain, a.key)}
                        disabled={state[k] === "busy"}
                        title={t(`integrity.${a.key}Hint`)}
                        className={`rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                          shake[k] ? " glim-shake" : ""
                        }`}
                      >
                        {state[k] === "busy" ? a.busy : a.label}
                      </button>
                      {state[k] === "ok" && (
                        <span className="inline-flex items-center gap-1 text-sm text-statusOk">
                          <CheckDraw />
                          {t("integrity.ok")}
                        </span>
                      )}
                      {/* Minimal fixed glyph, matching the "ok" indicator's OWN
                          visual weight — the actual error text now lives in the
                          toast the failure just pushed (jdp, live review,
                          emphatic standing rule: toast on failure, not a
                          permanent inline sentence; see run()'s own comment). */}
                      {state[k] === "fail" && <span className="text-sm text-statusFail">{t("integrity.failedShort")}</span>}
                    </span>
                  );
                })}
                <button
                  key={shake[dKey] || 0}
                  onClick={() => void runDrillFor(domain)}
                  disabled={state[dKey] === "busy"}
                  title={kind === "dr" ? t("drill.drNote") : t("verify.hint")}
                  className={`rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                    shake[dKey] ? " glim-shake" : ""
                  }`}
                >
                  {state[dKey] === "busy"
                    ? kind === "dr" ? t("drill.runningDR") : t("verify.running")
                    : kind === "dr" ? t("drill.runDR") : t("verify.now")}
                </button>
                {state[dKey] === "ok" && (
                  <span className="inline-flex items-center gap-1 text-sm text-statusOk">
                    <CheckDraw />
                    {t("verify.ok")}
                  </span>
                )}
                {/* Same minimal-glyph treatment as the action buttons above —
                    the raw error/detail text went to the toast when the
                    failure happened, not into a permanent inline sentence. */}
                {state[dKey] === "fail" && (
                  <span className="text-sm text-statusFail">✗ {t("verify.failed")}</span>
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
                    key={shake[`${domain}:tamper`] || 0}
                    onClick={() => void runTamperFor(domain)}
                    disabled={tRes?.kind === "busy"}
                    title={t("integrity.appendOnlyHint")}
                    className={`rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
                      shake[`${domain}:tamper`] ? " glim-shake" : ""
                    }`}
                  >
                    {tRes?.kind === "busy" ? t("integrity.checking") : t("integrity.appendOnly")}
                  </button>
                  {tRes?.kind === "verdict" && (
                    <span
                      className={`text-sm wrap-break-word ${
                        !tRes.testable ? "text-statusWarn" : tRes.protected ? "text-statusOk" : "text-statusFail"
                      }`}
                    >
                      {tRes.testable && (
                        <span aria-hidden="true" className="inline-flex items-center">
                          {tRes.protected ? <CheckDraw /> : "✗"}&nbsp;
                        </span>
                      )}
                      {!tRes.testable
                        ? t("offsite.tamperUnverifiable")
                        : tRes.protected
                          ? t("offsite.tamperOk")
                          : t("offsite.tamperFail")}
                    </span>
                  )}
                  {/* The test itself failing to run (network/server error) is the
                      "you clicked it, it didn't work" case — the toast pushed by
                      runTamperFor carries the real message now, so this stays a
                      fixed, short caption instead of the raw (possibly long)
                      backend text living on the page forever. A decisive
                      "not protected" VERDICT above is a different, legitimate
                      persistent status and is untouched. */}
                  {tRes?.kind === "error" && (
                    <span className="text-sm text-statusFail">{t("offsite.tamperError")}</span>
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
// cadenceLabel / ScheduleStatus / scheduleStatus / SCHEDULE_BADGE_TONE /
// ScheduleBadge all MOVED to components/ScheduleBadge.tsx this round, verbatim
// — plus a new `ScheduleRow` that owns the "Zeitplan: [badge]" line the four
// domain Cards and Selbst-Backup each used to hand-roll below. See that file's
// own header for why (CadenceBuilder's redundant inline preview could only be
// deleted once EVERY cadence editor was guaranteed to show its resolved
// schedule above, and ItemScheduleOverride needed the badge too — which an
// export from this file could not give it without an import cycle, since this
// file already imports ItemScheduleOverride).

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
  // Exclude BombVault's own container: it can never be backed up, so it must
  // never appear as a schedule member even if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    // Live-review round 5 REVERSES the previous round's "Bei allen
    // Zeitplanpicker Cards soll der Name raus... das ist redundant" removal
    // (jdp: "Den Text in die Cardtitelbadges wieder einfügen, den habe ich
    // nicht gemeint. Den 'Titeltext' aus der Zeitplancard entfernen" — put
    // the heading BADGE text back; the duplicate jdp actually meant was the
    // plain-text <legend> INSIDE CadenceBuilder, fixed there instead — see
    // CadenceBuilder.tsx's own header comment for that half of this
    // correction). `title` restored; `hint` stays alongside it exactly as
    // every other title+hint Card in this file already composes both.
    <Card title={t("jobs.containersSection")} hint={t("containers.scheduleHint")} hueIndex={hueIndex}>
      {/* Cadence row */}
      <ScheduleRow schedule={schedule} />

      {/* Editable cadence builder. `hueIndex` passed straight through — the
          SAME position this Card's own heading notch already got above, not
          a second independent value — so the TimePicker inside picks up this
          Card's own stable rainbow colour (Task 3, jdp: "Der Zeitpicker ist
          nicht im Regenbogenmodus"; see CadenceBuilder's own hueIndex doc). */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
          hueIndex={hueIndex}
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
  const included = vms.filter((v) => v.includeInSchedule);

  return (
    // See ContainersSection's own comment above — `title` restored, same
    // Task 3 `hueIndex` threaded into CadenceBuilder below.
    <Card title={t("jobs.vmsSection")} hint={t("jobs.vmIncludeHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.vmsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
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

  return (
    // See ContainersSection's own comment above — `title` restored
    // (jobs.flashScheduleHint's own text, added when the title was dropped,
    // stays too: unlike Containers/VMs/Folders, Flash has no per-item member
    // list, so the hint states what a Flash backup actually covers rather
    // than explaining a list). Same Task 3 `hueIndex` threaded into
    // CadenceBuilder below.
    <Card title={t("jobs.flashSection")} hint={t("jobs.flashScheduleHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
        {/* GlimStone follow-up pass: stays permanent text, NOT bubbled — a
            behavioural caveat ("this control looks live but silently does
            nothing yet") someone hits while confused about why a saved
            Flash schedule never runs, not a one-time "what does this do"
            explainer. Same carve-out category as notify.healthchecksLifecycle
            above (NotifyCard's own header comment).
              Live-review round (jdp, Task 7 — "Wieso steht in der Flash
            Zeitplan Card die Zeile mit dem Text 'Unraid Flash-
            Konfiguration'? Kann das nicht weg?"): that was a SEPARATE
            trailing "member row" below this paragraph (dot + name +
            "planned" status, styled like a ContainersSection/VMsSection
            member-list row) — removed outright, along with its now-orphaned
            jobs.flashRow/jobs.flashPlanned keys, since Flash has no actual
            per-item collection to list and the row conveyed nothing this
            paragraph doesn't already say. THIS paragraph is deliberately
            kept: jdp's own task description explicitly carves out "the Flash
            cadence itself not being wired to a real backend job yet" as the
            still-true, genuinely-different case — which is exactly what this
            says (the executor isn't implemented), as opposed to the removed
            row's purely decorative, redundant "planned" restatement of the
            same fact. */}
        {!syncSchedules && (
          <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.flashNotImplemented")}</p>
        )}
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
  syncSchedules,
  fileSets,
  onChange,
  onSetsChanged,
  t,
  hueIndex,
}: {
  settings: Settings;
  /** #? — "Container-Zeitplan auch für VMs, Flash und Ordner verwenden"
   *  (jdp, live-review): Folders now follows the same sync toggle VMs/Flash
   *  already had, mirroring their exact pattern below (schedule resolves to
   *  the Containers cadence while synced, its own CadenceBuilder disabled
   *  meanwhile). Previously this section had no syncSchedules concept at
   *  all and always used its own independent settings.filesSchedule. */
  syncSchedules: boolean;
  fileSets: FileSetView[];
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set — reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.filesSchedule;
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
    // See ContainersSection's own comment above — `title` restored, same
    // Task 3 `hueIndex` threaded into CadenceBuilder below.
    <Card title={t("jobs.filesSection")} hint={t("jobs.filesIncludeHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.filesSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
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
              {/* No-empty-toggles audit (jdp): this row used to `hideLabel`
                  with no visible caption anywhere in the row at all — worse
                  than the Card-title-redundant pattern found elsewhere, since
                  there wasn't even a duplicate label to point to, only the
                  set's own NAME (which identifies the row, not what the
                  switch does). Wrapped in the same `<label>` + sibling
                  `<span>` shape VMIncludeToggle/FileSetEnabledToggle already
                  use for a per-row switch: `hideLabel` stays on the bare
                  Toggle (legitimate here — the caller right beside it now
                  draws the same text), but the text is genuinely visible in
                  the row, always, not just conveyed via aria-label. */}
              <label className="flex items-center gap-2 shrink-0 cursor-pointer">
                <span className="text-xs text-carbon-textSub">{t("files.enabled")}</span>
                <Toggle
                  hideLabel
                  label={`${t("files.enabled")}: ${s.name}`}
                  checked={s.enabled}
                  onChange={() => void toggle(s)}
                  disabled={!!busy[s.id]}
                />
              </label>
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
  busy,
  shake,
  pulse,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  /** Task 5 auto-save (no Speichern button on this tab anymore): busy/shake
   *  feedback for this Card's two plain booleans, keyed the same way
   *  domainToggleBusy/domainToggleShake and mergedFieldBusy/mergedFieldShake
   *  already are elsewhere — see SettingsPage's own autoSaveScheduleField. */
  busy?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", boolean>>;
  shake?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", number>>;
  /** Confirmation-pulse (GlimStone motion-engine animation 2) — same shape
   *  as `shake` above, opposite outcome. SettingsPage passes its own shared
   *  `fieldPulse` map straight through (see that state's own declaration
   *  comment next to save()) — this narrower prop type is still satisfied
   *  because `fieldPulse` is keyed by the full `keyof Settings`, a superset
   *  of the two keys this Card actually reads. */
  pulse?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", number>>;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  return (
    <Card title={t("verify.auto")} hint={t("verify.hint")} hueIndex={hueIndex}>
      {/* Task 4 (jdp, live-review: "Bei erstem Toggle bitte 'Automatische
          Restore-Prüfungen' hinschreiben") — `hideLabel` removed: an earlier
          round hid this row's own caption on the reasoning that the Card's
          own title above already says the same thing (the same
          single-purpose-Card pattern the master Regenbogen-Modus toggle used
          too), but jdp reversed that exact pattern there as well and wants
          the label visible directly on the toggle here too. */}
      <ToggleRow
        label={t("verify.auto")}
        checked={settings.drillsEnabled}
        onChange={(v) => update({ drillsEnabled: v })}
        disabled={busy?.drillsEnabled}
        shakeNonce={shake?.drillsEnabled}
        pulseNonce={pulse?.drillsEnabled}
      />
      {/* Sub-toggle: only meaningful while scheduled drills are on. ToggleRow
          itself dims its switch AND its caption/description together — no
          wrapping container opacity needed here. */}
      <ToggleRow
        label={t("settings.offsiteDrills")}
        hint={t("settings.offsiteDrillsHelp")}
        checked={settings.offsiteDrillsEnabled}
        disabled={!settings.drillsEnabled || busy?.offsiteDrillsEnabled}
        onChange={(v) => update({ offsiteDrillsEnabled: v })}
        shakeNonce={shake?.offsiteDrillsEnabled}
        pulseNonce={pulse?.offsiteDrillsEnabled}
      />
      {/* Resolved-schedule badge — NEW this round. This Card was one of the
          three cadence editors with nothing above it, so CadenceBuilder's own
          inline preview paragraph was the only place its resolved schedule
          was shown; deleting that paragraph (see CadenceBuilder.tsx) without
          adding this row would have lost information rather than removed a
          duplicate. `enabled` is wired to `drillsEnabled` because THIS card's
          on/off lives in a separate toggle rather than in the cadence string's
          own "off" mode — see ScheduleRow's own `enabled` doc. */}
      <ScheduleRow schedule={settings.drillsSchedule} enabled={settings.drillsEnabled} />
      {/* `hueIndex` passed straight through to the TimePicker inside (Task 3
          fix) — the SAME position as this Card's own heading notch above. */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("settings.schedule")}
          value={settings.drillsSchedule}
          disabled={!settings.drillsEnabled}
          onChange={(v) => update({ drillsSchedule: v })}
          hueIndex={hueIndex}
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
function IconTabGeneral() {
  // Two stacked switches — the domain on/off toggles this tab actually holds.
  // Each pill + its knob is one evenodd path: the knob is a real cut-out,
  // not a second painted colour (see the fix note above this section).
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M3,3 H9 A2,2 0 0 1 9,7 H3 A2,2 0 0 1 3,3 Z M9.15,5 A1.15,1.15 0 1 0 6.85,5 A1.15,1.15 0 1 0 9.15,5 Z M7,9 H13 A2,2 0 0 1 13,13 H7 A2,2 0 0 1 7,9 Z M9.15,11 A1.15,1.15 0 1 0 6.85,11 A1.15,1.15 0 1 0 9.15,11 Z"
      />
    </svg>
  );
}

function IconTabStorage() {
  // A drive/disk stack — backup storage paths. REDESIGNED (jdp, live review,
  // same "unrecognizable when selected" report — this one was the odd one
  // out: unlike the other four, it never used a second colour at all, and
  // was still illegible in EVERY theme/state, selected or not). The a8eaa40
  // redraw flipped the old stroke's ellipse-top + two-line-body directly to
  // a single filled silhouette (a top semi-ellipse arced straight into a
  // bottom semi-ellipse, no seam) — as a flat, single-colour fill that
  // reads as a plain rounded blob with no internal structure at all, the
  // one visual cue that actually says "disk/cylinder" (a visible rim
  // separating the cap from the body) was lost entirely once the stroke's
  // own line-work disappeared. Rebuilt as two shapes: a plain filled body
  // (straight sides, curved visible-front bottom) UNDER a full top ellipse
  // that carries its own thin evenodd cut across its middle — a real seam,
  // not a second colour — so the cap reads as a distinct disk lid instead
  // of merging into one shapeless silhouette, in every theme/state/hue.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M2,4 V11 A6,2.2 0 0 0 14,11 V4 Z" />
      <path
        fillRule="evenodd"
        d="M8,1.8 A6,2.2 0 0 1 14,4 A6,2.2 0 0 1 8,6.2 A6,2.2 0 0 1 2,4 A6,2.2 0 0 1 8,1.8 Z M2.6,3.5 H13.4 V3.9 H2.6 Z"
      />
    </svg>
  );
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
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M14.2,8 A6.2,6.2 0 1 0 1.8,8 A6.2,6.2 0 1 0 14.2,8 Z M7.35,4.5 H8.65 V8.1 H7.35 Z M8.163,7.339 L11.506,9.347 L10.837,10.462 L7.494,8.453 Z"
      />
    </svg>
  );
}

function IconTabOffsite() {
  // A cloud — the remote/off-site replica target. Already a closed
  // silhouette under its old stroke (rule 218) — flips directly to a solid
  // fill with the identical path data, no redraw needed.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M4.5 12.5A3 3 0 0 1 4 6.53 3.5 3.5 0 0 1 10.9 5.1 2.75 2.75 0 0 1 12.5 10.4v.1a2.25 2.25 0 0 1-2 2h-6Z" />
    </svg>
  );
}

function IconTabNotifications() {
  // A bell — alerts. The bell body was already a closed silhouette (rule
  // 218 — direct flip). The clapper "ring" beneath it was a short open
  // stroke — redrawn as a small solid filled tab rather than a line.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M4 6.5a4 4 0 0 1 8 0c0 3 1 3.8 1 3.8H3s1-.8 1-3.8Z" />
      <path d="M6.5 12.1h3a1.5 1.5 0 0 1-3 0Z" />
    </svg>
  );
}

function IconTabIntegrity() {
  // A checked shield — repo/backup integrity checks. Shield + checkmark as
  // one evenodd path — the checkmark is a real cut-out, not a second
  // painted colour (see the fix note above this section); same check
  // polygon coordinates as before, now subtracted instead of painted over.
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M8 2 3 3.8v3.9c0 3.4 2.3 5.6 5 6.5 2.7-.9 5-3.1 5-6.5V3.8L8 2Z M5.2 7.85 6.9 9.55 10.55 5.6 11.65 6.6 7.15 11.5 4.15 8.5Z"
      />
    </svg>
  );
}

function IconTabSystem() {
  // Sliders — system/advanced/SSH knobs. Distinct from IconTabGeneral's
  // rounded toggle switches (a discrete on/off pair) — these are inline
  // continuous sliders instead. (Sidebar.tsx's own nav-rail IconConfig used
  // to be this same sliders motif too — GlimStone follow-up round replaced
  // THAT one with a padlock, jdp's live review: "sieht nicht gut aus" — see
  // that file's own IconConfig comment; this tab-strip glyph is a separate,
  // smaller-scale icon with its own real evenodd cut-out knobs, untouched by
  // that round, so the "distinct from a toggle switch" reasoning below still
  // holds on its own merits without needing that now-gone precedent.) Each
  // track + its knob is one evenodd path — the knob is a real cut-out, not
  // a second painted colour (see the fix note above this section).
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M2.65,4.35 H13.35 A0.65,0.65 0 0 1 13.35,5.65 H2.65 A0.65,0.65 0 0 1 2.65,4.35 Z M9.9,5 A1.4,1.4 0 1 0 7.1,5 A1.4,1.4 0 1 0 9.9,5 Z M2.65,10.35 H13.35 A0.65,0.65 0 0 1 13.35,11.65 H2.65 A0.65,0.65 0 0 1 2.65,10.35 Z M6.4,11 A1.4,1.4 0 1 0 3.6,11 A1.4,1.4 0 1 0 6.4,11 Z"
      />
    </svg>
  );
}

// "Reset to default" arrow — the Accent Card's preset-row reset button and
// the Off-site tab's rainbow-palette reset button (two separate call sites,
// same icon-only square/circle badge). FILLED (design-language.md "Icon
// glyphs"): a filled ring segment + a solid triangular arrowhead, not a
// `stroke`-width line — same construction family as Sidebar.tsx's own
// IconRecovery/IconRestore.
//
// REDRAWN, BOLDER (GlimStone follow-up round, jdp re-reporting after a prior
// round's fix didn't hold up: "Der Reset-Badge soll einen besseren Glyph
// bekommen" — the previous thinner ring, at this file's 16×16 icon-only-
// badge scale sitting directly inside a busy row of 8 bright colour
// swatches, read as an ambiguous "C" rather than an unambiguous reset arrow).
// Thickened the ring from a 1.8px band (outer r=6 / inner r=4.2) to a 3.0px
// band (outer r=6 / inner r=3.3) and enlarged the arrowhead proportionally —
// verified live at actual 28×28 badge size, on both a dark and a light
// neutral chip background (this glyph's own two real call sites are now
// both `tone="neutral"`, never a bright fill), reading unambiguously as a
// counter-clockwise "undo/reset" arrow at that size where the old thinner
// version did not.
//
// DELIBERATELY DIVERGES from IconRecovery/IconRestore now (previously
// identical path data to both — see each of THEIR OWN header comments,
// which this round leaves untouched about EACH OTHER but corrects about
// this icon): those two live in contexts with no adjacent competing colour
// (a full-page nav-tab icon; a plain single badge in a list row) and keep
// the original, slightly finer proportions that already read clearly there.
// This icon's one specific job — reading as "reset," at 16px, positioned
// directly beside 8 saturated colour swatches actively competing for the
// eye — is a strictly harder legibility case that earns its own bolder
// tuning rather than inheriting a ratio measured for an easier one. Renamed
// from IconResetSwirl to IconResetArrow to make that split explicit: this is
// no longer "the same glyph, reused," it is a purpose-built sibling.
function IconResetArrow() {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
      <path d="M14 8A6 6 0 1 1 8 2L8 4.7A3.3 3.3 0 1 0 11.3 8Z" />
      <path d="M8 1 3.5 3 8 5Z" />
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
  type ScheduleBoolKey =
    | "perItemSchedules"
    | "catchUpMissed"
    | "drillsEnabled"
    | "offsiteDrillsEnabled"
    | "restartHealthWait";
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
          // VMs == Flash == Folders, and not off), so the Schedules tab's sync
          // toggle reflects it on load. Reproduced from the retired Plans page;
          // filesSchedule added to the comparison alongside Task 2's own
          // extension of the toggle's live effect to cover Folders too — without
          // it, a server state where Containers/VMs/Flash already matched but
          // Folders didn't would show the toggle ON while Folders still quietly
          // held its own independent value until the next edit.
          const s = res.settings;
          if (
            s.vmsSchedule === s.containersSchedule &&
            s.flashSchedule === s.containersSchedule &&
            s.filesSchedule === s.containersSchedule &&
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
  // OFF writes the literal "off" cadence string; ON restores "daily 02:00"
  // (the same daily-at-02:00 baseline this grammar already uses elsewhere,
  // e.g. ContainersSchedule's own portable-settings test fixture) — this
  // does NOT introduce a new configScheduleEnabled field: parseCadenceString/
  // buildCadenceString (CadenceBuilder.tsx) already round-trip "off"/""
  // through CadenceMode "off" cleanly, so a second boolean would just be a
  // second source of truth for the exact same fact. (BombVault's separate
  // `configEnabled` field, toggled in the Domains card above, is a different
  // concept — whether the self-backup domain exists at all — left untouched.)
  async function toggleConfigSchedule(next: boolean) {
    const prev = settings?.configSchedule ?? "off";
    const value = next ? "daily 02:00" : "off";
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
        setPwSaveShake((n) => n + 1);
      }
    } catch {
      setPwSaveState("idle");
      push(t("auth.saveError"), "fail");
      setPwSaveShake((n) => n + 1);
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
    // `main` → its `bv-page-enter` child, both given a matching `flex-1 flex
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
    <div className="flex flex-col gap-10 flex-1">
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
      {/* any of the 42 locales, not just the one word measured live today.    */}
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
          `.bv-tab-slide`'s own entrance animation (index.css) replay every
          time instead of only once at Settings' own first mount (a
          persistent class on a node that never gets recreated never
          replays its animation, the same reasoning bv-stagger-row's own
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
        className="flex flex-col gap-10 bv-tab-slide flex-1"
        style={{ maxWidth: tabStripWidth ?? undefined, "--tab-dir": tabDir } as CSSProperties}
      >

      {/* ------------------------------------------------------------------ */}
      {/* SCHEDULES — the single owner of every cadence (migrated from Plans).  */}
      {/* Backup schedules reuse the proven per-domain sections + sync toggle;  */}
      {/* off-site / self-backup / restore-check cadences are edited here too.   */}
      {/* Task 5 (live-review — "Speichern-Buttons können weg, es soll immer    */}
      {/* alles live gespeichert werden"): every field on this tab auto-saves   */}
      {/* itself now (scheduleField/autoSaveScheduleField/handleSyncSchedules-  */}
      {/* Toggle above) — there is no tab-wide SaveBar left to persist them.    */}
      {/* ------------------------------------------------------------------ */}
      {tab === "schedules" && (
        <>
          {/* Schedule options (jdp, live-review — "Die beiden Toggle sollen in
              eine eigene Card"): perItemSchedules (#121) and the Containers-
              sync toggle used to be two raw <input type="checkbox"> rows
              sitting directly in this tab, outside any Card. Both are now the
              shared ToggleRow component, grouped in their own Card, first on
              this tab, directly above ContainersSection — perItem first, sync
              directly below it, per jdp's own ordering. (The group-level
              "Backup-Zeitpläne" Badge heading that used to sit above this
              Card was removed on jdp's live-review ask — the four domain
              schedule Cards below already carry their own clear headings, so
              the group label was redundant; nextHue()'s sequence starts
              directly with this Card now, one call short of before.) A
              genuine two-member list, so each ToggleRow gets its own LOCAL
              hueIndex (0/1, independent of this Card's own nextHue() notch),
              the same "own local 0-based index per group" rule the Domains
              card's seven rows and the merged Colors Card's three rainbow
              toggles already follow. */}
          <Card title={t("settings.schedulesOptions")} hueIndex={nextHue()}>
            <ToggleRow
              label={t("settings.perItemSchedules")}
              hint={t("settings.perItemSchedulesHint")}
              checked={settings.perItemSchedules}
              onChange={(v) => void autoSaveScheduleField("perItemSchedules", v)}
              disabled={schedFieldBusy.perItemSchedules}
              shakeNonce={schedFieldShake.perItemSchedules}
              pulseNonce={fieldPulse.perItemSchedules}
              hueIndex={0}
            />
            {/* Sync toggle — applies the Containers cadence to VMs, Flash AND
                Folders (Task 2 extended this from "VMs + Flash" to also cover
                Folders — see FilesSection's own new syncSchedules prop). */}
            <ToggleRow
              label={t("jobs.syncSchedules")}
              hint={t("jobs.syncSchedulesHint")}
              checked={syncSchedules}
              onChange={(v) => void handleSyncSchedulesToggle(v)}
              disabled={syncToggleBusy}
              shakeNonce={syncToggleShake || undefined}
              // handleSyncSchedulesToggle's own save() patch touches vms/
              // flash/filesSchedule together (see that function) — any one
              // of the three is bumped by save()'s success branch, so
              // vmsSchedule works as well as either of the others here.
              pulseNonce={fieldPulse.vmsSchedule}
              hueIndex={1}
            />
          </Card>
          <ContainersSection
            settings={settings}
            containers={containers}
            onChange={(v) => scheduleField("containersSchedule", v)}
            perItem={settings.perItemSchedules}
            t={t}
            hueIndex={nextHue()}
          />
          <VMsSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) => scheduleField("vmsSchedule", v)}
            vms={vms}
            perItem={settings.perItemSchedules}
            t={t}
            hueIndex={nextHue()}
          />
          <FlashSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) => scheduleField("flashSchedule", v)}
            t={t}
            hueIndex={nextHue()}
          />
          <FilesSection
            settings={settings}
            syncSchedules={syncSchedules}
            fileSets={fileSets}
            onChange={(v) => scheduleField("filesSchedule", v)}
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
                  onChange={(e) => scheduleField(key, e.target.value)}
                  placeholder={t("offsite.schedulePlaceholder")}
                  dir="ltr"
                  className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus text-start"
                />
              </div>
            ))}
          </Card>

          {/* Self-backup schedule (schedulesSelfBackup): BombVault's own config.
              Live-review (jdp: "Selbst-Backup-Zeitplan bitte mit Toggle für
              an/aus, und Zeitplancard wie bei Container, VMs usw."): this was
              the one schedule editor left in this tab as a bare hand-typed
              cadence <input> (had to type e.g. "daily 02:00" yourself, and
              "off" was only reachable by typing the word), no on/off control
              of its own. Rebuilt to match ContainersSection/VMsSection/
              FlashSection/FilesSection's own shape: the same status row +
              CadenceBuilder-in-a-well, one IIFE-captured `hueIdx` feeding
              both this Card's heading notch and the CadenceBuilder's
              TimePicker inside it — identical to the schedulesChecks Card
              just below (see that IIFE's own comment for why a bare inline
              `hueIndex={nextHue()}` can't feed two hue-aware children from
              one call).
                The toggle's label is NOT hidden behind this Card's own title
              — RestoreChecksSection's `verify.auto` ToggleRow right below
              this one used to hide its own caption the same way (reasoning:
              "the Card's title already says the same thing"), and jdp
              explicitly reversed that exact pattern there ("Bei erstem
              Toggle bitte 'Automatische Restore-Prüfungen' hinschreiben") —
              so this toggle reuses that Card's own corrected shape instead:
              the SAME string as both the Card's `title` and the ToggleRow's
              visible `label`, no `hideLabel`. No `hueIndex` on the toggle
              itself either, matching that same corrected ToggleRow (and
              FlashZipExportCard's lone ToggleRow) — a single stand-alone
              switch with no sibling toggles of its own kind in this Card is
              the one case ToggleRow's own hueIndex doc carves out as having
              no list to walk. See toggleConfigSchedule's own comment above
              for why this reuses the cadence string's existing "off" mode
              instead of a new configScheduleEnabled field. */}
          {(() => {
            const hueIdx = nextHue();
            const schedule = settings.configSchedule;
            const status = scheduleStatus(schedule);
            return (
              <Card title={t("settings.schedulesSelfBackup")} hint={t("config.scheduleHint")} hueIndex={hueIdx}>
                <ToggleRow
                  label={t("settings.schedulesSelfBackup")}
                  checked={status !== "off"}
                  onChange={(v) => void toggleConfigSchedule(v)}
                  disabled={configScheduleToggleBusy}
                  shakeNonce={configScheduleToggleShake}
                  pulseNonce={fieldPulse.configSchedule}
                />
                <ScheduleRow schedule={schedule} />
                <div className="rounded-card bg-carbon-surface2 p-4">
                  <CadenceBuilder
                    label={t("settings.schedulesSelfBackup")}
                    value={schedule}
                    onChange={(v) => scheduleField("configSchedule", v)}
                    hueIndex={hueIdx}
                  />
                </div>
              </Card>
            );
          })()}

          {/* Restore-check drills (RestoreChecksSection) moved to the Integrity
              tab (jdp, live-review: "Gehört die 'Automatische Restore-
              Prüfungen' Card nicht in den Integritäts-Tab?") — it configures
              WHAT gets verified and how often, which fits that tab's existing
              verify/unlock/prune/drill actions better than this tab's own
              "when do backup jobs run" focus. See the `tab === "integrity"`
              block below for its new call site; removing it here also frees
              up one `nextHue()` notch, automatically renumbering every Card
              still below on this tab (see that counter's own doc comment for
              why no manual re-numbering is needed). */}

          {/* Missed schedules: anacron-style catch-up after start. Backend runs
              the missed domain job ~2 minutes after boot (see internal/schedule
              CatchUpMissed). */}
          <Card title={t("settings.missedSchedulesTitle")} hueIndex={nextHue()}>
            <ToggleRow
              label={t("settings.catchUpMissed")}
              hint={t("settings.catchUpMissedHint")}
              checked={settings.catchUpMissed}
              onChange={(v) => void autoSaveScheduleField("catchUpMissed", v)}
              disabled={schedFieldBusy.catchUpMissed}
              shakeNonce={schedFieldShake.catchUpMissed}
              pulseNonce={fieldPulse.catchUpMissed}
            />
          </Card>

          {/* Health-gated ordered restart (#119): after a backup that stopped
              other containers ("Stop other containers during backup"), they
              restart in compose depends_on order and each must report
              healthy/running before its dependents start. The wait also holds
              through the post-backup update recreate (see internal/backup
              orchestrator WhileDependentsStopped). */}
          <Card title={t("settings.restartHealthTitle")} hueIndex={nextHue()}>
            {/* Full-page Speichern-Button sweep (jdp, live review, emphatic:
                "Die Speicher-Buttons sollen in allen Tabs weg. Überall soll
                es automatisch speichern."): this was the one Card left on
                the Schedules tab still batched into its own manual SaveBar
                after Task 5 converted every other field here — that task's
                own comment named it as a deliberate exception at the time;
                this pass closes it out with the exact same shapes Task 5
                already established one Card up (autoSaveScheduleField for
                the discrete toggle, scheduleField's debounce for the
                continuously-typed number). */}
            <ToggleRow
              label={t("settings.restartHealthWait")}
              hint={t("settings.restartHealthWaitHint")}
              checked={settings.restartHealthWait}
              onChange={(v) => void autoSaveScheduleField("restartHealthWait", v)}
              disabled={schedFieldBusy.restartHealthWait}
              shakeNonce={schedFieldShake.restartHealthWait}
              pulseNonce={fieldPulse.restartHealthWait}
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
                    scheduleField("restartHealthTimeoutSec", n);
                  }}
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
                />
              </label>
            )}
          </Card>

          {/* Restore-check schedule (schedulesChecks) moved to the Integrity
              tab alongside RestoreChecksSection above (jdp, live-review —
              same "belongs with WHAT/how-often gets verified, not WHEN
              backup jobs run" reasoning). See the `tab === "integrity"`
              block below for its new call site. */}
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
          pulseNonce={fieldPulse.containersEnabled}
          hueIndex={0}
        />
        <ToggleRow
          label={t("settings.vmsEnabled")}
          hint={t("settings.vmsEnabledHint")}
          checked={settings.vmsEnabled}
          onChange={(v) => void toggleDomainEnabled("vmsEnabled", v)}
          disabled={domainToggleBusy.vmsEnabled}
          shakeNonce={domainToggleShake.vmsEnabled}
          pulseNonce={fieldPulse.vmsEnabled}
          hueIndex={1}
        />
        <ToggleRow
          label={t("settings.flashEnabled")}
          hint={t("settings.flashEnabledHint")}
          checked={settings.flashEnabled}
          onChange={(v) => void toggleDomainEnabled("flashEnabled", v)}
          disabled={domainToggleBusy.flashEnabled}
          shakeNonce={domainToggleShake.flashEnabled}
          pulseNonce={fieldPulse.flashEnabled}
          hueIndex={2}
        />
        <ToggleRow
          label={t("settings.filesEnabled")}
          hint={t("settings.filesEnabledHint")}
          checked={settings.filesEnabled}
          onChange={(v) => void toggleDomainEnabled("filesEnabled", v)}
          disabled={domainToggleBusy.filesEnabled}
          shakeNonce={domainToggleShake.filesEnabled}
          pulseNonce={fieldPulse.filesEnabled}
          hueIndex={3}
        />
        <ToggleRow
          label={t("settings.configEnabled")}
          hint={t("settings.configEnabledHint")}
          checked={settings.configEnabled}
          onChange={(v) => void toggleDomainEnabled("configEnabled", v)}
          disabled={domainToggleBusy.configEnabled}
          shakeNonce={domainToggleShake.configEnabled}
          pulseNonce={fieldPulse.configEnabled}
          hueIndex={4}
        />
        <ToggleRow
          label={t("settings.receiverEnabled")}
          hint={t("settings.receiverEnabledHint")}
          checked={settings.receiverEnabled}
          onChange={(v) => void toggleDomainEnabled("receiverEnabled", v)}
          disabled={domainToggleBusy.receiverEnabled}
          shakeNonce={domainToggleShake.receiverEnabled}
          pulseNonce={fieldPulse.receiverEnabled}
          hueIndex={5}
        />
        <ToggleRow
          label={t("settings.fleetEnabled")}
          hint={t("settings.fleetEnabledHint")}
          checked={settings.fleetEnabled}
          onChange={(v) => void toggleDomainEnabled("fleetEnabled", v)}
          disabled={domainToggleBusy.fleetEnabled}
          shakeNonce={domainToggleShake.fleetEnabled}
          pulseNonce={fieldPulse.fleetEnabled}
          hueIndex={6}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Backup paths                                             */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.paths")} hint={t("settings.pathsHint").replace("{root}", hostMountRoot)} hueIndex={nextHue()}>
        {/* Full-page Speichern-Button sweep (jdp, live review, emphatic:
            "Die Speicher-Buttons sollen in allen Tabs weg. Überall soll es
            automatisch speichern."): all six fields below used to batch into
            one bottom SaveBar. Each now debounce-auto-saves itself instead —
            the exact same `debouncedSave`-keyed-by-field-name shape the
            Schedules tab's own `scheduleField` already established for
            continuously-typed values (a path is typed/browsed the same way a
            cron string is), just called directly here since these six PATCH
            single independent fields rather than a whole cadence group.
              `hueIndex={0..4}` below (GlimStone standing colour-engine rule,
            closing the gap OffsiteWizard's own hueIndex doc comment already
            named): these five PathModeSwitch rows are one related GROUP (own
            local 0-based index per group, same rule as the Domains Card's
            seven ToggleRows), separate from this Card's own heading
            `nextHue()` call above. */}
        <PathModeSwitch
          label={t("settings.containersPath")}
          domain="containers"
          value={settings.containersPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, containersPath: v } : prev);
            debouncedSave("containersPath", () =>
              void save({ containersPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={0}
        />
        <PathModeSwitch
          label={t("settings.vmsPath")}
          domain="vms"
          value={settings.vmsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, vmsPath: v } : prev);
            debouncedSave("vmsPath", () =>
              void save({ vmsPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={1}
        />
        <PathModeSwitch
          label={t("settings.flashPath")}
          domain="flash"
          value={settings.flashPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, flashPath: v } : prev);
            debouncedSave("flashPath", () =>
              void save({ flashPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={2}
        />
        <PathModeSwitch
          label={t("settings.configPath")}
          domain="config"
          value={settings.configPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, configPath: v } : prev);
            debouncedSave("configPath", () =>
              void save({ configPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={3}
        />
        <PathModeSwitch
          label={t("settings.filesPath")}
          domain="files"
          value={settings.filesPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, filesPath: v } : prev);
            debouncedSave("filesPath", () =>
              void save({ filesPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={4}
        />
        <FolderBrowser
          label={t("settings.restoreFolder")}
          value={settings.restoreFolder}
          hostMountRoot={hostMountRoot}
          hint={t("settings.restoreFolderHint")}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, restoreFolder: v } : prev);
            debouncedSave("restoreFolder", () =>
              void save({ restoreFolder: v }, setPathSaveState, setPathSaveError)
            );
          }}
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
                  // Full-page Speichern-Button sweep: this whole grid used to
                  // batch into one bottom SaveBar — each cell now debounce-
                  // auto-saves itself, keyed by its own field name so typing
                  // in one cell never resets another cell's pending timer.
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setRetSaveState, setRetSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ))}
        </div>
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
          pulseNonce={fieldPulse.pruneImageAfterUpdate}
        />
        <ToggleRow
          label={t("settings.reconcileUnraidStatus")}
          hint={t("settings.reconcileUnraidStatusHint")}
          checked={settings.reconcileUnraidUpdateStatus}
          onChange={(v) => void autoSaveField("reconcileUnraidUpdateStatus", v, setReconcileSaveState, setReconcileSaveError)}
          disabled={mergedFieldBusy.reconcileUnraidUpdateStatus}
          shakeNonce={mergedFieldShake.reconcileUnraidUpdateStatus}
          pulseNonce={fieldPulse.reconcileUnraidUpdateStatus}
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
                  Registry-add button below already made.
                    COLOUR-ENGINE ROUND (jdp's standing rule, five escalations
                  deep): this badge and the Registry-add one below were still
                  flat `bg-carbon-surface3` grey with no tie to this Card's own
                  hue at all, the same gap that got the delete badge's grey
                  special-casing removed a round earlier. Both are real
                  `Badge`s now (`as="button" tone="active" shape="square"
                  size="icon"`), which for an icon-only badge resolves to the
                  full solid `bg-accent`/`text-accentContrast` fill, NOT the
                  pale wash jdp rejected as "halb abgedunkelt" (see Badge.tsx's
                  own `toneClasses` ROUND 2 comment).
                    No `hueIndex` prop, and none needed: this Card is
                  `<Card ... hueIndex={nextHue()}>` with no local variable to
                  hand down, but Card's own wrapper carries `.glim-hue`, and
                  `[data-rainbow] .glim-hue` (index.css) redefines
                  `--color-accent` for its whole subtree, so `bg-accent` here
                  already computes to THIS Card's rainbow position by ordinary
                  custom-property inheritance. Same mechanism Containers.tsx's
                  own folder-add badge documents; wrapping this Card in an IIFE
                  purely to capture `nextHue()` would add a second source of
                  truth for a colour that already resolves correctly. Verified
                  live with getComputedStyle against the Card's own
                  `--item-hue`.
                    `size="icon"` is the app's ONE square-icon-badge size and
                  is the same 32px this call site already had, so the footprint
                  is unchanged; `shrink-0` survives as `className` because it
                  is layout, not appearance. Not a fresh guess either: this
                  row's own three text fields
                  are `text-sm px-3 py-1.5` — the SAME classes already
                  measured live to render at 32px for those other controls
                  (see Selector.tsx's own `iconOnly` doc for that
                  measurement's full writeup) — so 32px is this row's real
                  control height too, confirmed, not assumed from a token
                  used elsewhere. IconTrash (components/Sidebar.tsx) drawn
                  fresh for this — no trash glyph existed in this codebase
                  yet — filled/`currentColor`-only, no `stroke`, matching
                  every other icon in that file's icon-only-badge set. */}
              <Badge
                as="button"
                tone="active"
                shape="square"
                size="icon"
                className="shrink-0"
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
              >
                <IconTrash />
              </Badge>
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
              what it does. Same 32px square-icon-badge footprint as
              FolderBrowser's own "Durchsuchen" badge (the one real field/
              control height already established on this page), expressed as
              Badge's `size="icon"` stage now rather than a hand-written
              `h-8 w-8`. Converted from flat `bg-carbon-surface3` grey to a
              hue-carrying Badge in the same colour-engine round as the
              Registry-remove badge above: see that call site's own comment
              for the full reasoning, including why neither needs an explicit
              `hueIndex`. */}
          <div className="flex justify-end">
            <Badge
              as="button"
              tone="active"
              shape="square"
              size="icon"
              className="shrink-0"
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
            >
              <IconAdd />
            </Badge>
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
              // Full-page Speichern-Button sweep: was its own bottom SaveBar.
              debouncedSave("resticCacheMaxMB", () =>
                void save({ resticCacheMaxMB: n }, setCacheSaveState, setCacheSaveError)
              );
            }}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
          />
        </label>
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
            pulseNonce={fieldPulse.exportEncryptEnabled}
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
            pulseNonce={fieldPulse.encryptionEnabled}
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
              {/* No more heading-styled <h4> here either (jdp, live-review:
                  "Wiederherstellungs-Kit bitte auch normal formatieren") — same
                  fix, same reasoning, as the Plain-export/Encryption blocks
                  above: this sub-section is just a caption plus a single
                  icon-only download button beneath it, not a heading
                  introducing its own block of content, so it shouldn't LOOK
                  like a section heading either. Swapped the semantic `<h4>`
                  for a plain `<span>` carrying ToggleRow's own exact label
                  classes (`flex items-center gap-1.5 text-sm text-carbon-text`,
                  see ToggleRow's own label span above) instead of the retired
                  bold/uppercase/tracking-widest heading treatment — the ONE
                  normal-caption style this page already uses everywhere else,
                  reused verbatim rather than inventing a second one for this
                  call site. The InfoBubble stays put unchanged; there's no
                  ToggleRow here to fold it onto (this row is a caption + a
                  bare download button, not a toggle). */}
              <span className="flex items-center gap-1.5 text-sm text-carbon-text">
                {t("recovery.title")}
                <InfoBubble tip={t("recovery.why")} />
              </span>
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
                  start/end pairing on this page. `size="icon"` is the app's
                  ONE square-icon-badge size (32px), the same footprint
                  FolderBrowser's own Browse badge and the Registry-add badge
                  above use, expressed as Badge's own size stage rather than a
                  hand-written `h-8 w-8`. Converted from flat
                  `bg-carbon-surface3` grey to a hue-carrying Badge in the same
                  colour-engine round as those two: see the Registry-remove
                  badge's own comment for the full reasoning, including why the
                  enclosing Card's `.glim-hue` makes an explicit `hueIndex`
                  unnecessary here. */}
              <Badge
                as="button"
                tone="active"
                shape="square"
                size="icon"
                className="self-end shrink-0"
                tip={t("recovery.download")}
                onClick={() => {
                  setKitError(null);
                  void downloadRecoveryKit().then(setKitError);
                }}
              >
                <IconDownload />
              </Badge>
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
      <div id="offsite" className="flex flex-col gap-6">
      {/* Live-review round ("Bei der ersten Card überlappen sich zwei
          Cardtitelbadges. Können wir für Container, VMs, Flash, Ordner
          jeweils eine eigene Card machen?"): this used to be a group-heading
          `<h2>` badge (offsite.sectionTitle) immediately followed by ONE
          shared Card whose body looped over all four domains — the group
          badge's own `-top-[11px]` notch and the Card's own `-top-[11px]`
          notch, only `gap-6` (24px) apart with the group heading rendering
          at ZERO height (its only child is `position: absolute`, so it
          contributes nothing to flow height — see Card's own comment on why
          the badge straddles the card's top edge this way), landed the two
          22px-tall badges overlapping by several px. Splitting into four
          per-domain Cards does NOT fix that geometry on its own — the FIRST
          new Card would sit exactly `gap-6` below the same zero-height group
          heading, reproducing the identical overlap (verified against the
          live math before shipping this, not just assumed). The actual fix
          is the one this file's own Schedules tab already took this same
          round for the identical shape (see that tab's own history: its
          "Backup-Zeitpläne" group heading was removed because "these Cards
          already carry their own clear headings, so the group label was
          redundant") — dropping the group heading entirely, now that each
          of the four Cards below carries an unambiguous
          "OFFSITE-KOPIE <DOMAIN>" title of its own. offsite.sectionTitle
          had no other call site, so it's gone from i18n.ts (en/de) and all
          24 locale files, the same mechanical removal as
          settings.schedulesBackup got.
          `gap-6` on this wrapper: Card's own outer `<div>` no longer needs
          `relative` here (there is no sibling group-heading badge left to
          coexist with), and the four per-domain Cards need the SAME
          vertical rhythm every other multi-Card tab in this file already
          gets from the shared `<div className="flex flex-col gap-6">`
          wrapping the whole tab body two levels up — this nested wrapper
          exists only because `id="offsite"` (the deep-link anchor,
          `/settings#offsite`) needs a real element to attach to, not a
          Fragment. */}
      {([
        ["containersOffsite", "nav.containers", "containers"],
        ["vmsOffsite", "nav.vms", "vms"],
        ["flashOffsite", "nav.flash", "flash"],
        ["filesOffsite", "nav.files", "files"],
      ] as const).map(([repoKey, label, domain]) => {
        const wizardOpen = offsiteWizard === domain;
        // This domain's OWN rainbow position — the SAME value fed to this
        // Card's own heading notch below AND to every clickable control
        // inside it (TestConnectionButton/ReplicateNowButton/the Einrichten
        // toggle/OffsiteTargetsSection's own "Ziel hinzufügen" button), per
        // jdp's explicit ask ("Die Buttons ... in die Farbengine
        // aufnehmen") — not four independent nextHue() calls, which would
        // desync a domain's own action buttons from its own Card's colour.
        const hueIdx = nextHue();
        return (
        <Card key={repoKey} title={t("offsite.copyDomainTitle").replace("{domain}", t(label))} hueIndex={hueIdx}>
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
              not a mechanical one — flagged rather than decided here.
              CARD-SPLIT FOLLOW-UP: this text applies identically to all four
              domains (it's about repo URL syntax, not domain-specific), so it
              stays a ONE-TIME read rather than repeating verbatim in every
              new Card — shown once, in the first (Containers) Card only. */}
          {domain === "containers" && (
            <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
          )}
          <div className="flex flex-col gap-1">
            <div className="flex items-center justify-between">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <span className="inline-flex items-center gap-2">
                {settings[repoKey] && !wizardOpen && (
                  <>
                    <TestConnectionButton domain={domain} t={t} hueIndex={hueIdx} />
                    <ReplicateNowButton domain={domain} t={t} hueIndex={hueIdx} />
                  </>
                )}
                {/* GlimStone follow-up round (jdp, live review: "Können wir die
                    Buttons in quadratische Badges mit Glyphen umwandeln?") — a
                    square icon-only badge, IconGear when the wizard is closed
                    (offering to open setup) swapping to IconClose when it's
                    open, the exact same open/closed condition that used to
                    swap the button's own visible text between
                    "Einrichten…"/"Schließen". Both strings survive unchanged
                    as the `tip` tooltip's content instead — see
                    ReplicateNowButton's own comment above for the full
                    "coloured text -> neutral glyph, wash -> solid fill"
                    writeup this shares. */}
                <Badge
                  as="button"
                  tone="active"
                  shape="square"
                  size="icon"
                  hueIndex={hueIdx}
                  onClick={() => setOffsiteWizard(wizardOpen ? null : domain)}
                  tip={wizardOpen ? t("offsite.wizard.close") : t("offsite.wizard.setup")}
                >
                  {wizardOpen ? <IconClose /> : <IconGear />}
                </Badge>
              </span>
            </div>
            {wizardOpen ? (
              <OffsiteWizard
                domain={domain}
                settings={settings}
                setSettings={setSettings}
                save={save}
                t={t}
                hueIndex={hueIdx}
              />
            ) : (
              <>
                <input
                  value={settings[repoKey]}
                  spellCheck={false}
                  onChange={(e) => {
                    const v = e.target.value;
                    setSettings((prev) => (prev ? { ...prev, [repoKey]: v } : prev));
                    // Full-page Speichern-Button sweep: this Card's own bottom
                    // SaveBar is gone — each repo URL debounce-auto-saves
                    // itself, keyed by its own field name (the off-site
                    // *cadences* stay owned by the Schedules tab, unaffected).
                    debouncedSave(repoKey, () =>
                      void save({ [repoKey]: v } as Partial<Settings>, setOffsiteSaveState, setOffsiteSaveError)
                    );
                  }}
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
                domain beyond the primary editor above, managed via the CRUD API.
                hueIndex threaded through for the same "Ziel hinzufügen" button —
                see that component's own comment. */}
            <OffsiteTargetsSection domain={domain} t={t} hueIndex={hueIdx} />
          </div>
        </Card>
        );
      })}
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
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setOffRetSaveState, setOffRetSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ))}
        </div>
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
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setLimSaveState, setLimSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ))}
        </div>
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Monitoring (Prometheus)                                   */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper — same reason as
          the Storage tab's cacheTitle Card above. */}
      {tab === "system" && advanced && (
      <Card title={t("settings.metrics")} hueIndex={nextHue()}>
        {/* GlimStone follow-up round (jdp, live review: "Prometheus-Metriken
            unter /metrics ... in eine InfoBubble" — design-language.md rule 8,
            "explanations live in a bubble, not on the page"): this used to be
            a permanent `<p>` under the Card title, reasoned at the time as an
            "exact syntax to copy correctly" carve-out (the same one RcloneCard's/
            CloudCard's own hints still use). jdp's live review overruled that
            specifically for this text — unlike rclone.pathHint's own
            "rclone:<remote>:<bucket>/path" syntax (which someone fills into a
            DIFFERENT tab's Backup Path field from memory, so it needs to stay
            findable without already hovering an icon here), this hint is
            self-contained: /metrics and the Bearer-token syntax are both used
            right here, on the same toggle, so a hover bubble is not hiding
            anything a reader would need on a different screen. Moved onto the
            ToggleRow's own `hint` prop below (the same "(i) beside the label"
            mechanism as every other bubbled explanation in this file) — no
            `description` here for the same "the Card's own hint already
            covers it" reasoning this row's OLD comment gave, just now living
            on the toggle's `hint` instead of a Card-level paragraph. */}
        <ToggleRow
          label={t("settings.metricsEnable")}
          hint={t("settings.metricsHint")}
          checked={settings.metricsEnabled}
          onChange={(v) => void autoSaveToggle("metricsEnabled", v, setMetricsSaveState, setMetricsSaveError)}
          disabled={fieldBusy.metricsEnabled}
          shakeNonce={fieldShake.metricsEnabled}
          pulseNonce={fieldPulse.metricsEnabled}
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
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => prev ? { ...prev, metricsToken: v } : prev);
              // Full-page Speichern-Button sweep: was this Card's own bottom
              // SaveBar. Keeps the SAME "is-set flag honest locally" patch
              // shape the old onSave sent — a non-blank token being saved
              // marks itself set; a blank save keeps whatever was stored.
              debouncedSave("metricsToken", () =>
                void save(
                  { metricsToken: v, metricsTokenSet: v.trim() !== "" || settings.metricsTokenSet },
                  setMetricsSaveState,
                  setMetricsSaveError
                )
              );
            }}
            placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
          />
        </label>
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
      {/* NOTIFICATIONS — NotifyCard now renders THREE Cards internally: its  */}
      {/* settings Card (always), its channels Card (advanced only), and its  */}
      {/* Healthchecks Card (advanced only, card-split follow-up) — see       */}
      {/* NotifyCard's own header comment. `channelsHueIndex`/                */}
      {/* `healthchecksHueIndex` MUST each be their own `nextHue()` call made  */}
      {/* INSIDE `tab === "notifications" && advanced &&`, not an eager call   */}
      {/* at the unconditional site above: both Cards only paint while         */}
      {/* Advanced is on, and this file already has one documented            */}
      {/* live-Playwright-caught bug from doing it the eager way — see the     */}
      {/* SYSTEM tab's Spike Card comment ("fires every render regardless")    */}
      {/* for the exact silent-hue-shift failure mode this avoids: a slot      */}
      {/* burned on a Card that never painted, shifting every later heading    */}
      {/* on this tab by one position while Advanced was off. Plain `&&`       */}
      {/* short-circuits correctly, so with Advanced off both values are       */}
      {/* simply never computed and the props below evaluate to `undefined` —  */}
      {/* NotifyCard never renders either Card in that case anyway, so the     */}
      {/* unused values never matter, and no hue slot is spent.                */}
      {/* ------------------------------------------------------------------ */}
      {tab === "notifications" && (() => {
        const settingsHue = nextHue();
        const channelsHue = advanced ? nextHue() : undefined;
        const healthchecksHue = advanced ? nextHue() : undefined;
        return (
          <NotifyCard
            t={t}
            platformKind={platformKind}
            hueIndex={settingsHue}
            channelsHueIndex={channelsHue}
            healthchecksHueIndex={healthchecksHue}
          />
        );
      })()}

      {/* NOTIFICATIONS — Weekly digest: one summary message per week through
          the channels configured above. Schedule input mirrors the drills/
          tamper cadence editors (CadenceBuilder's own <fieldset disabled>
          handles the dimming — no opacity gate on the wrapping container).
            IIFE for the same reason as the tamper-test schedule Card above
          (Task 3): `hueIdx` is captured once and handed to BOTH this Card's
          own heading notch and the CadenceBuilder's TimePicker inside it,
          instead of two independent `nextHue()` calls landing on different
          colours for one visually-grouped Card. */}
      {tab === "notifications" && (() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.digestTitle")} hint={t("settings.digestHint")} hueIndex={hueIdx}>
            {/* Full-page Speichern-Button sweep: this Card's own bottom
                SaveBar is gone — the toggle auto-saves immediately (revert +
                shake on failure, via the page-wide autoSaveToggle), the
                cadence debounces (via debouncedSave), same split every other
                toggle+cadence pairing on this page already uses.
                  No-empty-toggles audit (jdp: "Wochenbericht-Toggle mit Text
                'Wochenbericht' hinschreiben. Es soll nie 'leere' Toggles
                geben."): this row used to `hideLabel` on the Card-title-
                already-says-it reasoning, the third time that exact pattern
                got built in this file after jdp reversed it twice before
                (Rainbow master toggle, Restore-Prüfungen toggle). `hideLabel`
                is gone from ToggleRow entirely now (see its own header
                comment) — the row's own label is always visible. */}
            <ToggleRow
              label={t("settings.digestToggle")}
              checked={settings.digestEnabled}
              onChange={(v) => void autoSaveToggle("digestEnabled", v, setDigestSaveState, setDigestSaveError)}
              disabled={fieldBusy.digestEnabled}
              shakeNonce={fieldShake.digestEnabled}
              pulseNonce={fieldPulse.digestEnabled}
            />
            {/* Resolved-schedule badge — NEW this round, same reason as
                RestoreChecksSection's (see that call site's own comment):
                this was the second of the three cadence editors that had no
                badge above them and relied on CadenceBuilder's own inline
                preview, now removed. `enabled` wired to `digestEnabled` for
                the same reason — the on/off is a separate toggle here, not
                the cadence string's own "off" mode. */}
            <ScheduleRow schedule={settings.digestSchedule} enabled={settings.digestEnabled} />
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.schedule")}
                value={settings.digestSchedule}
                disabled={!settings.digestEnabled}
                onChange={(v) => {
                  setSettings((prev) => (prev ? { ...prev, digestSchedule: v } : prev));
                  debouncedSave("digestSchedule", () =>
                    void save({ digestSchedule: v }, setDigestSaveState, setDigestSaveError)
                  );
                }}
                hueIndex={hueIdx}
              />
            </div>
          </Card>
        );
      })()}

      {/* NOTIFICATIONS — Overdue-backup watchdog: a fixed daily check (09:00)
          that pushes ONE notification per overdue episode through the channels
          configured above; a new successful backup re-arms it. */}
      {tab === "notifications" && (
        <Card title={t("settings.watchdogTitle")} hint={t("settings.watchdogHint")} hueIndex={nextHue()}>
          {/* Full-page Speichern-Button sweep: was this Card's own bottom
              SaveBar — a single toggle, so it now just auto-saves itself. */}
          <ToggleRow
            label={t("settings.watchdogToggle")}
            checked={settings.watchdogEnabled}
            onChange={(v) => void autoSaveToggle("watchdogEnabled", v, setWatchdogSaveState, setWatchdogSaveError)}
            disabled={fieldBusy.watchdogEnabled}
            shakeNonce={fieldShake.watchdogEnabled}
            pulseNonce={fieldPulse.watchdogEnabled}
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
      {tab === "system" && advanced && (() => {
        // Button-size/colour-engine sweep (jdp, live review — "Die vielen
        // Buttons sind unterschiedlich groß und nicht alle im
        // Regenbogenmodus"): the Check Now button inside SpikePanel had no
        // tie to this Card's own hueIndex at all. `hueIdx` captured once in
        // this IIFE and threaded into BOTH the Card's own heading notch and
        // SpikePanel's new `hueIndex` prop — the same "one Card, two
        // hue-aware children share ONE position" shape the schedulesChecks
        // Card's own IIFE below already uses for its Card+CadenceBuilder
        // pair, not a second independent `nextHue()` call.
        const hueIdx = nextHue();
        return (
          <Card title={t("spike.title")} hueIndex={hueIdx}>
            <SpikePanel t={t} hueIndex={hueIdx} />
          </Card>
        );
      })()}

      {/* ------------------------------------------------------------------ */}
      {/* INTEGRITY — Integrity, maintenance & restore drills                 */}
      {/* Default-visible (v4): manual restore drills — including the real     */}
      {/* off-site DR restore — are part of the core ransomware-protection     */}
      {/* flow, alongside the un-gated off-site + retention cards above.       */}
      {/* ------------------------------------------------------------------ */}
      {tab === "integrity" && (
      <>
        {/* IntegrityCard used to be documented here as the ONLY Card this tab
            ever rendered — a genuine singleton per design-language's own
            exclusion ("the only one of its kind on the page keeps the single
            accent"), so it deliberately took no `hueIndex` at all. That
            exemption no longer applies (jdp, live-review — "Gehört die
            'Automatische Restore-Prüfungen' Card nicht in den
            Integritäts-Tab?"): RestoreChecksSection and the schedulesChecks
            Card below moved here from the Schedules tab, both configuring
            WHAT gets verified and how often — a natural fit next to this
            Card's own verify/unlock/prune/drill actions. With three Cards
            now genuinely on this tab, IntegrityCard gets a real `nextHue()`
            call like everything else, first in visual order since it's the
            tab's primary/pre-existing content. */}
        <IntegrityCard t={t} settings={settings} setSettings={setSettings} save={save} hueIndex={nextHue()} />

        {/* Restore-check drills (RestoreChecksSection renders its own Card) —
            moved from the Schedules tab (see that tab's own comment at its
            old call site). */}
        <RestoreChecksSection
          settings={settings}
          update={scheduleUpdate}
          busy={schedFieldBusy}
          shake={schedFieldShake}
          pulse={fieldPulse}
          t={t}
          hueIndex={nextHue()}
        />

        {/* Restore-check schedule (schedulesChecks): the scheduled off-site
            append-only tamper test — moved from the Schedules tab (see that
            tab's own comment at its old call site).
              `hueIdx` captured once in this IIFE and reused for both the
            Card's own heading notch and the CadenceBuilder's TimePicker
            inside it (Task 3, jdp: "Der Zeitpicker ist nicht im
            Regenbogenmodus") — a bare inline `<Card hueIndex={nextHue()}>`
            here has no local variable to also hand the CadenceBuilder below,
            and calling `nextHue()` a second time would consume a SECOND,
            different position for one visually-grouped Card (exactly the
            trap SaveBar's own header comment already warns about for the
            identical "one Card, two hue-aware children" shape). The IIFE is
            the smallest change that captures the single call's result
            without lifting this ad-hoc Card block into its own named
            component purely to receive a prop. */}
        {(() => {
          const hueIdx = nextHue();
          return (
            <Card title={t("settings.schedulesChecks")} hueIndex={hueIdx}>
              {/* Resolved-schedule badge — NEW this round, the third and last
                  cadence editor that had none (see RestoreChecksSection's own
                  comment for why CadenceBuilder's inline preview could only be
                  deleted once all three had one). NO `enabled` prop here,
                  unlike the other two: this Card has no on/off toggle of its
                  own — the cadence string's own "off" mode IS the control, the
                  same shape the four domain Cards use. The separate
                  `tamperScheduleActive` precondition below is deliberately NOT
                  folded into the badge: it isn't this card's own on/off but a
                  cross-cutting "no qualifying domain configured" state, and it
                  already has its own explicit amber explanation right beneath
                  (#109 — the one place that told manilx why Sun 08:00 never
                  ran). Restating it as a grey "Kein Zeitplan" badge would
                  contradict the cadence the user can plainly see set in the
                  editor. */}
              <ScheduleRow schedule={settings.tamperTestSchedule} />
              <div className="rounded-card bg-carbon-surface2 p-4">
                <CadenceBuilder
                  label={t("settings.tamperTestSchedule")}
                  value={settings.tamperTestSchedule}
                  onChange={(v) => scheduleField("tamperTestSchedule", v)}
                  hueIndex={hueIdx}
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
          );
        })()}
      </>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Security                                                  */}
      {/* ------------------------------------------------------------------ */}
      {/* Button-size/colour-engine sweep (jdp, live review — "Die vielen
          Buttons sind unterschiedlich groß und nicht alle im
          Regenbogenmodus"): the Save/Logout/Logout-everywhere buttons below
          had no tie to this Card's own hue at all. IIFE captures `hueIdx`
          once and reuses it for both the Card's own heading notch and every
          button inside it — the same "one Card, several hue-aware children
          share ONE position" shape the schedulesChecks/Spike Cards above
          already use, not several independent `nextHue()` calls. */}
      {tab === "system" && (() => {
        const hueIdx = nextHue();
        const hueStyle = hueVars(rainbowAt(hueIdx)) as CSSProperties;
        return (
      <Card title={t("auth.security")} hint={t("auth.passwordHint")} hueIndex={hueIdx}>
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
              key={pwSaveShake || 0}
              onClick={() => void handleSetPassword()}
              disabled={pwSaveState === "saving"}
              className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 glim-hue${
                pwSaveShake ? " glim-shake" : ""
              }`}
              style={hueStyle}
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
              className="rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors glim-hue"
              style={hueStyle}
            >
              {t("auth.logout")}
            </button>
            <button
              onClick={() => void handleLogoutAll()}
              className="rounded-control bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors glim-hue"
              style={hueStyle}
            >
              {t("settings.logoutAll")}
            </button>
          </div>
        )}
      </Card>
        );
      })()}

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
      {/* Security Card + Settings Portability Card further down — AboutFooter, */}
      {/* the system tab's old THIRD tab-conditioned element, has since moved  */}
      {/* out of this repeated-per-Card condition entirely; see its own header */}
      {/* comment), not a wrapping Fragment introduced just for this section.  */}
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
            `variant="well" equalWidth` (GlimStone follow-up pass,
          live-review point 7 — "turn the shape picker into a horizontal
          selector styled like the one in TrickWork"): the FIRST call site to
          exercise Selector's grooved variant (components/Selector.tsx's own
          file header, item 5) — TrickWork's shared padded background with
          flush, crossfade-only segments, no sliding pill. Picked for that
          first try specifically because it's already icon-free (no glyph
          competing with the groove's own look) and already the page's most
          "three mutually exclusive settings, read together as one control"
          Selector — the shape it suits best. A LATER round gave the Theme
          Card's own light/dark picker (above) this exact same treatment, and
          round 8 spread the variant itself (minus `equalWidth`) to every
          small in-card selector in the app. The 7-tab strip above stays on
          `variant="chip"` — it is a tab strip of individual badges, not a
          grooved segmented control; see Selector.tsx's item 5b. */}
      {tab === "general" && (
      <Card title={t("settings.shape")} hint={t("settings.shapeHint")} hueIndex={nextHue()}>
        {/* No "don't stretch" wrapper div here any more — `variant="well"`
            carries `w-fit max-w-full` itself as of round 8, which opts the
            row out of this Card's `flex flex-col` default
            `align-items: stretch` without an extra element. See the Theme
            Card's own Selector above for the full note. */}
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
          equalWidth
        />
      </Card>
      )}

      {/* Motion intensity (GlimStone motion-engine — jdp, live-review:
          "Wäre eine Animationsengine gut?" -> "Echte Engine mit eigenem
          Nutzer-Schalter"). A DELIBERATE reversal of design-language.md's
          own prior Motion-Engine section (2026-08-18: "kein In-App-Schalter
          dafür ... kein fünfter Nutzer-Schalter, rein OS-gesteuert für
          jetzt") — see that doc's updated Motion Intensity write-up for the
          full course-correction note, quoting the old text rather than
          silently dropping it.
            Same architecture as the Shape Card right above (lib/motion.ts
          mirrors lib/shape.ts's getShape/setShape/applyShape/
          applyStoredShape exactly; index.css's `[data-motion="..."]` token
          blocks mirror `[data-shape="..."]`'s own), so this Card sits
          directly below Shape: same kind of setting (client-only, applied
          at the app root), same "one Selector, no Save step" shape, same
          `variant="well" equalWidth`/`size="lg"` treatment already proven
          live on Theme's and Shape's own pickers right above.
            `hue` stays on its plain `true` default (Selector's own
          default) — this repo's standing colour-engine rule is explicit
          that "it's a settings control, not content" is exactly the kind
          of self-authored exception that rule forbids; this Selector's
          three segments read RAINBOW[0]/[1]/[2] like Shape's own segments
          right above, and the Card's own heading badge gets a real
          `hueIndex={nextHue()}` the same way every other Card on this tab
          does. */}
      {tab === "general" && (
      <Card title={t("settings.motion")} hint={t("settings.motionHint")} hueIndex={nextHue()}>
        {/* No "don't stretch" wrapper div, same as the Theme/Shape Selectors
            right above — `variant="well"` hugs its own segments now. */}
        <Selector
          items={MOTION_INTENSITIES.map((m) => ({
            id: m,
            label: t(`settings.motion.${m}` as TranslationKey),
          }))}
          label={t("settings.motion")}
          select="one"
          active={motion}
          onChange={(id) => {
            setMotionLocal(id as MotionIntensity);
            setMotionIntensity(id as MotionIntensity);
          }}
          size="lg"
          variant="well"
          equalWidth
        />
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
          on that ToggleRow below — see its own comment for both.
            IIFE-captured `hueIdx` feeds this Card's own heading notch plus
          the three rainbow ToggleRows below (hueIndex 0/1/2) — the same
          one-call-feeds-several-children shape the schedulesSelfBackup Card
          (Card+CadenceBuilder) and every offsite per-domain Card above
          already use, so a bare inline `hueIndex={nextHue()}` on the Card
          alone doesn't also have to be re-derived at each child call site.
            This Card's own two reset Badges (the accent-preset reset inside
          AccentCard below, and the rainbow-palette reset further down) are
          DELIBERATELY NOT among hueIdx's consumers — both are `tone="neutral"`,
          not hue-tinted, on purpose (see each Badge's own call-site comment
          for the full "a reset control must not blend into the very colours
          it resets" reasoning), so neither reads `hueIdx` at all. */}
      {tab === "general" && (() => {
      const hueIdx = nextHue();
      // "Is there anything left to reset?" for the palette row below — the
      // mirror of AccentCard's own `presetsAreDefault`, same case-insensitive
      // comparison (setRainbow()/isValidPalette() accept either case, so a
      // palette restored by hand as "#ff8389" must still count as default).
      const paletteIsDefault =
        rainbow.palette.length === RAINBOW.length &&
        rainbow.palette.every((hex, i) => hex.toLowerCase() === RAINBOW[i]?.toLowerCase());
      return (
      <Card title={t("settings.colors")} hueIndex={hueIdx}>
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
              …), so the key stays in every locale unchanged.
                Live-review round 4 REVERSES the point above (jdp: "Bei der
              Rainbow-Farbpalette soll 'Farbpalette' stehen und dann die
              Farbfelder kommen") — a caption is back after all, just a
              different string than the one removed: settings.rainbowPalette
              ("Palette colour") stays a per-swatch aria-label only, unchanged;
              this new settings.rainbowPaletteLabel ("Colour palette") is a
              standalone row-opening label, matching how the Accent row right
              above it now opens with its own "Akzentfarbe" caption before
              its controls — same "label first" ordering, so the two rows in
              this merged Card read as one consistent pair rather than the
              swatch row being the odd one out again.
                Label now bare, no trailing colon (jdp, live-review: "Der
              Doppelpunkt nach Akzentfarbe und Farbpalette weg") — the colon
              was appended in JSX only, never in the translated string (all
              42 locales checked), so removing it here is the whole fix.
                Swatches+reset now right-aligned as their own `ms-auto` group
              (jdp, live-review: "Die Farbfelder der Farbpalette auch ganz
              nach rechts verschieben"), matching the Accent row's own
              identical right-alignment right above — the label stays at the
              row's start, everything clickable moves to the row's end. */}
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm text-carbon-text">{t("settings.rainbowPaletteLabel")}</span>
            <div className="flex items-center gap-2 flex-wrap ms-auto">
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
                clickable is a badge" convention (Task 5 rule 13), sized via
                the `icon` stage to land on the exact same 28px (h-7 w-7)
                footprint as the PaletteSwatch circles it sits beside, so it
                reads as part of the same row of controls rather than a
                mismatched afterthought.
                  SQUARE (jdp, live-review: "Die Zurücksetzen-Option soll ein
                quadratischer Badge mit Glyph sein") — `shape="square"` still
                resolves through `rounded-control`, the shape engine's own
                live token, so this genuinely tracks round/soft/square (under
                "Rund" it renders as a full circle, same as every other
                square badge in the app).
                  NEUTRAL now, not hue-tinted (jdp, re-reporting: "Der
                Reset-Badge soll ... nicht farbig sein, damit er sich besser
                von den Farbflächen abhebt" — measured live, this Badge's
                `tone="active"` fill was rendering the literal flat accent
                gold, `rgb(252,196,25)` — the EXACT SAME value as one of the
                eight palette swatches sitting right beside it, making the
                reset control indistinguishable from an actual colour choice
                at a glance). DELIBERATE EXCEPTION to this app's standing
                "every icon-only badge gets real colour-engine integration"
                rule — see AccentCard's mirror reset Badge (Settings.tsx,
                same file, a few hundred lines up) for the full "a reset
                control sitting beside the very colours it resets must not
                itself be one of those colours" reasoning, which applies
                here identically. `border-2 border-carbon-border` (NEW)
                matches PaletteSwatch's own border exactly (`h-7 w-7 ...
                border-2 border-carbon-border` a few dozen lines up) — without
                it this Badge's solid fill filled the full 28×28 box edge to
                edge while each PaletteSwatch's own visible colour disc is
                actually only 24×24 (28px border-box minus its own 2px
                ring), which is what actually made this control read as
                BIGGER than its neighbours (jdp: "der Reset-Badge ist größer
                als die Farbfelder") despite an IDENTICAL measured 28×28
                bounding box — a real optical-weight defect a bounding-box
                check alone never catches, now closed by giving this Badge
                the exact same border every swatch beside it already has.
                  IconResetArrow (redesigned — see that icon's own header
                comment) — the established counter-clockwise "reset" arrow
                convention, redrawn bolder for legibility at this small
                badge-in-a-busy-row size.
                  DISABLED ONLY WHEN THERE IS NOTHING TO RESET — the same
                dead-control audit that unified AccentCard's two resets above
                was run against this badge, and it had a milder version of the
                same defect from the other direction: gated on `!rainbow.on`
                alone, it sat ENABLED whenever rainbow mode was on even with
                the palette already byte-identical to RAINBOW, so a click was
                a silent no-op — a control that looks live and does nothing is
                the same broken promise as one that is permanently greyed out,
                just harder to notice. `paletteIsDefault` (computed beside
                `hueIdx` at this Card's own IIFE head) closes that: the badge
                is now live exactly when a click would actually change
                something. The `!rainbow.on` half of the gate STAYS — the
                eight PaletteSwatches next to it carry the identical
                `disabled={!rainbow.on}`, so the whole row switches off
                together, which reads as "this section is off" rather than as
                one arbitrarily dead control among live ones (the very
                confusion that made AccentCard's badge unreadable).
                  size="icon" — the app's ONE square-icon-badge size (32px),
                never re-derived from the swatch box model. It was 28px, sized
                to this row's swatches; the app-wide unification moved the
                swatches instead (PaletteSwatch h-7 w-7 → h-8 w-8), so this
                badge and its eight neighbours still share one measured
                footprint and one 28px inner disc inside their `border-2`.
                  tip (not title/ariaLabel) — IconTipButton's real hover/focus
                bubble. Now settings.rainbowPaletteReset ("Reset color
                palette"), not the generic common.reset it carried before:
                after the merge above, this Card holds TWO neutral square
                reset badges a few rows apart, and two identical "Reset"
                bubbles on two controls with different targets is exactly the
                ambiguity the accentPresetsReset key was originally introduced
                to avoid. Each bubble now names its own target. (common.reset
                itself had no other reader left once the accent row's text
                button was deleted, and was dropped from all 42 locales.)
                Kept shape/tone/size/glyph/border identical to the AccentCard
                mirror above — the established "these two mirror each
                other" pairing. */}
            <Badge
              as="button"
              shape="square"
              size="icon"
              tone="neutral"
              className="border-2 border-carbon-border"
              disabled={!rainbow.on || paletteIsDefault}
              onClick={() => updateRainbow({ palette: RAINBOW })}
              tip={t("settings.rainbowPaletteReset")}
            >
              <IconResetArrow />
            </Badge>
            </div>
          </div>
        </div>
      </Card>
      );
      })()}

      {/* Quiet toasts (GlimStone form-engine Task 9) — the toast system's
          severity-based quiet mode. Its own Card now (previously the last,
          divider-less sub-topic tacked onto the shared Appearance Card), next
          to the other purely client-side display preferences, rather than
          being bolted onto NotifyConfig's server-side "on" field above (that
          one gates external webhook/Matrix/email notifications — a different
          axis entirely; muting a toast in THIS browser must never silently
          change what a webhook receives elsewhere).
            No-empty-toggles audit (jdp): this row used to `hideLabel`
          because the Card's own title already says "Quiet toasts" — the
          exact single-purpose-Card pattern the merged Colors Card's master
          "Regenbogen-Modus" toggle and RestoreChecksSection's "Automatische
          Restore-Prüfungen" toggle already had reversed, and this one was
          the leftover fourth instance the full-app grep in this pass caught.
          `hideLabel` is gone from ToggleRow entirely now (see its own header
          comment) — the label is visible again.
            "Explanations belong in a bubble" pass (jdp): what explains this
          toggle was a permanent `description` caption printed under the row
          on every load, not an explanation gated behind the (i) affordance
          the rest of this page already uses — the exact anti-pattern
          Apprise's own ToggleRow comment above documents fixing the same way.
          Moved verbatim into `hint` instead (ToggleRow's own InfoBubble prop,
          same content contract as Card's `title`/`hint` pair): no wording
          change needed on either the EN source string or its DE translation
          — both were already a single compact two-sentence explanation, well
          within the register settings.offsiteDrillsHelp's own much longer
          hint text already establishes as normal for this bubble, so only
          the display mechanism moved, not the copy. Only this call site's
          own prop changed; the shared `settings.quietToastsHint` key and its
          text are untouched in i18n.ts and all 40 satellite locale files. */}
      {tab === "general" && (
      <Card title={t("settings.quietToasts")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.quietToasts")}
          hint={t("settings.quietToastsHint")}
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
      </div>

      {/* Version + report-a-bug footer. Rendered ONCE, unconditionally, here
          — outside the `key={tab}` tab-panels wrapper above (so it doesn't
          remount, and re-fetch the version, on every tab switch) and outside
          every `{tab === "x" && ...}` conditional (so it's not tied to any
          one tab at all). A perfectly ordinary last child of the page root
          now, in normal document flow — no portal, no `position: fixed` —
          the tab-panels wrapper's own `flex-1` (its comment above) is what
          pushes it down to the bottom of the column; see AboutFooter's own
          header comment for the full sticky-footer mechanism and why that
          replaces the earlier fixed+portal version. */}
      <AboutFooter />
    </div>
  );
}
