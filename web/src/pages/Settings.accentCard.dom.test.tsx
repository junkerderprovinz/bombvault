// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// AccentCard (GlimStone follow-up pass, live-review round 6 — jdp: "Die
// Voreinstellungsfelder der Akzentfarbe sollen auch bearbeitbar sein und
// auch ein Reset-Badge bekommen. Bitte mehr Voreinstellungsfarbfelder"). This
// covers the interaction this round actually decided on: a click both
// SELECTS a preset as the live accent AND opens its editor, further edits
// inside that popover keep updating the live accent too, resetting is
// ROW-LEVEL and restores the shipped defaults, and the preset count is now 8
// — see AccentCard's/AccentPresetSwatch's own header comments in Settings.tsx
// and lib/accent.ts's own header comment for the full reasoning.
//
// Extracted the same way Settings.languageCard.dom.test.tsx already covers
// LanguageCard in isolation — jsdom opted in explicitly per Selector.dom.
// test.tsx's own naming convention for the exception.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { AccentCard } from "./Settings";
import { I18nProvider, useT } from "../lib/i18n";
import { DEFAULT_ACCENT, DEFAULT_ACCENT_PRESETS } from "../lib/accent";

const ACCENT_KEY = "bv-accent";
const PRESETS_KEY = "bv-accent-presets";

function Harness() {
  const { t } = useT();
  // No hueIndex prop any more (GlimStone follow-up round, jdp's
  // neutral-reset-badge fix): AccentCard's preset-reset Badge is now
  // deliberately `tone="neutral"`, never hue-tinted — see that Badge's own
  // call-site comment in Settings.tsx — so AccentCard has no remaining
  // reader for a hue value and the prop was removed from its signature.
  return <AccentCard t={t} />;
}

function renderCard() {
  return render(
    <I18nProvider>
      <Harness />
    </I18nProvider>
  );
}

beforeEach(() => {
  localStorage.removeItem(ACCENT_KEY);
  localStorage.removeItem(PRESETS_KEY);
});

afterEach(() => {
  cleanup();
  localStorage.removeItem(ACCENT_KEY);
  localStorage.removeItem(PRESETS_KEY);
  document.documentElement.style.removeProperty("--accent");
});

describe("AccentCard — preset count", () => {
  it("renders all 8 default presets, each with its own numbered accessible name", () => {
    renderCard();
    for (let i = 1; i <= 8; i++) {
      expect(screen.getByRole("button", { name: `Preset ${i}` })).toBeTruthy();
    }
    expect(screen.queryByRole("button", { name: "Preset 9" })).toBeNull();
  });

  it("each preset swatch shows its own default colour", () => {
    renderCard();
    // hex -> rgb() is jsdom's own normalization of the inline style, same
    // assertion shape ColorPickerPopover.dom.test.tsx already uses.
    const EXPECTED_RGB = [
      "rgb(252, 196, 25)", // #FCC419 Sunflower
      "rgb(29, 153, 243)", // #1D99F3 Blue
      "rgb(111, 220, 140)", // #6FDC8C Green
      "rgb(255, 131, 137)", // #FF8389 Red
      "rgb(190, 149, 255)", // #BE95FF Purple
      "rgb(255, 131, 43)", // #FF832B Orange
      "rgb(61, 219, 217)", // #3DDBD9 Teal
      "rgb(255, 126, 182)", // #FF7EB6 Magenta
    ];
    for (let i = 0; i < DEFAULT_ACCENT_PRESETS.length; i++) {
      const swatch = screen.getByRole("button", { name: `Preset ${i + 1}` }) as HTMLButtonElement;
      expect(swatch.style.backgroundColor).toBe(EXPECTED_RGB[i]);
    }
  });
});

describe("AccentCard — clicking a preset selects AND opens its editor", () => {
  it("selects the preset as the live accent immediately on click", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" })); // Blue
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[1]);
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe(DEFAULT_ACCENT_PRESETS[1]);
  });

  it("the SAME click also opens that preset's own editor popover", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" }));
    const dialog = screen.getByRole("dialog", { name: "Preset 2" });
    expect(dialog).toBeTruthy();
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    expect(hexField.value.toLowerCase()).toBe(DEFAULT_ACCENT_PRESETS[1].toLowerCase());
  });

  it("clicking a different preset selects/opens IT without leaving two popovers open", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 1" }));
    fireEvent.click(screen.getByRole("button", { name: "Preset 3" })); // Green
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[2]);
    expect(screen.queryByRole("dialog", { name: "Preset 1" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "Preset 3" })).toBeTruthy();
  });
});

