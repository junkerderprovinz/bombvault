import { useRef, useState } from "react";
import { AccentCard, IconResetArrow } from "../AccentCard";
import { ThemeCard } from "../ThemeCard";
import {
  CONTROL_AXES,
  LABEL_MODES,
  getLabelMode,
  setLabelMode,
  type ControlAxis,
  type LabelMode,
} from "../../../lib/controls";
import { labelModeChanged } from "../../../lib/useLabelMode";
import { InfoBubble } from "../../../components/InfoBubble";
import { Badge } from "../../../components/Badge";
import { useT, type TranslationKey } from "../../../lib/i18n";
import { ColorPickerSwatch } from "../../../components/ColorPickerPopover";
import { RAINBOW, getRainbow, setRainbow, type RainbowState } from "../../../lib/appearance";
import { SHAPES, getShape, leafTap, setShape, type Shape } from "../../../lib/shape";
import {
  MOTION_INTENSITIES,
  getMotionIntensity,
  setMotionIntensity,
  stormTap,
  type MotionIntensity,
} from "../../../lib/motion";
import { applyStoredDisco, discoTap, getDisco, setDisco } from "../../../lib/disco";
import { HUE_OFFSET, Selector } from "../../../components/Selector";
import { Card, ToggleRow, hueCounter } from "../shared";

// PaletteSwatch is one editable colour in the rainbow palette editor. It opens
// the shared colour popover instead of a native colour input, which would open
// a separate OS window, and uses `rounded-pill` rather than `rounded-full` so
// it follows the shape setting through var(--radius-pill).
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
  // h-8 w-8 matches the reset badge in the same row: every square icon badge
  // in the app is 32px, so the swatch follows the badge.
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

