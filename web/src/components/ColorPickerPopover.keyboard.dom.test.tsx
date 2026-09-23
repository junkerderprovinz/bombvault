// @vitest-environment jsdom
// The SV square and the hue bar are sliders that work from the keyboard.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ColorPickerSwatch } from "./ColorPickerPopover";

function renderPicker(value = "#3366cc") {
  const onChange = vi.fn();
  render(
    <I18nProvider>
      <ColorPickerSwatch value={value} onChange={onChange} label="Accent" />
    </I18nProvider>
  );
  return onChange;
}

async function openPanel() {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Accent" }));
  });
}

const hue = () => screen.getByRole("slider", { name: en["picker.hue"] });
const sv = () => screen.getByRole("slider", { name: en["picker.saturationBrightness"] });

beforeEach(() => {
  Element.prototype.scrollIntoView = function () {};
});

afterEach(cleanup);

describe("colour picker keyboard operation", () => {
  it("exposes both axes as sliders with real values", async () => {
    renderPicker();
    await openPanel();
    expect(hue().getAttribute("tabindex")).toBe("0");
    expect(sv().getAttribute("tabindex")).toBe("0");
    expect(hue().getAttribute("aria-valuenow")).toBeTruthy();
    // The 2-D control announces both axes, since aria-valuenow can only carry one.
    expect(sv().getAttribute("aria-valuetext")).toMatch(/%.*%/);
  });

  it("steps the hue with the arrow keys and reports the new colour", async () => {
    const onChange = renderPicker();
    await openPanel();
    const before = Number(hue().getAttribute("aria-valuenow"));

    await act(async () => {
      fireEvent.keyDown(hue(), { key: "ArrowRight" });
    });
    expect(Number(hue().getAttribute("aria-valuenow"))).toBe(before + 1);
    expect(onChange).toHaveBeenCalled();
    expect(onChange.mock.calls.at(-1)?.[0]).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("takes a coarse step with Shift and wraps around the circle", async () => {
    renderPicker();
    await openPanel();

    await act(async () => {
      fireEvent.keyDown(hue(), { key: "Home" });
    });
    expect(Number(hue().getAttribute("aria-valuenow"))).toBe(0);

    // Hue is a circle: stepping below 0 wraps to the top rather than clamping.
    await act(async () => {
      fireEvent.keyDown(hue(), { key: "ArrowLeft" });
    });
    expect(Number(hue().getAttribute("aria-valuenow"))).toBe(359);

    await act(async () => {
      fireEvent.keyDown(hue(), { key: "ArrowRight", shiftKey: true });
    });
    expect(Number(hue().getAttribute("aria-valuenow"))).toBe(14);
  });

  it("moves saturation and brightness on their own axes, and clamps", async () => {
    renderPicker();
    await openPanel();

    await act(async () => {
      fireEvent.keyDown(sv(), { key: "Home" });
    });
    // Home is the white corner: no saturation, full brightness.
    expect(sv().getAttribute("aria-valuetext")).toBe("0% / 100%");

    await act(async () => {
      fireEvent.keyDown(sv(), { key: "ArrowLeft" });
    });
    expect(sv().getAttribute("aria-valuetext")).toBe("0% / 100%"); // clamped, not negative

    await act(async () => {
      fireEvent.keyDown(sv(), { key: "ArrowRight" });
      fireEvent.keyDown(sv(), { key: "ArrowDown" });
    });
    expect(sv().getAttribute("aria-valuetext")).toBe("1% / 99%");
  });
});