describe("AccentCard — editing a preset", () => {
  it("typing a new hex in the open popover persists it into THAT preset's own slot", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" })); // Red
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#ABCDEF" } });

    const stored = JSON.parse(localStorage.getItem(PRESETS_KEY)!);
    expect(stored[3]).toBe("#abcdef");
    // The other 7 presets are untouched.
    for (let i = 0; i < DEFAULT_ACCENT_PRESETS.length; i++) {
      if (i === 3) continue;
      expect(stored[i]).toBe(DEFAULT_ACCENT_PRESETS[i]);
    }
  });

  it("editing a preset also live-updates the active accent (selecting and editing are the same gesture)", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#ABCDEF" } });

    expect(localStorage.getItem(ACCENT_KEY)).toBe("#abcdef");
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe("#abcdef");
  });

  it("editing preset A while preset B was previously active does not change B's own stored colour", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 1" }));
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#ABCDEF" } });

    const stored = JSON.parse(localStorage.getItem(PRESETS_KEY)!);
    expect(stored[0]).toBe(DEFAULT_ACCENT_PRESETS[0]);
  });
});

// ---------------------------------------------------------------------------
// ONE reset for the whole row (jdp, live-review, third re-report: "Der
// Resetbutton ist da, hat aber keine Funktion und der Zurücksetzen-Text ist
// immer noch da"). The row used to ship two competing controls — a preset-only
// icon badge gated `disabled={presetsAreDefault}`, and an accent-only "Reset"
// TEXT button gated `accentHex !== DEFAULT_ACCENT`. Because presets sit at
// their shipped defaults for anyone who never opened a preset's editor
// popover, the badge was permanently greyed out in every ordinary session
// while the text link appeared right beside it. See AccentCard's own header
// comment in Settings.tsx for the full root cause.
//
// What these tests deliberately exercise, rather than merely assert the
// existence of: the control's ENABLED/DISABLED transitions in both drift
// directions, and that clicking it actually changes stored state. Every prior
// round's check ("the badge exists / has the right classes") passed against a
// badge that could not be clicked, which is exactly how this shipped broken
// three times.
// ---------------------------------------------------------------------------
const RESET_NAME = "Reset accent color and presets";

describe("AccentCard — one row-level reset for BOTH the accent and the presets", () => {
  it("is present but disabled while the accent AND every preset are at their shipped defaults", () => {
    renderCard();
    // Plain DOM property access, not toBeDisabled() — no @testing-library/
    // jest-dom in this repo, see ColorPickerPopover.dom.test.tsx's own header
    // comment; `.disabled` mirrors the native `disabled` attribute React's
    // `disabled={...}` prop sets on a real <button>.
    const resetButton = screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement;
    expect(resetButton).toBeTruthy();
    expect(resetButton.disabled).toBe(true);
  });

  // THE regression this round exists for: picking any non-default accent is
  // the single most common thing a user does in this row, and it used to
  // leave the badge dead.
  it("becomes ENABLED as soon as only the ACTIVE ACCENT has drifted (presets untouched)", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" })); // Blue
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[1]);
    // Presets are still byte-identical to their defaults here — under the old
    // `disabled={presetsAreDefault}` gate this assertion would read `true`.
    expect((screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("becomes ENABLED as soon as only a PRESET has drifted", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 1" })); // already the default accent
    fireEvent.change(screen.getByLabelText("Hex"), { target: { value: "#ABCDEF" } });
    expect((screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("one click restores BOTH the active accent and the presets, then disables itself again", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" })); // Red, and opens its editor
    fireEvent.change(screen.getByLabelText("Hex"), { target: { value: "#ABCDEF" } });
    expect(localStorage.getItem(ACCENT_KEY)).toBe("#abcdef");

    const resetButton = screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement;
    expect(resetButton.disabled).toBe(false);
    fireEvent.click(resetButton);

    expect(JSON.parse(localStorage.getItem(PRESETS_KEY)!)).toEqual(DEFAULT_ACCENT_PRESETS);
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT);
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe(DEFAULT_ACCENT);
    // The control stays present, just becomes disabled again once nothing is
    // left to reset — it never disappears.
    expect((screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("is a SINGLE control — not one per preset, and not one per concern", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    fireEvent.change(screen.getByLabelText("Hex"), { target: { value: "#ABCDEF" } });
    expect(screen.getAllByRole("button", { name: RESET_NAME })).toHaveLength(1);
  });
});

describe("AccentCard — the leftover 'Reset' TEXT button is gone", () => {
  // Asserted in the exact state that used to render it: a non-default active
  // accent. A check run only in the default state would have passed against
  // the broken build too, since the old text button was itself conditional.
  it("renders no bare 'Reset' text control while the accent is non-default", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" }));
    expect(localStorage.getItem(ACCENT_KEY)).not.toBe(DEFAULT_ACCENT);

    expect(screen.queryByRole("button", { name: "Reset" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Reset presets" })).toBeNull();
    // Nothing anywhere in the card renders the word on its own as visible
    // text — the surviving control is icon-only, its wording lives in the
    // tooltip bubble/accessible name instead.
    const texts = Array.from(document.querySelectorAll("button")).map((b) => b.textContent?.trim());
    expect(texts).not.toContain("Reset");
  });
});
