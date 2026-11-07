// @vitest-environment jsdom
// The device's own colour as the accent. Choosing it stores the word system
// on this device alone; the synced accent stays underneath for every other
// device, and for this one whenever the source has nothing to say.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const save = vi.fn();
vi.mock("./displayPrefs", () => ({ save: () => save() }));

const TONES = { light: "#3F5F91", dark: "#A8C8FF" };

async function accent() {
  vi.resetModules();
  return import("./accent");
}

const painted = () => document.documentElement.style.getPropertyValue("--accent");

beforeEach(() => {
  localStorage.clear();
  save.mockClear();
  document.documentElement.setAttribute("data-theme", "dark");
});

afterEach(() => {
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.removeAttribute("style");
});

describe("the system accent", () => {
  it("paints the device's colour for the theme on screen once chosen", async () => {
    const { applyStoredAccent, chooseSystemAccent, setSystemAccent } = await accent();
    applyStoredAccent();
    setSystemAccent(TONES);
    chooseSystemAccent();
    expect(painted()).toBe(TONES.dark);

    document.documentElement.setAttribute("data-theme", "light");
    applyStoredAccent();
    expect(painted()).toBe(TONES.light);
  });

  it("stores the word system on this device and leaves the synced accent alone", async () => {
    localStorage.setItem("bv-accent", "#FF8389");
    const { chooseSystemAccent, getAccent, setSystemAccent } = await accent();
    setSystemAccent(TONES);
    chooseSystemAccent();
    expect(localStorage.getItem("bv-accent-device")).toBe("system");
    expect(getAccent()).toBe("#FF8389");
    expect(Object.values(localStorage).some((v) => v === TONES.dark || v === TONES.light)).toBe(false);
    expect(save).not.toHaveBeenCalled();
  });

  it("falls back to the synced accent where the device has no colour", async () => {
    localStorage.setItem("bv-accent", "#FF8389");
    localStorage.setItem("bv-accent-device", "system");
    const { applyStoredAccent, setSystemAccent, systemAccentChosen } = await accent();
    applyStoredAccent();
    expect(painted()).toBe("#FF8389");

    setSystemAccent(TONES);
    expect(painted()).toBe(TONES.dark);
    setSystemAccent(undefined);
    expect(painted()).toBe("#FF8389");
    expect(systemAccentChosen()).toBe(true);
  });

  it("gives way to a preset, which goes to the server as before", async () => {
    const { chooseSystemAccent, setAccent, setSystemAccent, systemAccentChosen } = await accent();
    setSystemAccent(TONES);
    chooseSystemAccent();
    setAccent("#6FDC8C");
    expect(systemAccentChosen()).toBe(false);
    expect(painted()).toBe("#6FDC8C");
    expect(save).toHaveBeenCalledTimes(1);
  });

  it("tells subscribers when the source comes or goes, and only then", async () => {
    const { setSystemAccent, subscribeSystemAccent, systemAccent } = await accent();
    const seen: (string | undefined)[] = [];
    subscribeSystemAccent(() => seen.push(systemAccent()));
    setSystemAccent(TONES);
    setSystemAccent({ ...TONES });
    setSystemAccent({ light: "nope", dark: "#A8C8FF" });
    expect(seen).toEqual([TONES.dark, undefined]);
  });

  it("paints the default for a stored accent that is no colour, such as the word system", async () => {
    localStorage.setItem("bv-accent", "system");
    const { applyStoredAccent, DEFAULT_ACCENT } = await accent();
    applyStoredAccent();
    expect(painted()).toBe(DEFAULT_ACCENT);
  });
});
