// The pieces every Settings card needs ([337]).
//
// Settings.tsx had grown to 9,841 lines against a 3,242-line runner-up. That
// is not a style problem: every reported issue on that page began with
// finding the card it belonged to. Splitting the cards out needs a shared
// module first, because Card and ToggleRow were defined inside the file the
// cards were leaving.
//
// Moved verbatim, comments included. No behaviour changes here at all.

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

import { Badge } from "../../components/Badge";
import { ColorPickerSwatch } from "../../components/ColorPickerPopover";
import { InfoBubble } from "../../components/InfoBubble";
import { Toggle } from "../../components/Toggle";
import { hueVars, rainbowAt } from "../../lib/appearance";
import { type CSSProperties } from "react";
import { useT } from "../../lib/i18n";

export type SaveState = "idle" | "saving" | "saved" | "error";

// ---------------------------------------------------------------------------
// Card wrapper
// ---------------------------------------------------------------------------

export function Card({
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
export function AccentPresetSwatch({
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

// ---------------------------------------------------------------------------
// Toggle row
// ---------------------------------------------------------------------------

export function ToggleRow({
  label,
  hint,
  checked,
  onChange,
  disabled,
  shakeNonce,
  pulseNonce,
  hueIndex,
}: {
  label: string;
  // `description` — a permanent grey caption under the row — is deliberately
  // GONE, not merely unused. Rule 8 ("explanations live in a bubble, not on the
  // page") had already moved ~40 rows onto `hint`, and the last two survivors
  // sat on Config.tsx's Selbst-Backup card, whose own heading text the same
  // sweep did convert. Leaving the prop behind would have left the pattern one
  // autocomplete away; removing it makes the next attempt a type error instead
  // of a review comment. A row that genuinely needs prose next to it has `hint`.
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
