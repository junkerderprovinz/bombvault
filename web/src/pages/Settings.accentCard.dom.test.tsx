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

describe("AccentCard — reset presets (row-level, not per-preset)", () => {
  // ALWAYS rendered, disabled (not hidden) while nothing has drifted —
  // GlimStone follow-up round, jdp re-reporting after a prior round's fix
  // didn't hold up live: "Bei der Akzentfarbe ist das Zurücksetzen immer
  // noch kein Badge mit Glyph." Fresh live inspection found the DEFAULT
  // state rendered no badge AT ALL (the previous `{!presetsAreDefault && ()}`
  // conditional unmounted it entirely), which is exactly what a reviewer
  // who never happened to edit a preset's own colour would see. Switched to
  // `disabled={presetsAreDefault}` on an unconditionally-rendered Badge —
  // the same pattern the rainbow-palette reset badge already used — so the
  // control is a real, present, measurable badge at all times; only its
  // enabled/disabled state changes. See AccentCard's own header comment in
  // Settings.tsx for the full root-cause writeup.
  it("the reset control is present but disabled while every preset still matches its shipped default", () => {
    renderCard();
    // Plain DOM property access, not toBeDisabled() — no @testing-library/
    // jest-dom in this repo, see ColorPickerPopover.dom.test.tsx's own header
    // comment; `.disabled` mirrors the native `disabled` attribute React's
    // `disabled={...}` prop sets on a real <button>.
    const resetButton = screen.getByRole("button", { name: "Reset presets" }) as HTMLButtonElement;
    expect(resetButton).toBeTruthy();
    expect(resetButton.disabled).toBe(true);
  });

  it("becomes enabled once a preset has drifted, and restores the ORIGINAL shipped defaults on click", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    fireEvent.change(hexField, { target: { value: "#ABCDEF" } });

    const resetButton = screen.getByRole("button", { name: "Reset presets" }) as HTMLButtonElement;
    expect(resetButton.disabled).toBe(false);
    fireEvent.click(resetButton);

    expect(JSON.parse(localStorage.getItem(PRESETS_KEY)!)).toEqual(DEFAULT_ACCENT_PRESETS);
    // Row-level: resetting the PRESETS never touches the currently active
    // accent — that is the separate "Reset" text button's own job.
    expect(localStorage.getItem(ACCENT_KEY)).toBe("#abcdef");
    // The control stays present, just becomes disabled again once nothing
    // is left to reset — it never disappears.
    expect((screen.getByRole("button", { name: "Reset presets" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("is a SINGLE row-level control, not one reset per preset", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    fireEvent.change(screen.getByLabelText("Hex"), { target: { value: "#ABCDEF" } });
    expect(screen.getAllByRole("button", { name: "Reset presets" })).toHaveLength(1);
  });
});

describe("AccentCard — the pre-existing active-accent reset is unaffected", () => {
  it("still resets accentHex to DEFAULT_ACCENT, independent of the presets reset", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[3]);

    fireEvent.click(screen.getByRole("button", { name: "Reset" }));
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT);
    // The preset itself is untouched by the ACCENT reset.
    expect(JSON.parse(localStorage.getItem(PRESETS_KEY) ?? "null") ?? DEFAULT_ACCENT_PRESETS).toEqual(
      DEFAULT_ACCENT_PRESETS
    );
  });
});
