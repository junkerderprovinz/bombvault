// @vitest-environment jsdom
// The wand swatch: the device's own colour, first in the accent row wherever
// the device has one, and a choice that stays on this device.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

const save = vi.fn();
vi.mock("../lib/displayPrefs", async (real) => ({ ...(await real<object>()), save: () => save() }));

import { AccentCard } from "./settings/AccentCard";
import { I18nProvider, useT } from "../lib/i18n";
import { setSystemAccent } from "../lib/accent";

const TONES = { light: "#3F5F91", dark: "#A8C8FF" };

function Harness() {
  const { t } = useT();
  return <AccentCard t={t} />;
}

function renderCard() {
  return render(
    <I18nProvider>
      <Harness />
    </I18nProvider>,
  );
}

const wand = () => screen.queryByRole("button", { name: "System color" });
const ring = (el: HTMLElement) => (el.closest("span.rounded-pill") as HTMLElement).style.borderColor;

beforeEach(() => {
  localStorage.clear();
  save.mockClear();
  document.documentElement.setAttribute("data-theme", "dark");
});

afterEach(() => {
  cleanup();
  act(() => setSystemAccent(undefined));
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.removeAttribute("style");
});

describe("the wand swatch", () => {
  it("is not there while the device has no colour", () => {
    renderCard();
    expect(wand()).toBeNull();
  });

  it("comes and goes with the device's colour, first in the row", () => {
    renderCard();
    act(() => setSystemAccent(TONES));
    const swatches = screen.getAllByRole("button").filter((b) => /System color|Preset/.test(b.getAttribute("aria-label") ?? ""));
    expect(swatches[0]).toBe(wand());
    expect(wand()!.style.background).toBe("rgb(168, 200, 255)");
    act(() => setSystemAccent(undefined));
    expect(wand()).toBeNull();
  });

  it("is chosen on this device alone and takes the ring from the preset", () => {
    localStorage.setItem("bv-accent", "#1D99F3");
    renderCard();
    act(() => setSystemAccent(TONES));
    fireEvent.click(wand()!);
    expect(wand()!.getAttribute("aria-pressed")).toBe("true");
    expect(ring(screen.getByRole("button", { name: "Preset 2" }))).toBe("var(--carbon-border)");
    expect(localStorage.getItem("bv-accent")).toBe("#1D99F3");
    expect(localStorage.getItem("bv-accent-device")).toBe("system");
    expect(save).not.toHaveBeenCalled();
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe(TONES.dark);
  });

  it("lets go when a preset is picked", () => {
    renderCard();
    act(() => setSystemAccent(TONES));
    fireEvent.click(wand()!);
    fireEvent.click(screen.getByRole("button", { name: "Preset 3" }).closest("span.rounded-pill")!);
    expect(wand()!.getAttribute("aria-pressed")).toBe("false");
    expect(localStorage.getItem("bv-accent-device")).toBeNull();
  });
});
