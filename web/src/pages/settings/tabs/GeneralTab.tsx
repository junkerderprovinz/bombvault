import { AccentCard, IconResetArrow } from "../AccentCard";
import { LanguageCard } from "../LanguageCard";
import { ThemeCard } from "../ThemeCard";
import { CONTROL_AXES, LABEL_MODES, setLabelMode, type LabelMode } from "../../../lib/controls";
import { labelModeChanged } from "../../../lib/useLabelMode";
import { Badge } from "../../../components/Badge";
import { useT, type TranslationKey } from "../../../lib/i18n";
import { tLtr } from "../../../lib/ltrFragments";
import { ColorPickerSwatch } from "../../../components/ColorPickerPopover";
import { RAINBOW } from "../../../lib/appearance";
import { SHAPES, setShape, type Shape } from "../../../lib/shape";
import { MOTION_INTENSITIES, setMotionIntensity, stormTap, type MotionIntensity } from "../../../lib/motion";
import { setDisco } from "../../../lib/disco";
import { HUE_OFFSET, Selector } from "../../../components/Selector";
import { Card, ToggleRow } from "../shared";
import { AboutCard } from "../AboutCard";
import type { SettingsTabProps } from "./types";

// Palette swatch: one editable colour in the rainbow palette editor
// (GlimStone form-engine Phase 2, Task 1). Deliberately matches the existing
// accent-preset swatches' own visual language above (a rounded-pill circle
// showing the colour, a border) rather than introducing a new component
// family: ColorPickerSwatch (the shared GlimStone-picker trigger, see its
// own header comment) opens the same floating popover the Accent Card's
// custom swatch below uses, pre-synced to this position's own value, instead
// of a native <input type="color"> (a genuinely separate browser/OS window,
// jdp: "kein eigenes Fenster welches sich öffnet"). `disabled` dims the
// control on its OWN element (native `disabled` + `disabled:opacity-50`),
// never via a wrapping container's opacity (rule 15 / this branch's own
// established "dimmed via disabled, not opacity-on-container" fix from Phase
// 1 Task 4), ColorPickerSwatch's own `disabled` prop follows that same
// contract.
//
// `rounded-pill`, not a hardcoded `rounded-full` (GlimStone follow-up pass,
// live-review point 4): a literal `rounded-full` is a fixed 50% radius that
// never moves, so this swatch (and the accent-preset swatches below) used to
// stay a perfect circle no matter what shape the user picked in Settings,
// the one pair of controls on this page that silently ignored the shape
// engine. `rounded-pill` is the SAME token every pill-shaped Badge already
// reads (Badge.tsx's shape="pill"/"circle" -> RADIUS_CLASSES.pill/circle),
// and both swatches here are already fixed equal-width/-height boxes, so no
// `aspect-square` is needed the way Badge's circle shape needs one, points
// at var(--radius-pill), which index.css already varies per data-shape:
// 9999px (round, a true circle), 0.3125rem (soft, a lightly rounded square),
// 0 (square, a hard corner), reactive with zero new CSS.
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
  // badge, not the other way round, jdp has twice reported this row when the
  // two disagreed ("der Reset-Badge ist größer als die Farbfelder"), so they
  // are kept equal by moving whichever side is not bound by the app-wide rule.
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

