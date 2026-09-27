// The pieces every settings card uses.

import { Badge } from "../../components/Badge";
import { ColorPickerSwatch } from "../../components/ColorPickerPopover";
import { InfoBubble } from "../../components/InfoBubble";
import { Toggle } from "../../components/Toggle";
import { hueVars } from "../../lib/appearance";
import { type CSSProperties } from "react";
import { useT } from "../../lib/i18n";

export type SaveState = "idle" | "saving" | "saved" | "error";

/** The new-password field on the System tab. The MCP card sends the operator
 *  there, because a key created while the web interface has no password is
 *  handed to whoever can reach the page. */
export const LOGIN_PASSWORD_FIELD = "bv-login-password";

export function Card({
  title,
  hint,
  children,
  hueIndex,
  nested,
}: {
  /** Without a title but with a hint the heading badge still renders, so the
   *  card keeps its hue notch. A card with neither renders no heading. */
  title?: string;
  /** One-line explanation of the whole card, shown as an (i) bubble beside
   *  the title (design-language.md rule 8). */
  hint?: string;
  children: React.ReactNode;
  /** Rainbow position of the heading notch among the cards on the active
   *  tab. Call sites take it from SettingsPage's nextHue() counter. */
  hueIndex?: number;
  /** Rendered inside another card that already provides the surface and the
   *  padding, as CloudCard and RcloneCard are in Recovery's step 3, so both
   *  are dropped and the content lines up with the parent's. pt-5 stays
   *  because the heading notch straddles the top edge and would otherwise
   *  sit on the first field. */
  nested?: boolean;
}) {
  return (
    // `relative` anchors the absolutely positioned heading badge; the hint
    // bubble rides inside the badge, on its solid fill, hence onAccent.
    // `glim-notch-card` is the hover zone the reactive rainbow mode keys on
    // in index.css. `.glim-hue` sets --accent and --focus-ring once for the
    // whole card, so no control inside has to repeat its card's hue.
    <div
      className={`relative glim-notch-card flex flex-col gap-4 ${
        nested ? "pt-5" : "bg-carbon-surface rounded-card p-5"
      }${hueIndex !== undefined ? " glim-hue" : ""}`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      {/* The h2 is for screen readers; what shows is the badge
          (design-language.md rule 11). */}
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

// AccentPresetSwatch is one preset in the accent card's row. A click selects
// it as the live accent and opens its editor, and edits in the open popover
// keep updating the accent. ColorPickerSwatch's button owns the click and
// opens the popover; the wrapping span sees the same click as it bubbles up
// and selects. The active ring sits on the wrapper because ColorPickerSwatch
// draws its own border.
export function AccentPresetSwatch({
  hex,
  index,
  active,
  onSelect,
  onChangePreset,
  disabled = false,
  t,
}: {
  hex: string;
  index: number;
  /** Whether this preset's stored colour matches the live accent. */
  active: boolean;
  /** Fires on click and on every live edit in the popover. */
  onSelect: (hex: string) => void;
  /** Fires on every live edit in the popover, to store the value in this
   *  preset's slot. */
  onChangePreset: (hex: string) => void;
  /** Inert while another setting, such as rainbow mode, owns the colours.
   *  The swatch stays on screen so the choice is still seen to exist. */
  disabled?: boolean;
  t: ReturnType<typeof useT>["t"];
}) {
  const label = `${t("settings.accentPreset")} ${index + 1}`;
  // A 28px disc inside the 2px ring makes a 32px box, the size of every square
  // icon badge, including the reset badge in this row.
  return (
    <span
      onClick={disabled ? undefined : () => onSelect(hex)}
      aria-disabled={disabled || undefined}
      // pointer-events-none also reaches the ColorPickerSwatch inside. The
      // picker opens from the disc itself, so blocking only this onClick
      // would still let it open and edit a preset nobody sees applied.
      className={`inline-flex rounded-pill border-2 transition-transform ${
        disabled ? "pointer-events-none" : "hover:scale-110"
      }`}
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
  /** Optional (i) bubble beside the label, same contract as Card's hint. */
  hint?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  /** Bump to replay the .glim-shake error animation once. It becomes part of
   *  the Toggle's key, so a new value remounts the switch and the animation
   *  restarts even when the same row fails twice in a row. */
  shakeNonce?: number;
  /** Bump to replay .glim-pulse, the success counterpart of shakeNonce. */
  pulseNonce?: number;
  /** Rainbow position of this row's switch, as its 0-based index among the
   *  ToggleRows rendered together in one group. Omit it only for a lone row
   *  with no siblings of its kind on screen. */
  hueIndex?: number;
}) {
  // No hooks: Settings.toggleRow.test.ts calls ToggleRow as a plain function,
  // outside a React renderer. hueVars is a plain function.
  //
  // The switch dims itself (Toggle.tsx), so only the label needs `dim`. A
  // plain <div> rather than <fieldset disabled>: around a single control a
  // fieldset only adds an unnamed group to the accessibility tree.
  const dim = disabled ? " opacity-50" : "";
  const hueOn = hueIndex !== undefined;
  // shakeNonce and pulseNonce count independently, so the key combines both;
  // either alone would stop changing once the other one fired. It stays
  // undefined until one has fired, so a plain render remounts nothing.
  const feedbackKey = shakeNonce || pulseNonce ? `${shakeNonce ?? 0}:${pulseNonce ?? 0}` : undefined;
  return (
    <div
      className={`flex items-start justify-between gap-4${hueOn ? " glim-hue" : ""}`}
      style={hueOn ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      <div className="flex flex-col gap-0.5">
        {/* The dimming goes on the label, not on the span that holds the
            bubble: a child cannot be less transparent than its parent, and
            on a disabled row the bubble is what explains why. */}
        <span className="flex items-center gap-1.5 text-sm">
          <span className={`text-carbon-text${dim}`}>{label}</span>
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
