// @vitest-environment jsdom
// The theme is one of three stored values. "system" paints what the device
// prefers and repaints when the device changes its mind; dark and light hold
// against the device until System is chosen again.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./displayPrefs", () => ({ save: vi.fn() }));

const KEY = "bv-theme";

let prefersDark = false;
let listeners: Array<() => void> = [];

function deviceTurns(dark: boolean) {
  prefersDark = dark;
  listeners.forEach((cb) => cb());
}

async function theme() {
  vi.resetModules();
  return import("./theme");
}

const oledPainted = () => document.documentElement.hasAttribute("data-oled");

/** The Android app's message port and what it says about the phone, as the app sets them before a page runs. */
function inApp(oled: boolean): string[] {
  const posted: string[] = [];
  Object.assign(window, {
    bombvaultApp: { postMessage: (m: string) => posted.push(m), addEventListener: () => {}, removeEventListener: () => {} },
    bombvaultDevice: { oled },
  });
  return posted;
}

const painted = () => document.documentElement.getAttribute("data-theme");
const chrome = () => document.querySelector('meta[name="theme-color"]')?.getAttribute("content");

beforeEach(() => {
  localStorage.clear();
  prefersDark = false;
  listeners = [];
  window.matchMedia = ((query: string) => ({
    get matches() {
      return prefersDark;
    },
    media: query,
    addEventListener: (_event: string, cb: () => void) => {
      listeners.push(cb);
    },
    removeEventListener: () => {},
  })) as unknown as typeof window.matchMedia;
  document.head.innerHTML = '<meta name="theme-color" content="#161616" />';
});

afterEach(() => {
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.removeAttribute("data-oled");
  document.head.innerHTML = "";
  delete (window as { bombvaultApp?: unknown }).bombvaultApp;
  delete (window as { bombvaultDevice?: unknown }).bombvaultDevice;
});

describe("the stored theme", () => {
  it("is System in a browser that never stored one", async () => {
    const { getTheme } = await theme();
    expect(getTheme()).toBe("system");
  });

  it.each(["dark", "light", "system"] as const)("keeps %s as it was stored", async (stored) => {
    localStorage.setItem(KEY, stored);
    const { getTheme } = await theme();
    expect(getTheme()).toBe(stored);
  });

  it("reads anything else as System", async () => {
    localStorage.setItem(KEY, "sepia");
    const { getTheme } = await theme();
    expect(getTheme()).toBe("system");
  });

  it("stores each of the three choices under its own name", async () => {
    const { setTheme } = await theme();
    for (const choice of ["dark", "light", "system"] as const) {
      setTheme(choice);
      expect(localStorage.getItem(KEY)).toBe(choice);
    }
  });
});

describe("what System paints", () => {
  it("is the device's scheme, on the page and on the browser's own chrome", async () => {
    prefersDark = true;
    const { applyStoredTheme, getResolvedTheme } = await theme();
    applyStoredTheme();
    expect(painted()).toBe("dark");
    expect(getResolvedTheme()).toBe("dark");
    expect(chrome()).toBe("#161616");
  });

  it("follows the device when it switches while the app is open", async () => {
    const { applyStoredTheme, getResolvedTheme } = await theme();
    applyStoredTheme();
    expect(painted()).toBe("light");
    expect(chrome()).toBe("#f4f4f4");

    deviceTurns(true);
    expect(painted()).toBe("dark");
    expect(getResolvedTheme()).toBe("dark");
    expect(chrome()).toBe("#161616");

    deviceTurns(false);
    expect(painted()).toBe("light");
  });

  it("listens to the device once, however often the theme is applied", async () => {
    const { applyStoredTheme } = await theme();
    applyStoredTheme();
    applyStoredTheme();
    expect(listeners).toHaveLength(1);
  });
});

