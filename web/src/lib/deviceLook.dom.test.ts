// @vitest-environment jsdom
// Where the device's colour and OLED come from: the phone inside the app, the
// browser everywhere else.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./displayPrefs", () => ({ save: () => {} }));

type Listener = (e: { data: string }) => void;

async function boot() {
  vi.resetModules();
  const look = await import("./deviceLook");
  const accent = await import("./accent");
  look.watchDeviceLook();
  return accent;
}

function inApp(device: unknown) {
  const listeners: Listener[] = [];
  Object.assign(window, {
    bombvaultApp: {
      postMessage: () => {},
      addEventListener: (_: string, fn: Listener) => listeners.push(fn),
      removeEventListener: () => {},
    },
    bombvaultDevice: device,
  });
  return (msg: unknown) => listeners.forEach((fn) => fn({ data: JSON.stringify(msg) }));
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.setAttribute("data-theme", "dark");
  window.matchMedia = ((query: string) => ({
    matches: true,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  })) as unknown as typeof window.matchMedia;
});

afterEach(() => {
  delete (window as { bombvaultApp?: unknown }).bombvaultApp;
  delete (window as { bombvaultDevice?: unknown }).bombvaultDevice;
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.removeAttribute("data-oled");
});

describe("inside the app", () => {
  it("takes Material You's colour before the first render and follows a new one", async () => {
    const say = inApp({ oled: false, accent: { light: "#3F5F91", dark: "#A8C8FF" } });
    const { systemAccent } = await boot();
    expect(systemAccent()).toBe("#A8C8FF");
    say({ op: "device", oled: false, accent: { light: "#6B5778", dark: "#D7BEE4" } });
    expect(systemAccent()).toBe("#D7BEE4");
  });

  it("has no colour to offer below Android 12", async () => {
    inApp({ oled: false });
    const { systemAccent } = await boot();
    expect(systemAccent()).toBeUndefined();
  });

  it("paints OLED when the phone switches it", async () => {
    const say = inApp({ oled: false });
    await boot();
    say({ op: "device", oled: true });
    expect(document.documentElement.hasAttribute("data-oled")).toBe(true);
  });
});

describe("in a browser", () => {
  afterEach(() => vi.restoreAllMocks());

  it("has no colour in Chromium, whose AccentColor is its own", async () => {
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue("Mozilla/5.0 AppleWebKit/537.36 Chrome/146.0.0.0 Safari/537.36");
    const { systemAccent } = await boot();
    expect(systemAccent()).toBeUndefined();
  });

  it("takes AccentColor for both themes elsewhere", async () => {
    const { browserAccent } = await import("./browserAccent");
    const { systemAccent } = await boot();
    expect(systemAccent()).toBe(browserAccent());
    document.documentElement.setAttribute("data-theme", "light");
    expect(systemAccent()).toBe(browserAccent());
  });
});
