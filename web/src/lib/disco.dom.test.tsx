// @vitest-environment jsdom
// Disco: a persisted walk would write localStorage and sync to the server on
// every frame, so only the switch is stored. The unlock gesture counts
// turn-ons inside a time window, so comparing rainbow on and off does not
// trigger it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DISCO_TICK_MS,
  DISCO_UNLOCK_CLICKS,
  DISCO_UNLOCK_WINDOW_MS,
  applyStoredDisco,
  discoTap,
  getDisco,
  setDisco,
  stopDisco,
} from "./disco";
import { RAINBOW, getRainbow, rainbowAt, rainbowState, setRainbow } from "./appearance";
import { contrastOn } from "./accent";

const STORAGE_KEY = "bv-disco";

// jsdom has no matchMedia. The walk asks it once and keeps the answer, so the
// stub reads `reduceMotion` live.
let reduceMotion = false;
window.matchMedia = ((query: string) => ({
  media: query,
  get matches() {
    return reduceMotion;
  },
})) as unknown as typeof window.matchMedia;

const root = document.documentElement;
const hue = (i: number) => root.style.getPropertyValue(`--rb-${i}`);

beforeEach(() => {
  localStorage.clear();
  root.removeAttribute("data-rainbow");
  root.removeAttribute("data-disco");
  root.removeAttribute("data-motion");
  reduceMotion = false;
  vi.useFakeTimers();
});

afterEach(() => {
  setDisco(false);
  vi.useRealTimers();
});

describe("the switch", () => {
  it("is off when nothing is stored", () => {
    expect(getDisco()).toBe(false);
  });

  it("persists and reads back", () => {
    setDisco(true);
    expect(localStorage.getItem(STORAGE_KEY)).toBe("true");
    expect(getDisco()).toBe(true);
    setDisco(false);
    expect(getDisco()).toBe(false);
  });

  it("is off for a corrupt stored value", () => {
    localStorage.setItem(STORAGE_KEY, "{{");
    expect(getDisco()).toBe(false);
  });
});

