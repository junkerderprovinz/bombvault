import { AccentPresetSwatch } from "../settings/shared";
import { Badge } from "../../components/Badge";
import { InfoBubble } from "../../components/InfoBubble";
import { getAccent, setAccent, DEFAULT_ACCENT, getAccentPresets, setAccentPresets, DEFAULT_ACCENT_PRESETS } from "../../lib/accent";
import { useState } from "react";
import { useT } from "../../lib/i18n";

export function AccentCard({
  t,
  rainbowOn = false,
}: {
  t: ReturnType<typeof useT>["t"];
  /** While rainbow mode owns the colours the row is dimmed and inert rather
   *  than hidden, so it still shows that an accent exists. The rainbow only
   *  overrides --accent on elements with `.glim-hue`, so the hint says how to
   *  choose an accent again rather than claiming it has no effect. */
  rainbowOn?: boolean;
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

  // One reset restores both the accent and the presets, so it is enabled when
  // either has drifted.
  const accentIsDefault = accentHex.toLowerCase() === DEFAULT_ACCENT.toLowerCase();
  const presetsAreDefault = presets.every(
    (hex, i) => hex.toLowerCase() === DEFAULT_ACCENT_PRESETS[i]?.toLowerCase()
  );
  const nothingToReset = accentIsDefault && presetsAreDefault;

  return (
    // The dimming sits on the label and the swatch group rather than on the
    // row: opacity applies to a whole subtree, and the info bubble in the row
    // has to stay readable.
    <div className="flex items-center gap-3 flex-wrap">
      <span className={`text-sm text-carbon-text ${rainbowOn ? "opacity-45" : ""}`}>
        {t("settings.accentColor")}
      </span>
      {/* Only while the rainbow owns the colours: an (i) that is always there
          would suggest the control itself needs explaining. */}
      {rainbowOn && <InfoBubble tip={t("settings.accentRainbowHint")} />}
      <div className={`flex items-center gap-2 flex-wrap ms-auto ${rainbowOn ? "opacity-45" : ""}`}>
        {/* Every preset opens the colour picker, so a custom colour is set by
            editing a preset. */}
        {presets.map((hex, i) => (
          <AccentPresetSwatch
            key={i}
            hex={hex}
            index={i}
            active={accentHex.toLowerCase() === hex.toLowerCase()}
            onSelect={selectAccent}
            onChangePreset={(v) => changePreset(i, v)}
            disabled={rainbowOn}
            t={t}
          />
        ))}
        {/* A neutral square badge rather than a colour-engine Button: it has
            the same box and border as the swatches beside it, and an accent
            fill would make it read as one more colour to pick when it throws
            the picked colour away. The border also keeps its fill the size of
            a swatch's disc. The rainbow-palette reset mirrors it. */}
        <Badge
          as="button"
          shape="square"
          size="icon"
          tone="neutral"
          tip={t("settings.accentReset")}
          onClick={() => {
            selectAccent(DEFAULT_ACCENT);
            setPresets(setAccentPresets(DEFAULT_ACCENT_PRESETS));
          }}
          disabled={nothingToReset || rainbowOn}
          className="border-2 border-carbon-border"
        >
          <IconResetArrow />
        </Badge>
      </div>
    </div>
  );
}

// IconResetArrow is the glyph of the accent and rainbow-palette reset badges.
// It is bolder than IconRecovery and IconRestore because at 16px, beside a row
// of saturated swatches, a thinner ring reads as a "C".
export function IconResetArrow() {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
      <path d="M14 8A6 6 0 1 1 8 2L8 4.7A3.3 3.3 0 1 0 11.3 8Z" />
      <path d="M8 1 3.5 3 8 5Z" />
    </svg>
  );
}
