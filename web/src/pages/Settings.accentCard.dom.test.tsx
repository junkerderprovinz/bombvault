// @vitest-environment jsdom
// A click on a preset selects it as the live accent and opens its editor,
// edits in that popover keep updating the accent, and one row-level reset
// restores both the accent and the shipped presets.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { AccentCard } from "./settings/AccentCard";
import { I18nProvider, useT } from "../lib/i18n";
import { DEFAULT_ACCENT, DEFAULT_ACCENT_PRESETS } from "../lib/accent";

const ACCENT_KEY = "bv-accent";
const PRESETS_KEY = "bv-accent-presets";

function Harness() {
  const { t } = useT();
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

describe("AccentCard preset count", () => {
  it("renders all 8 default presets, each with its own numbered accessible name", () => {
    renderCard();
    for (let i = 1; i <= 8; i++) {
      expect(screen.getByRole("button", { name: `Preset ${i}` })).toBeTruthy();
    }
    expect(screen.queryByRole("button", { name: "Preset 9" })).toBeNull();
  });

  it("each preset swatch shows its own default colour", () => {
    renderCard();
    // jsdom normalizes the inline hex to rgb().
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

describe("AccentCard clicking a preset", () => {
  it("selects the preset as the live accent immediately on click", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" })); // Blue
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[1]);
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe(DEFAULT_ACCENT_PRESETS[1]);
  });

  it("the same click also opens that preset's own editor popover", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" }));
    const dialog = screen.getByRole("dialog", { name: "Preset 2" });
    expect(dialog).toBeTruthy();
    const hexField = screen.getByLabelText("Hex") as HTMLInputElement;
    expect(hexField.value.toLowerCase()).toBe(DEFAULT_ACCENT_PRESETS[1].toLowerCase());
  });

  it("clicking a different preset selects and opens it without leaving two popovers open", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 1" }));
    fireEvent.click(screen.getByRole("button", { name: "Preset 3" })); // Green
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[2]);
    expect(screen.queryByRole("dialog", { name: "Preset 1" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "Preset 3" })).toBeTruthy();
  });
});

describe("AccentCard editing a preset", () => {
  it("typing a new hex in the open popover persists it into that preset's slot", () => {
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

  it("editing a preset also live-updates the active accent", () => {
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

// These click the reset in each drift direction rather than only checking
// that it renders.
const RESET_NAME = "Reset accent color and presets";

describe("AccentCard row-level reset for the accent and the presets", () => {
  it("is present but disabled while the accent and every preset are at their shipped defaults", () => {
    renderCard();
    // There is no jest-dom here, so the native disabled property stands in
    // for toBeDisabled().
    const resetButton = screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement;
    expect(resetButton).toBeTruthy();
    expect(resetButton.disabled).toBe(true);
  });

  it("becomes enabled as soon as only the active accent has drifted", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" })); // Blue
    expect(localStorage.getItem(ACCENT_KEY)).toBe(DEFAULT_ACCENT_PRESETS[1]);
    // The presets still equal their defaults here.
    expect((screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("becomes enabled as soon as only a preset has drifted", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 1" })); // already the default accent
    fireEvent.change(screen.getByLabelText("Hex"), { target: { value: "#ABCDEF" } });
    expect((screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("one click restores both the active accent and the presets, then disables itself again", () => {
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
    expect((screen.getByRole("button", { name: RESET_NAME }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("is a single control, not one per preset or per concern", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 4" }));
    fireEvent.change(screen.getByLabelText("Hex"), { target: { value: "#ABCDEF" } });
    expect(screen.getAllByRole("button", { name: RESET_NAME })).toHaveLength(1);
  });
});

describe("AccentCard reset is icon-only", () => {
  // Checked with a non-default accent, the state in which a text reset would
  // show up.
  it("renders no bare 'Reset' text control while the accent is non-default", () => {
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Preset 2" }));
    expect(localStorage.getItem(ACCENT_KEY)).not.toBe(DEFAULT_ACCENT);

    expect(screen.queryByRole("button", { name: "Reset" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Reset presets" })).toBeNull();
    // The wording lives in the tooltip and the accessible name.
    const texts = Array.from(document.querySelectorAll("button")).map((b) => b.textContent?.trim());
    expect(texts).not.toContain("Reset");
  });
});