describe("the walk", () => {
  it("starts from the colours at rest", () => {
    setRainbow({ on: true, rotate: true, seed: 3 });
    setDisco(true);
    vi.advanceTimersByTime(20);
    expect(hue(0)).toBe(RAINBOW[3]);
  });

  it("glides through the colours between two palette entries", () => {
    setRainbow({ on: true, seed: 0 });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2);
    expect(RAINBOW).not.toContain(hue(0));
  });

  it("keeps moving from one frame to the next", () => {
    setRainbow({ on: true });
    setDisco(true);
    const seen = new Set<string>();
    for (let i = 0; i < 20; i++) {
      vi.advanceTimersByTime(DISCO_TICK_MS / 20);
      seen.add(hue(0));
    }
    expect(seen.size).toBe(20);
  });

  it("writes the ink for the colour it paints", () => {
    setRainbow({ on: true, palette: ["#161616", ...RAINBOW.slice(1)] });
    setDisco(true);
    for (let i = 0; i < 8; i++) {
      vi.advanceTimersByTime(DISCO_TICK_MS / 3);
      expect(root.style.getPropertyValue("--rb-ink-7")).toBe(contrastOn(hue(7)));
    }
  });

  it("comes back round to where it started after a full turn", () => {
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(20);
    const first = hue(2);
    vi.advanceTimersByTime(DISCO_TICK_MS * RAINBOW.length);
    expect(hue(2)).toBe(first);
  });

  it("leaves the rainbow state and the stored look alone", () => {
    setRainbow({ on: true, seed: 0 });
    setDisco(true);
    const stored = localStorage.getItem("bv-rainbow");
    vi.advanceTimersByTime(DISCO_TICK_MS * 5);
    expect(localStorage.getItem("bv-rainbow")).toBe(stored);
    expect(getRainbow().seed).toBe(0);
    expect(rainbowState().seed).toBe(0);
  });

  it("does not run while rainbow is off", () => {
    setRainbow({ on: false });
    setDisco(true);
    const before = hue(0);
    vi.advanceTimersByTime(DISCO_TICK_MS * 3);
    expect(hue(0)).toBe(before);
  });

  it("does not run while the switch is off", () => {
    setRainbow({ on: true });
    applyStoredDisco(false);
    const before = hue(0);
    vi.advanceTimersByTime(DISCO_TICK_MS * 3);
    expect(hue(0)).toBe(before);
  });

  it("walks at one pace however often it is applied", () => {
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS);
    const once = hue(0);
    setDisco(false);

    setDisco(true);
    applyStoredDisco();
    applyStoredDisco();
    vi.advanceTimersByTime(DISCO_TICK_MS);
    expect(hue(0)).toBe(once);
  });

  it("carries on from where it is when applied again", () => {
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2);
    const halfway = hue(0);
    applyStoredDisco();
    vi.advanceTimersByTime(20);
    const channels = (hex: string) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
    const now = channels(hue(0));
    channels(halfway).forEach((c, i) => expect(Math.abs(c - now[i])).toBeLessThanOrEqual(3));
  });

  it("puts the resting palette back when the switch goes off", () => {
    setRainbow({ on: true, rotate: false, seed: 0 });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS * 2.5);
    setDisco(false);
    vi.advanceTimersByTime(DISCO_TICK_MS);
    for (let i = 0; i < RAINBOW.length; i++) expect(hue(i)).toBe(rainbowAt(i));
    expect(rainbowState().rotate).toBe(false);
  });

  it("keeps a chosen rotation and seed when it stops", () => {
    setRainbow({ on: true, rotate: true, seed: 3 });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS * 2);
    setDisco(false);
    expect(rainbowState().rotate).toBe(true);
    expect(rainbowState().seed).toBe(3);
    expect(hue(0)).toBe(RAINBOW[3]);
  });

  it("leaves the colours where the last frame put them when stopped", () => {
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2);
    stopDisco();
    const frozen = hue(0);
    vi.advanceTimersByTime(DISCO_TICK_MS);
    expect(hue(0)).toBe(frozen);
    stopDisco();
  });

  it("stops when rainbow goes off", () => {
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2);
    setRainbow({ on: false });
    applyStoredDisco();
    vi.advanceTimersByTime(DISCO_TICK_MS);
    expect(hue(0)).toBe(RAINBOW[0]);
  });

  it("sets data-disco on the document while it is on", () => {
    setRainbow({ on: true });
    setDisco(true);
    expect(root.getAttribute("data-disco")).toBe("on");
    setDisco(false);
    expect(root.hasAttribute("data-disco")).toBe(false);
  });

  it("steps a colour at a time at the off level", () => {
    root.setAttribute("data-motion", "off");
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2);
    expect(hue(0)).toBe(RAINBOW[0]);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2 + 20);
    expect(hue(0)).toBe(RAINBOW[1]);
  });

  it("steps a colour at a time when the system asks for reduced motion", () => {
    reduceMotion = true;
    setRainbow({ on: true });
    setDisco(true);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2);
    expect(hue(0)).toBe(RAINBOW[0]);
    vi.advanceTimersByTime(DISCO_TICK_MS / 2 + 20);
    expect(hue(0)).toBe(RAINBOW[1]);
  });
});

describe("the unlock gesture", () => {
  const at = (ms: number) => ({ now: ms });

  it("opens on the fifth turn-on", () => {
    const s = { taps: 0, last: 0 };
    for (let i = 1; i < DISCO_UNLOCK_CLICKS; i += 1) {
      expect(discoTap(s, true, at(i * 500))).toBe(false);
    }
    expect(discoTap(s, true, at(DISCO_UNLOCK_CLICKS * 500))).toBe(true);
  });

  it("ignores turn-offs", () => {
    const s = { taps: 0, last: 0 };
    for (let i = 0; i < DISCO_UNLOCK_CLICKS * 2; i += 1) {
      expect(discoTap(s, false, at(i * 500))).toBe(false);
    }
    expect(s.taps).toBe(0);
  });

  it("forgets the count when the clicks are too far apart", () => {
    const s = { taps: 0, last: 0 };
    discoTap(s, true, at(0));
    discoTap(s, true, at(DISCO_UNLOCK_WINDOW_MS + 1));
    // The second click starts a new run.
    expect(s.taps).toBe(1);
  });

  it("resets the count once it opens", () => {
    const s = { taps: 0, last: 0 };
    for (let i = 1; i <= DISCO_UNLOCK_CLICKS; i += 1) discoTap(s, true, at(i * 500));
    expect(s.taps).toBe(0);
  });
});
