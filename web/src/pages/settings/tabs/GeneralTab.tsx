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
import { SHAPES, leafTap, setShape, type Shape } from "../../../lib/shape";
import { MOTION_INTENSITIES, setMotionIntensity, stormTap, type MotionIntensity } from "../../../lib/motion";
import { setDisco } from "../../../lib/disco";
import { HUE_OFFSET, Selector } from "../../../components/Selector";
import { Card, ToggleRow } from "../shared";
import { AboutCard } from "../AboutCard";
import type { SettingsTabProps } from "./types";

// PaletteSwatch is one editable colour of the rainbow palette. It uses the
// accent presets' floating picker because a native colour input would open a
// window of its own.
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
  // 32px, the size of a square icon badge, so the reset at the end of the row
  // matches the discs.
  return (
    <ColorPickerSwatch
      value={hex}
      onChange={onChange}
      label={label}
      disabled={disabled}
      className="h-8 w-8 shrink-0 rounded-pill transition-transform hover:scale-110 disabled:cursor-not-allowed disabled:opacity-50"
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
  leafFound,
  setLeafFound,
  leafClicks,
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
  toggleDbDumps,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      <Card title={t("settings.domains")} hint={t("settings.domainsHint")} hueIndex={nextHue()}>
        {/* Each toggle saves on its own and flips optimistically, so the
            domain's nav tab appears at once; a rejected save (VMs without a
            working SSH connection, say) reverts and shakes. `disabled` holds
            the row while its request is in flight. */}
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
        {/* Indented under Containers: it only acts on containers, and reads as
            a sub-option of that domain rather than a domain of its own. */}
        <div className="ps-6">
          <ToggleRow
            label={t("settings.dbDumps")}
            hint={t("settings.dbDumpsHint")}
            checked={settings.dbDumpsEnabled}
            onChange={(v) => void toggleDbDumps(v)}
            disabled={domainToggleBusy.dbDumpsEnabled}
            shakeNonce={domainToggleShake.dbDumpsEnabled}
            pulseNonce={fieldPulse.dbDumpsEnabled}
          />
        </div>
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
          label={t("settings.zfsEnabled")}
          hint={t("settings.zfsEnabledHint")}
          checked={settings.zfsEnabled}
          onChange={(v) => void toggleDomainEnabled("zfsEnabled", v)}
          disabled={domainToggleBusy.zfsEnabled}
          shakeNonce={domainToggleShake.zfsEnabled}
          pulseNonce={fieldPulse.zfsEnabled}
          hueIndex={4}
        />
        <ToggleRow
          label={t("settings.configEnabled")}
          hint={t("settings.configEnabledHint")}
          checked={settings.configEnabled}
          onChange={(v) => void toggleDomainEnabled("configEnabled", v)}
          disabled={domainToggleBusy.configEnabled}
          shakeNonce={domainToggleShake.configEnabled}
          pulseNonce={fieldPulse.configEnabled}
          hueIndex={5}
        />
        <ToggleRow
          label={t("settings.receiverEnabled")}
          hint={t("settings.receiverEnabledHint")}
          checked={settings.receiverEnabled}
          onChange={(v) => void toggleDomainEnabled("receiverEnabled", v)}
          disabled={domainToggleBusy.receiverEnabled}
          shakeNonce={domainToggleShake.receiverEnabled}
          pulseNonce={fieldPulse.receiverEnabled}
          hueIndex={6}
        />
        <ToggleRow
          label={t("settings.fleetEnabled")}
          hint={t("settings.fleetEnabledHint")}
          checked={settings.fleetEnabled}
          onChange={(v) => void toggleDomainEnabled("fleetEnabled", v)}
          disabled={domainToggleBusy.fleetEnabled}
          shakeNonce={domainToggleShake.fleetEnabled}
          pulseNonce={fieldPulse.fleetEnabled}
          hueIndex={7}
        />
        {/* Unlike receiver and fleet, pull writes: it fetches another
            instance's backups into this box's repository, and its hint says
            so. */}
        <ToggleRow
          label={t("settings.pullEnabled")}
          hint={t("settings.pullEnabledHint")}
          checked={settings.pullEnabled}
          onChange={(v) => void toggleDomainEnabled("pullEnabled", v)}
          disabled={domainToggleBusy.pullEnabled}
          shakeNonce={domainToggleShake.pullEnabled}
          pulseNonce={fieldPulse.pullEnabled}
          hueIndex={8}
        />
      </Card>

      <LanguageCard t={t} hueIndex={nextHue()} />

      <ThemeCard t={t} hueIndex={nextHue()} />

      {/* The options carry no preview glyph: the segment itself is drawn at
          the radius it names, and a scaled-down preview beside it made
          "round" look barely round. */}
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
          variant="well"
          equalWidth
          hueOffset={HUE_OFFSET.shape}
        />
      </Card>

      <Card title={t("settings.motion")} hint={t("settings.motionHint")} hueIndex={nextHue()}>
        {/* The hidden fourth level is offered while it is chosen, since a
            picker must not hide its own value, and otherwise only while this
            screen stays open. That is why `stormFound` is component state
            and never storage. */}
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
            // The gesture comes first because it fires on the level already
            // chosen, on a click that changes nothing else.
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

      {/* Three axes rather than one switch, because the right answer differs
          per axis: a sidebar reduced to glyphs narrows the whole page, tabs
          do not, and action buttons are a density preference. */}
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
                // Each row starts one colour further along, so the same
                // option is not the same colour in every row, while the
                // block still reads as one group.
                hueOffset={HUE_OFFSET.labels + axisIndex}
              />
            </div>
          ))}
        </div>
      </Card>

      {/* Accent and rainbow share one card because together they decide the
          colour of every control. */}
      {(() => {
      const hueIdx = nextHue();
      // Case-insensitive, like AccentCard's `presetsAreDefault`: setRainbow
      // accepts either case, so a palette typed back as "#ff8389" still
      // counts as the default.
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

          {/* Hidden rather than dimmed while the rainbow is off: with nothing
              behind them these controls would do nothing. The accent row
              stays dimmed instead, because its value still paints whatever
              the rainbow does not reach. */}
          {rainbow.on && (
          <>
          {/* Shown while disco is on as well as once found, so somebody who
              reloads with it running can still stop it. */}
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

          {/* The same row as the accent colour above: the label first, the
              swatches and the reset at the end of the row. setRainbow in
              lib/appearance.ts applies an edited palette whole or not at
              all. */}
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
            {/* Neutral, like the accent reset: a hued fill would make the
                control that empties the palette look like a ninth colour in
                it. The same 32px box as the discs. */}
            <Badge
              as="button"
              shape="square"
              size="icon"
              tone="neutral"
              tip={t("settings.rainbowPaletteReset")}
              onClick={() => updateRainbow({ palette: RAINBOW })}
              disabled={paletteIsDefault}
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

      {/* A browser-side preference, kept apart from the server's
          notification switch so muting toasts here never changes what a
          webhook receives. */}
      <Card title={t("settings.quietToasts")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.quietToasts")}
          hint={t("settings.quietToastsHint")}
          checked={quiet}
          onChange={setQuiet}
        />
      </Card>

      {/* Last card of the first tab, where a version number is looked for;
          the card is a footer. */}
      <AboutCard hueIndex={nextHue()} />
    </>
  );
}