describe("a theme that was chosen", () => {
  it.each([
    ["dark", false],
    ["light", true],
  ] as const)("stays %s whatever the device prefers", async (stored, deviceDark) => {
    localStorage.setItem(KEY, stored);
    prefersDark = deviceDark;
    const { applyStoredTheme } = await theme();
    applyStoredTheme();
    expect(painted()).toBe(stored);

    deviceTurns(!deviceDark);
    expect(painted()).toBe(stored);
  });

  it("gives way to the device again once System is chosen", async () => {
    localStorage.setItem(KEY, "light");
    prefersDark = true;
    const { applyStoredTheme, setTheme } = await theme();
    applyStoredTheme();
    expect(painted()).toBe("light");

    setTheme("system");
    expect(painted()).toBe("dark");
    deviceTurns(false);
    expect(painted()).toBe("light");
  });

  it("holds against the device from the moment it is chosen", async () => {
    const { applyStoredTheme, setTheme } = await theme();
    applyStoredTheme();

    setTheme("dark");
    deviceTurns(false);
    expect(painted()).toBe("dark");
  });
});

describe("OLED", () => {
  it("is off in a browser that never chose it", async () => {
    const { applyStoredTheme, getOled } = await theme();
    applyStoredTheme();
    expect(getOled()).toBe(false);
    expect(oledPainted()).toBe(false);
  });

  it("puts the dark theme and the browser's chrome on black, and is kept in this browser", async () => {
    localStorage.setItem(KEY, "dark");
    const { applyStoredTheme, setOled } = await theme();
    applyStoredTheme();
    setOled(true);
    expect(oledPainted()).toBe(true);
    expect(chrome()).toBe("#000000");
    expect(localStorage.getItem("bv-oled")).toBe("1");

    setOled(false);
    expect(oledPainted()).toBe(false);
    expect(chrome()).toBe("#161616");
    expect(localStorage.getItem("bv-oled")).toBeNull();
  });

  it("stays on through the light theme, which it leaves as it is", async () => {
    localStorage.setItem("bv-oled", "1");
    const { applyStoredTheme } = await theme();
    applyStoredTheme();
    expect(painted()).toBe("light");
    expect(oledPainted()).toBe(true);
    expect(chrome()).toBe("#f4f4f4");

    deviceTurns(true);
    expect(chrome()).toBe("#000000");
  });

  it("takes the phone's choice inside the app over this page's own storage", async () => {
    localStorage.setItem(KEY, "dark");
    localStorage.setItem("bv-oled", "1");
    inApp(false);
    const { applyStoredTheme, getOled } = await theme();
    applyStoredTheme();
    expect(getOled()).toBe(false);
    expect(chrome()).toBe("#161616");
  });

  it("hands a change inside the app to the phone, which keeps it for every server", async () => {
    localStorage.setItem(KEY, "dark");
    const posted = inApp(false);
    const { applyStoredTheme, getOled, setOled } = await theme();
    applyStoredTheme();
    setOled(true);
    expect(getOled()).toBe(true);
    expect(posted.map((m) => JSON.parse(m))).toContainEqual({ op: "oled", on: true });
    expect(chrome()).toBe("#000000");
  });
});

describe("whether dark shows", () => {
  it("tells a watcher when the painted theme turns", async () => {
    const { applyStoredTheme, darkShowing, watchDarkShowing } = await theme();
    applyStoredTheme();
    expect(darkShowing()).toBe(false);
    const seen: boolean[] = [];
    const stop = watchDarkShowing(() => seen.push(darkShowing()));
    deviceTurns(true);
    await Promise.resolve();
    expect(seen).toEqual([true]);
    stop();
  });
});

describe("the device's colour", () => {
  it("takes the tone of the theme it turns to", async () => {
    const { setTheme } = await theme();
    const { chooseSystemAccent, setSystemAccent } = await import("./accent");
    setTheme("dark");
    setSystemAccent({ light: "#3F5F91", dark: "#A8C8FF" });
    chooseSystemAccent();
    setTheme("light");
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe("#3F5F91");
    document.documentElement.removeAttribute("style");
  });
});