export function GeneralTab({
  t,
  quiet,
  setQuiet,
  settings,
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
  domainToggleBusy,
  domainToggleShake,
  fieldPulse,
  toggleDomainEnabled,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* ------------------------------------------------------------------ */}
      {/* GENERAL: Domains                                                    */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.domains")} hint={t("settings.domainsHint")} hueIndex={nextHue()}>
        {/* Live-review round 3, point 4: all 7 rows here used to show a
            permanent visible caption under the label (rule 8 violation, the
            first 5 were even raw hardcoded English strings, never localized
            at all). Every row now carries its explanation via ToggleRow's
            `hint` prop (an InfoBubble beside the label) instead, receiver/
            fleet already had a real i18n key for their caption and just
            needed the prop swapped; containers/vms/flash/files/config
            needed a NEW *Hint key added (and translated into all 26
            locales) since their old text was never a translation key.

            #142 (jdp, live review): "Bei Domänen der Speichern-Button
            entfernen, es soll automatisch speichern und den Tab live
            einblenden/ausblenden", no more batched SaveBar. Each row now
            calls toggleDomainEnabled directly: optimistic flip, persist via
            the shared save() (which already broadcasts
            "bv:settings-changed" so Layout/Sidebar re-fetch and the domain's
            nav tab appears/disappears live, no reload), and on a rejected
            save, e.g. enabling VMs with no working SSH connection to the
            libvirt host, revert to the pre-click state and shake. `disabled`
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
          hint={tLtr(t, "settings.flashEnabledHint")}
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
        {/* Pull (#227). Last of the three and the only one that WRITES: the two
            above watch, this one fetches another instance's backups into this
            box's own repository. Its hint says so rather than leaving it to be
            discovered. */}
        <ToggleRow
          label={t("settings.pullEnabled")}
          hint={t("settings.pullEnabledHint")}
          checked={settings.pullEnabled}
          onChange={(v) => void toggleDomainEnabled("pullEnabled", v)}
          disabled={domainToggleBusy.pullEnabled}
          shakeNonce={domainToggleShake.pullEnabled}
          pulseNonce={fieldPulse.pullEnabled}
          hueIndex={7}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL: Language (GlimStone follow-up pass, live-review point 9).  */}
      {/* Moved out of Sidebar.tsx's footer, see LanguageCard's own header     */}
      {/* comment above for the full move rationale. Sits right after Domains */}
      {/* (a fundamental, whole-app setting, same register) and right before  */}
      {/* the purely-cosmetic Appearance cluster below.                       */}
      {/* ------------------------------------------------------------------ */}
      <LanguageCard t={t} hueIndex={nextHue()} />

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL: Theme (GlimStone follow-up pass, later live-review round).  */}
      {/* Moved out of Sidebar.tsx's footer, see ThemeCard's own header        */}
      {/* comment above. Same register and immediately below Language: both   */}
      {/* are fundamental, whole-app identity settings (not one of the purely */}
      {/* cosmetic Appearance sub-topics below), so this Card sits right      */}
      {/* after Language and right before Accent colour.                      */}
      {/* ------------------------------------------------------------------ */}
      <ThemeCard t={t} hueIndex={nextHue()} />

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL: Appearance                                                */}
      {/* GlimStone follow-up pass, live-review point 5: this used to be ONE  */}
      {/* shared Card with four sub-topics (accent / shape / rainbow / quiet  */}
      {/* toasts) separated by `border-t border-carbon-border` divider lines, */}
      {/* a real, previously-unnoticed violation of this app's own "never a   */}
      {/* border line, only shade/shadow" house rule (see index.css's shape-  */}
      {/* token comments and Badge.tsx's file header: every OTHER visual      */}
      {/* separation in this app comes from a surface's own elevation, not a  */}
      {/* rule). Each of the four became its OWN Card that round, same        */}
      {/* bg-carbon-surface + rounded-card + shadow every other Settings      */}
      {/* topic already renders through, no divider needed because there's   */}
      {/* no longer a shared surface to divide.                              */}
      {/*   LATER live-review round (jdp: "Die card von Akzentfarbe und       */}
      {/* Regenbogenmodus in eine mergen. Gehört ja zusammen"): accent and    */}
      {/* rainbow are back to ONE Card below, see that Card's own header      */}
      {/* comment for the merge, the hint relocation, and the hue-integration  */}
      {/* fixes that landed in the same pass. Shape and Quiet toasts stay     */}
      {/* their own separate Cards; `settings.appearance` (the old umbrella   */}
      {/* title from the FOUR-way split) still has no call site and stays     */}
      {/* removed from every locale rather than kept as a dead key. Same      */}
      {/* "general" tab condition repeated per Card, the pattern every OTHER  */}
      {/* multi-Card tab on this page already uses (e.g. the "system" tab's   */}
      {/* Security Card + Settings Portability Card further down, AboutFooter,  */}
      {/* the system tab's old THIRD tab-conditioned element, has since moved  */}
      {/* out of this repeated-per-Card condition entirely; see its own header */}
      {/* comment), not a wrapping Fragment introduced just for this section.  */}
      {/* ------------------------------------------------------------------ */}

      {/* Shape (GlimStone form-engine, shape engine; design-language.md's
          "The user-owned axes": data-shape on <html>, round/soft/square,
          one radius token set driving every rounded corner). lib/shape.ts is
          the JS half (read/write/persist which of the three is chosen, stamp
          the attribute), index.css already carries the matching
          [data-shape="soft"|"square"] radius-token overrides. Lives directly
          above the merged Colors Card below: same kind of setting
          (client-only, applied at the app root, shape.ts's own header
          comment), same "one picker, no Save step" shape.
            Selector, not a bespoke button row: this IS "three mutually
          exclusive options" (design-language.md's "The one horizontal
          selector"), the exact shape Dashboard.tsx's heatmap-domain toggle
          already uses this component for.
            REVERSED (jdp, live-review, extremely emphatic standing rule:
          "Der horizontale Selektor der Ecken ist nicht im Regenbogen-Modus
          integriert... Es soll immer alles in die Farb- und Formengine
          integriert werden!! IMMER!!"): this used to carry `hue={false}`,
          reasoned at the time as "round/soft/square are a form choice, not a
          position in a list, and tinting the segments would compete with
          the choice itself." That is exactly the kind of self-authored
          aesthetic exception jdp has now ruled out categorically, a
          plausible-sounding taste judgement is never grounds to unilaterally
          exclude a control from the colour engine. `hue` now stays on its
          plain `true` default, so this Selector's three segments read
          RAINBOW[0]/[1]/[2] like any other hue-enabled Selector in the app,
          see Selector.tsx's own file header item 1 for the full reversal
          note (Dashboard's heatmap toggle got the identical fix in the same
          pass).
            `size="lg"` (GlimStone follow-up pass, live-review point 1,
          up from the original "sm"): this is a full, standalone Settings
          decision in its own right, the same visual register as the page's
          OWN 7-tab Selector strip further up this file (also `size="lg"`),
          not a tight toolbar chip like Dashboard's heatmap toggle or
          CadenceBuilder's weekday pills: "sm" undersold it next to
          everything else in this Card.
            No `icon` per item anymore (live-review point 2): the original
          per-option glyph (a small outlined square drawn at a SCALED-DOWN
          6px/2px/0 preview radius, deliberately not the real 10px/5px/0
          --radius-control values, for legibility at 14px) turned out to
          undercut its own point live: a smaller-than-real preview sitting
          right next to the label read as "round isn't very round," the
          opposite of what it was meant to show. Text-only avoids that
          entirely, the real Selector segment the user is looking at IS the
          shape preview, at its own true radius, with no scaled-down stand-in
          competing with it.
            `variant="well" equalWidth` (GlimStone follow-up pass,
          live-review point 7: "turn the shape picker into a horizontal
          selector styled like the one in TrickWork"): the FIRST call site to
          exercise Selector's grooved variant (components/Selector.tsx's own
          file header, item 5), TrickWork's shared padded background with
          flush, crossfade-only segments, no sliding pill. Picked for that
          first try specifically because it's already icon-free (no glyph
          competing with the groove's own look) and already the page's most
          "three mutually exclusive settings, read together as one control"
          Selector, the shape it suits best. A LATER round gave the Theme
          Card's own light/dark picker (above) this exact same treatment, and
          round 8 spread the variant itself (minus `equalWidth`) to every
          small in-card selector in the app. The 7-tab strip above stays on
          `variant="chip"`, it is a tab strip of individual badges, not a
          grooved segmented control; see Selector.tsx's item 5b. */}
      <Card title={t("settings.shape")} hint={t("settings.shapeHint")} hueIndex={nextHue()}>
        {/* No "don't stretch" wrapper div here any more, `variant="well"`
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
          hueOffset={HUE_OFFSET.shape}
        />
      </Card>

      {/* Motion intensity (GlimStone motion-engine, jdp, live-review:
          "Wäre eine Animationsengine gut?" -> "Echte Engine mit eigenem
          Nutzer-Schalter"). A DELIBERATE reversal of design-language.md's
          own prior Motion-Engine section (2026-08-18: "kein In-App-Schalter
          dafür ... kein fünfter Nutzer-Schalter, rein OS-gesteuert für
          jetzt"), see that doc's updated Motion Intensity write-up for the
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
          default), this repo's standing colour-engine rule is explicit
          that "it's a settings control, not content" is exactly the kind
          of self-authored exception that rule forbids; this Selector's
          three segments read RAINBOW[0]/[1]/[2] like Shape's own segments
          right above, and the Card's own heading badge gets a real
          `hueIndex={nextHue()}` the same way every other Card on this tab
          does. */}
      <Card title={t("settings.motion")} hint={t("settings.motionHint")} hueIndex={nextHue()}>
        {/* No "don't stretch" wrapper div, same as the Theme/Shape Selectors
            right above, `variant="well"` hugs its own segments now. */}
        {/* THE FOURTH SEGMENT IS NOT ALWAYS THERE, GSS 1.17.0's hidden level.
            It is offered while it is CHOSEN - a picker that hid the value it
            is currently showing would be lying about the interface - and
            otherwise only for as long as this screen stays open, which is why
            `stormFound` is component state and never storage. Pick something
            else and leave, and it is gone until the gesture is made again. */}
        <Selector
          items={[...MOTION_INTENSITIES, ...(stormFound || motion === "storm" ? (["storm"] as const) : [])].map(
            (m) => ({
              id: m,
              label: t(`settings.motion.${m}` as TranslationKey),
            }),
          )}
          label={t("settings.motion")}
          select="one"
          active={motion}
          onChange={(id) => {
            // The gesture first, because it fires on the level ALREADY chosen
            // and therefore on a click that changes nothing else.
            const storm = stormTap(stormClicks.current, id, motion);
            if (storm) setStormFound(true);
            const next = (storm ?? id) as MotionIntensity;
            setMotionLocal(next);
            setMotionIntensity(next);
          }}
          size="lg"
          variant="well"
          equalWidth
          hueOffset={HUE_OFFSET.motion}
        />
      </Card>

      {/* Control labels (#178): how much of a control's identity is shown.
          Three axes rather than one switch, because the right answer differs
          per axis: a sidebar reduced to glyphs narrows the whole page, tabs do
          not, and action buttons are a density preference. jdp asked for one
          selector each, sharing the same three options.
          Placed straight after Animations on purpose: both are per-viewer
          appearance dials kept in this browser rather than server settings,
          and they read as a pair. */}
      <Card title={t("settings.labels")} hint={t("settings.labelsHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-4">
          {CONTROL_AXES.map((axis, axisIndex) => (
            <div key={axis} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">
                {t(`settings.labels.${axis}` as TranslationKey)}
              </span>
              <Selector
                items={LABEL_MODES.map((m) => ({
                  id: m,
                  label: t(`settings.labels.mode.${m}` as TranslationKey),
                }))}
                label={t(`settings.labels.${axis}` as TranslationKey)}
                select="one"
                active={labelModes[axis]}
                onChange={(id) => {
                  setLabelMode(axis, id as LabelMode);
                  setLabelModes((prev) => ({ ...prev, [axis]: id as LabelMode }));
                  // Every mounted control re-reads on this, so the page changes
                  // under the selector instead of only after a reload.
                  labelModeChanged();
                }}
                size="lg"
                variant="well"
                equalWidth
                // Each of the three rows starts one colour further along the
                // palette (jdp, 2026-09-15: "nicht jeder selektor soll am
                // gleichen feld die gleiche farbe haben"). Offsetting by the
                // row index rather than by the row COUNT keeps neighbouring
                // rows adjacent in the palette, so the block still reads as
                // one group instead of three unrelated strips. Every other
                // start in the settings tree comes out of the same table.
                hueOffset={HUE_OFFSET.labels + axisIndex}
              />
            </div>
          ))}
        </div>
      </Card>

      {/* Colors (GlimStone form-engine Phase 2, Task 1; the accent Card and
          the Rainbow Card, MERGED, jdp, live-review: "Die card von
          Akzentfarbe und Regenbogenmodus in eine mergen. Gehört ja
          zusammen"). AccentCard above now returns just its own body (no
          Card wrapper of its own, see its header comment), composed here
          alongside the Rainbow controls this Card used to hold on its own.
          One heading, `settings.colors` ("Colours"/"Farben"), new key, not
          a repurposed `settings.accentColor`/`settings.rainbow`: those two
          stay in use as the sub-topics' own row labels below, so the Card's
          own title needed a THIRD string that reads as "colour, broadly"
          without clashing with either. No `hint` on the Card itself any
          more (see the master toggle below for where Rainbow's own hint
          moved). No divider between the two halves, spacing only, this
          app's established "cards separate sections, never a rule line"
          convention (see the Shape/Rainbow split's own comment above for
          the fuller house-rule writeup); AccentCard's body and the rainbow
          `<div>` below it are simply two direct children of this Card's own
          `flex flex-col gap-4`, the same "adjacent flex children, no divider"
          shape the (now-relocated) Flash-zip-export/Plain-export/Repository
          trio in the Storage tab's encryption Card already established.
            hueIndex: merging two Cards into one Card means one FEWER
          `nextHue()` call in the sequence than before, removed here rather
          than left as a dead call, since `hueSeq++` would otherwise burn a
          position nothing renders. Every Card below this one (Quiet toasts,
          the "system"/"storage"/etc. tabs' own Cards) is still numbered
          correctly with no manual re-numbering: `nextHue()` is a plain
          `hueSeq++` evaluated in JSX order at render time (see this
          function's own `hueSeq`/`nextHue` comment above), so removing one
          call site automatically shifts every LATER one down by one, the
          exact self-correcting behaviour that comment already documents.
            This switch genuinely repaints the app: every hue-enabled
          Selector segment (components/Selector.tsx, its own default,
          twelve call sites across seven files, including the Settings tab
          strip above and the drill-type toggle further down) and the
          container/VM/file-set list rows all read a rainbow position, so
          turning this on sets data-rainbow + --rb-0..--rb-7 on <html> AND
          immediately recolours those real call sites. The sidebar nav is
          deliberately NOT a consumer (Sidebar.tsx carries the reasoning), so
          flipping this switch never changes the rail's own colours.
            The master toggle's own hueIndex/hint fixes are documented right
          on that ToggleRow below, see its own comment for both.
            IIFE-captured `hueIdx` feeds this Card's own heading notch plus
          the three rainbow ToggleRows below (hueIndex 0/1/2), the same
          one-call-feeds-several-children shape the schedulesSelfBackup Card
          (Card+CadenceBuilder) and every offsite per-domain Card above
          already use, so a bare inline `hueIndex={nextHue()}` on the Card
          alone doesn't also have to be re-derived at each child call site.
            This Card's own two reset Badges (the accent-preset reset inside
          AccentCard below, and the rainbow-palette reset further down) are
          DELIBERATELY NOT among hueIdx's consumers, both are `tone="neutral"`,
          not hue-tinted, on purpose (see each Badge's own call-site comment
          for the full "a reset control must not blend into the very colours
          it resets" reasoning), so neither reads `hueIdx` at all. */}
      {(() => {
      const hueIdx = nextHue();
      // "Is there anything left to reset?" for the palette row below, the
      // mirror of AccentCard's own `presetsAreDefault`, same case-insensitive
      // comparison (setRainbow()/isValidPalette() accept either case, so a
      // palette restored by hand as "#ff8389" must still count as default).
      const paletteIsDefault =
        rainbow.palette.length === RAINBOW.length &&
        rainbow.palette.every((hex, i) => hex.toLowerCase() === RAINBOW[i]?.toLowerCase());
      return (
      <Card title={t("settings.colors")} hueIndex={hueIdx}>
        <AccentCard t={t} rainbowOn={rainbow.on} />
        <div className="flex flex-col gap-3">
          {/* Three toggles rendered together are a list, so each takes a
              rainbow position of its own, counted locally like the Domains
              card's rows. */}
          <ToggleRow
            label={t("settings.rainbow")}
            // Moved DOWN from the Card's own `hint` (jdp, live-review: "die
            // infobubble des regenbogenmodus ist unverständlich und sie soll
            // von titelbadge runterwandern in die toggle zeile"), same
            // `hint` prop mechanism the Reactive/Rotation rows below already
            // use for their own explanations. Text rewritten for this same
            // move: see settings.rainbowHint's own value for the rewrite
            // rationale (a fresh, concrete one-pass explanation, not the old
            // abstract "handed out by position" phrasing jdp found unclear).
            hint={t("settings.rainbowHint")}
            checked={rainbow.on}
            onChange={rainbowToggled}
            hueIndex={0}
          />

          {/* Everything below hangs off the rainbow being ON, so while it is
              off none of it is here at all (GlimStone 1.10.0). It used to be
              dimmed, under the reasoning "switched off, not hidden": leave it
              visible so nobody has to guess what the mode does. The language
              answers that directly - a palette editor under a rainbow that is
              not running is eight swatches nobody can open beside a reset
              nobody can press, and the switch above already says what the mode
              is. The "not hidden" rule protects the switch for the MODE, not
              its sub-controls.

              The accent row two cards up is deliberately NOT this case and
              keeps its dimming: its value still paints every control the
              rainbow does not reach, so it is a setting that is partly
              overridden rather than one with nothing behind it. GlimStone
              1.16.0 states the test - does the control still do anything. */}
          {rainbow.on && (
          <>
          {/* Disco, once found. Shown while it is ON as well as while found,
              for the reason the storm's own picker entry is: a switch that
              hid the value it is currently showing would be lying, and
              somebody who reloads with disco running needs a way to stop it.
              Sits first inside the rainbow's sub-controls because it changes
              what the whole set of them does, rather than one more property
              of it. */}
          {(discoFound || disco) && (
            <ToggleRow
              label={t("settings.disco")}
              hint={t("settings.discoHint")}
              checked={disco}
              onChange={(v) => {
                setDisco(v);
                setDiscoLocal(v);
              }}
              hueIndex={0}
            />
          )}
          <ToggleRow
            label={t("settings.rainbowReactive")}
            hint={t("settings.rainbowReactiveHint")}
            checked={rainbow.reactive}
            onChange={(v) => updateRainbow({ reactive: v })}
            hueIndex={1}
          />
          <ToggleRow
            label={t("settings.rainbowRotate")}
            hint={t("settings.rainbowRotateHint")}
            checked={rainbow.rotate}
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
              document.documentElement.style, see lib/appearance.ts.
                Live-review round 3, point 2: the "Palettenfarbe:"/"Palette
              colour" caption in front of the swatches read as noise once you
              can already see eight colour swatches sitting there, removed.
              settings.rainbowPalette itself is NOT orphaned: PaletteSwatch
              still reads it (see that component's own `label` line above)
              for each swatch's title/aria-label ("Palette colour 1", "...2",
              …), so the key stays in every locale unchanged.
                Live-review round 4 REVERSES the point above (jdp: "Bei der
              Rainbow-Farbpalette soll 'Farbpalette' stehen und dann die
              Farbfelder kommen"), a caption is back after all, just a
              different string than the one removed: settings.rainbowPalette
              ("Palette colour") stays a per-swatch aria-label only, unchanged;
              this new settings.rainbowPaletteLabel ("Colour palette") is a
              standalone row-opening label, matching how the Accent row right
              above it now opens with its own "Akzentfarbe" caption before
              its controls, same "label first" ordering, so the two rows in
              this merged Card read as one consistent pair rather than the
              swatch row being the odd one out again.
                Label now bare, no trailing colon (jdp, live-review: "Der
              Doppelpunkt nach Akzentfarbe und Farbpalette weg"), the colon
              was appended in JSX only, never in the translated string (all
              42 locales checked), so removing it here is the whole fix.
                Swatches+reset now right-aligned as their own `ms-auto` group
              (jdp, live-review: "Die Farbfelder der Farbpalette auch ganz
              nach rechts verschieben"), matching the Accent row's own
              identical right-alignment right above, the label stays at the
              row's start, everything clickable moves to the row's end. */}
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm text-carbon-text">{t("settings.rainbowPaletteLabel")}</span>
            <div className="flex items-center gap-2 flex-wrap ms-auto">
            {rainbow.palette.map((hex, i) => (
              <PaletteSwatch
                key={i}
                hex={hex}
                index={i}
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
                quadratischer Badge mit Glyph sein"), `shape="square"` still
                resolves through `rounded-control`, the shape engine's own
                live token, so this genuinely tracks round/soft/square (under
                "Rund" it renders as a full circle, same as every other
                square badge in the app).
                  NEUTRAL now, not hue-tinted (jdp, re-reporting: "Der
                Reset-Badge soll ... nicht farbig sein, damit er sich besser
                von den Farbflächen abhebt", measured live, this Badge's
                `tone="active"` fill was rendering the literal flat accent
                gold, `rgb(252,196,25)`, the EXACT SAME value as one of the
                eight palette swatches sitting right beside it, making the
                reset control indistinguishable from an actual colour choice
                at a glance). DELIBERATE EXCEPTION to this app's standing
                "every icon-only badge gets real colour-engine integration"
                rule, see AccentCard's mirror reset Badge (Settings.tsx,
                same file, a few hundred lines up) for the full "a reset
                control sitting beside the very colours it resets must not
                itself be one of those colours" reasoning, which applies
                here identically. `border-2 border-carbon-border` (NEW)
                matches PaletteSwatch's own border exactly (`h-7 w-7 ...
                border-2 border-carbon-border` a few dozen lines up), without
                it this Badge's solid fill filled the full 28×28 box edge to
                edge while each PaletteSwatch's own visible colour disc is
                actually only 24×24 (28px border-box minus its own 2px
                ring), which is what actually made this control read as
                BIGGER than its neighbours (jdp: "der Reset-Badge ist größer
                als die Farbfelder") despite an IDENTICAL measured 28×28
                bounding box, a real optical-weight defect a bounding-box
                check alone never catches, now closed by giving this Badge
                the exact same border every swatch beside it already has.
                  IconResetArrow (redesigned, see that icon's own header
                comment), the established counter-clockwise "reset" arrow
                convention, redrawn bolder for legibility at this small
                badge-in-a-busy-row size.
                  DISABLED ONLY WHEN THERE IS NOTHING TO RESET, the same
                dead-control audit that unified AccentCard's two resets above
                was run against this badge, and it had a milder version of the
                same defect from the other direction: gated on `!rainbow.on`
                alone, it sat ENABLED whenever rainbow mode was on even with
                the palette already byte-identical to RAINBOW, so a click was
                a silent no-op, a control that looks live and does nothing is
                the same broken promise as one that is permanently greyed out,
                just harder to notice. `paletteIsDefault` (computed beside
                `hueIdx` at this Card's own IIFE head) closes that: the badge
                is now live exactly when a click would actually change
                something. The `!rainbow.on` half of the gate STAYS, the
                eight PaletteSwatches next to it carry the identical
                `disabled={!rainbow.on}`, so the whole row switches off
                together, which reads as "this section is off" rather than as
                one arbitrarily dead control among live ones (the very
                confusion that made AccentCard's badge unreadable).
                  size="icon", the app's ONE square-icon-badge size (32px),
                never re-derived from the swatch box model. It was 28px, sized
                to this row's swatches; the app-wide unification moved the
                swatches instead (PaletteSwatch h-7 w-7 → h-8 w-8), so this
                badge and its eight neighbours still share one measured
                footprint and one 28px inner disc inside their `border-2`.
                  tip (not title/ariaLabel), IconTipButton's real hover/focus
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
                mirror above, the established "these two mirror each
                other" pairing.
                  tone="neutral" IS A DELIBERATE EXCEPTION to "every icon
                badge goes in the colour engine", see the AccentCard mirror
                above for the full reasoning, which applies here even more
                literally: this badge is the ninth 32px `border-2` tile in a
                row whose other eight ARE the rainbow palette's own colours
                (measured live: #FF8389 #FF832B #FCC419 #6FDC8C #3DDBD9
                #1D99F3 #BE95FF #FF7EB6). Giving it a rainbow fill would make
                the control that RESETS the palette look like a ninth entry
                IN the palette. Do not "fix" this to tone="active". */}
            {/* Same deliberate exception as the accent reset above (jdp,
                2026-08-29), and for a reason that is even more literal here:
                a row of eight palette swatches, with a ninth control that
                empties it. */}
            <Badge
              as="button"
              shape="square"
              size="icon"
              tone="neutral"
              tip={t("settings.rainbowPaletteReset")}
              onClick={() => updateRainbow({ palette: RAINBOW })}
              disabled={paletteIsDefault}
              className="border-2 border-carbon-border"
            >
              <IconResetArrow />
            </Badge>
            </div>
          </div>
          </>
          )}
        </div>
      </Card>
      );
      })()}

      {/* Quiet toasts (GlimStone form-engine Task 9): the toast system's
          severity-based quiet mode. Its own Card now (previously the last,
          divider-less sub-topic tacked onto the shared Appearance Card), next
          to the other purely client-side display preferences, rather than
          being bolted onto NotifyConfig's server-side "on" field above (that
          one gates external webhook/Matrix/email notifications, a different
          axis entirely; muting a toast in THIS browser must never silently
          change what a webhook receives elsewhere).
            No-empty-toggles audit (jdp): this row used to `hideLabel`
          because the Card's own title already says "Quiet toasts", the
          exact single-purpose-Card pattern the merged Colors Card's master
          "Regenbogen-Modus" toggle and RestoreChecksSection's "Automatische
          Restore-Prüfungen" toggle already had reversed, and this one was
          the leftover fourth instance the full-app grep in this pass caught.
          `hideLabel` is gone from ToggleRow entirely now (see its own header
          comment), the label is visible again.
            "Explanations belong in a bubble" pass (jdp): what explains this
          toggle was a permanent `description` caption printed under the row
          on every load, not an explanation gated behind the (i) affordance
          the rest of this page already uses, the exact anti-pattern
          Apprise's own ToggleRow comment above documents fixing the same way.
          Moved verbatim into `hint` instead (ToggleRow's own InfoBubble prop,
          same content contract as Card's `title`/`hint` pair): no wording
          change needed on either the EN source string or its DE translation:
          both were already a single compact two-sentence explanation, well
          within the register settings.offsiteDrillsHelp's own much longer
          hint text already establishes as normal for this bubble, so only
          the display mechanism moved, not the copy. Only this call site's
          own prop changed; the shared `settings.quietToastsHint` key and its
          text are untouched in i18n.ts and all 40 satellite locale files. */}
      <Card title={t("settings.quietToasts")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.quietToasts")}
          hint={t("settings.quietToastsHint")}
          checked={quiet}
          onChange={setQuiet}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL: About                                                      */}
      {/* Both versions, each linking to its own release, the ways to give,   */}
      {/* and the two routes for saying something. Replaces the old version   */}
      {/* footer rather than joining it ([363]), shipping both is the         */}
      {/* failure the design language names by name: one number in two type   */}
      {/* sizes twelve pixels apart.                                          */}
      {/*                                                                     */}
      {/* It stood on SYSTEM until [3559], which was a defensible reading of  */}
      {/* the language's "end of Settings" and the wrong one on a tabbed      */}
      {/* page. System is where the host integration and the export live,     */}
      {/* things somebody comes here to operate. General is the first tab in  */}
      {/* the strip, so this is the last card of the first thing anybody      */}
      {/* opens, which is where a version number and an invitation to give    */}
      {/* are actually found. The sibling apps already had it there, so this  */}
      {/* also ends a three-way disagreement about one standard card.         */}
      {/* Stays LAST in its tab either way: the card is a footer.             */}
      {/* ------------------------------------------------------------------ */}
      <AboutCard hueIndex={nextHue()} />
    </>
  );
}