export function LookPage() {
  const { t } = useT();

  const [shape, setShapeLocal] = useState<Shape>(() => getShape());
  // The hidden leaf follows the storm below: found and counted in this
  // screen's state, never in storage, so it is offered only while chosen or
  // until this page is left.
  const [leafFound, setLeafFound] = useState(false);
  const leafClicks = useRef({ taps: 0 });

  const [motion, setMotionLocal] = useState<MotionIntensity>(() => getMotionIntensity());
  // The storm is the hidden fourth motion level. Both of these are component
  // state: `stormFound` must not survive leaving this page (an egg that
  // changes behaviour has to be switchable back off, never a permanent picker
  // entry), and the click counter has nothing to remember past the gesture.
  const [stormFound, setStormFound] = useState(false);
  const stormClicks = useRef({ taps: 0 });
  // Disco, the colour engine's own hidden mode, with the same two-part shape
  // the storm above uses: `discoFound` is component state so a found egg is
  // not a permanent row, and the counter has nothing to remember once the
  // gesture completes. Unlike the storm's, this counter carries a timestamp,
  // because its gesture is five turn-ons of Rainbow Mode and somebody merely
  // comparing the mode on and off would otherwise unlock it by accident.
  const [discoFound, setDiscoFound] = useState(false);
  const [disco, setDiscoLocal] = useState<boolean>(() => getDisco());
  const discoClicks = useRef({ taps: 0, last: 0 });
  // The label modes (#178), mirrored into local state so the selectors show
  // the current choice. The controls read through useLabelMode, which the
  // labelModeChanged() call below wakes.
  const [labelModes, setLabelModes] = useState<Record<ControlAxis, LabelMode>>(() => ({
    buttons: getLabelMode("buttons"),
    sidebar: getLabelMode("sidebar"),
    tabs: getLabelMode("tabs"),
    bottombar: getLabelMode("bottombar"),
  }));

  // setRainbow() persists, applies and returns the validated state, so local
  // state is updated from that return value rather than a second read.
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

  const nextHue = hueCounter();

  return (
    <>
      <ThemeCard t={t} hueIndex={nextHue()} />

      {/* Shape, the corner axis (lib/shape.ts). The segments carry no glyph:
          each segment is drawn at the real radius, so the strip itself is the
          preview, and a scaled-down stand-in beside it read as a weaker shape
          than the one it named. */}
      <Card title={t("settings.shape")} hint={t("settings.shapeHint")} hueIndex={nextHue()}>
        <Selector
          items={[...SHAPES, ...(leafFound || shape === "leaf" ? (["leaf"] as const) : [])].map((s) => ({
            id: s,
            label: t(`settings.shape.${s}` as TranslationKey),
          }))}
          label={t("settings.shape")}
          select="one"
          active={shape}
          onChange={(id) => {
            const leaf = leafTap(leafClicks.current, id, shape);
            if (leaf) setLeafFound(true);
            const next = (leaf ?? id) as Shape;
            setShapeLocal(next);
            setShape(next);
          }}
          size="lg"
          equalWidth
          hueOffset={HUE_OFFSET.shape}
        />
      </Card>

      {/* Motion intensity, built like the shape card: lib/motion.ts mirrors
          lib/shape.ts, and the level applies at the app root without a Save. */}
      <Card title={t("settings.motion")} hint={t("settings.motionHint")} hueIndex={nextHue()}>
        {/* The fourth level, storm, is hidden. It is offered while it is
            chosen, since a picker that hid its current value would misstate
            the interface, and otherwise only while this screen stays open,
            which is why `stormFound` is component state and not storage. */}
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
            // The gesture first, because it fires on the level already chosen
            // and therefore on a click that changes nothing else.
            const storm = stormTap(stormClicks.current, id, motion);
            if (storm) setStormFound(true);
            const next = (storm ?? id) as MotionIntensity;
            setMotionLocal(next);
            setMotionIntensity(next);
          }}
          size="lg"
          equalWidth
          hueOffset={HUE_OFFSET.motion}
        />
      </Card>

      {/* Control labels (#178), how much of a control's identity is shown.
          One selector per chrome surface rather than one switch, because the
          right answer differs: a rail reduced to glyphs narrows the whole
          page, tabs do not, and action buttons are a density preference.
          Placed straight after Animations: both are per-viewer appearance
          dials kept in this browser rather than server settings, and they
          read as a pair. */}
      <Card title={t("settings.labels")} hint={t("settings.labelsHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-4">
          {CONTROL_AXES.map((axis, axisIndex) => (
            <div key={axis} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(`settings.labels.${axis}` as TranslationKey)}
                {/* Layout mounts the rail or the bar, never both, so each of
                    those two rows is dead on the other width and says so.
                    Buttons and tabs need no hint: they answer everywhere. */}
                {axis === "bottombar" && <InfoBubble tip={t("settings.axisBottombarHint")} />}
                {axis === "sidebar" && <InfoBubble tip={t("settings.axisSidebarHint")} />}
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
                equalWidth
                // Each row starts one colour further along the palette, so two
                // selectors never repeat the same colour down the page.
                // Offsetting by the row index rather than by the row count
                // keeps the rows adjacent in the palette, so the block still
                // reads as one group rather than unrelated strips. Every other
                // start in the settings tree comes out of the same table.
                hueOffset={HUE_OFFSET.labels + axisIndex}
              />
            </div>
          ))}
        </div>
      </Card>

      {/* Accent colour and rainbow mode share one card. Turning the rainbow
          on sets data-rainbow and --rb-0..--rb-7 on <html>, which recolours
          every hue-enabled control; the sidebar is not one of them
          (Sidebar.tsx says why). The two reset badges stay neutral, so a
          reset does not blend into the colours it resets. */}
      {(() => {
      const hueIdx = nextHue();
      // Whether the palette row has anything left to reset, compared
      // case-insensitively like AccentCard's presetsAreDefault: setRainbow()
      // accepts either case, so "#ff8389" typed by hand still counts as the
      // default.
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
            hint={t("settings.rainbowHint")}
            checked={rainbow.on}
            onChange={rainbowToggled}
            hueIndex={0}
          />

          {/* Everything below depends on the rainbow being on, so while it is
              off none of it is shown: a palette editor under a rainbow that is
              not running offers edits with no effect. The accent row keeps its
              dimming instead, because its value still paints every control
              the rainbow does not reach. */}
          {rainbow.on && (
          <>
          {/* Disco, once found. Shown while it is on as well as while found,
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

          {/* The same row shape as the accent swatches, because it is the
              same job. setRainbow() and isValidPalette() validate the whole
              palette before it reaches the document (lib/appearance.ts). */}
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
            {/* Neutral rather than hue-tinted: beside eight palette swatches, a
                coloured reset would look like a ninth entry in the palette.
                The border matches the swatches' ring so it does not read
                bigger than they do, and it is disabled only when there is
                nothing to reset. The tip names its target, since the card
                holds two reset badges. */}
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
    </>
  );
}
