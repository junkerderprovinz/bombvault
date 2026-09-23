// @vitest-environment jsdom
// The colour wipe runs for a flip somebody made, never for a look that merely
// arrived. main.tsx applies every stored axis before first paint and again when
// the server hands this browser a different stored look; animating that second
// apply would walk every hued element to its rainbow hue and back right after
// paint. Both sides are pinned: a real flip still wipes, and an adopted look
// skips the wipe but its value still lands.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const WIPE = "glim-colour-wipe";

async function freshModule() {
  vi.resetModules();
  return import("./appearance");
}

beforeEach(() => {
  document.documentElement.className = "";
  document.documentElement.removeAttribute("data-rainbow");
  localStorage.clear();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("colour wipe", () => {
  it("stays quiet on the boot apply, because nothing is on screen yet", async () => {
    const a = await freshModule();
    a.applyRainbow({ on: true });
    expect(document.documentElement.classList.contains(WIPE)).toBe(false);
    expect(document.documentElement.getAttribute("data-rainbow")).toBe("on");
  });

  it("wipes on a genuine flip after mount", async () => {
    const a = await freshModule();
    a.applyRainbow({ on: false }); // boot
    a.applyRainbow({ on: true }); // somebody switched it on
    expect(document.documentElement.classList.contains(WIPE)).toBe(true);
  });

  it("does not wipe when a look arrives from the server", async () => {
    const a = await freshModule();
    a.applyRainbow({ on: false }); // boot, from localStorage
    a.applyRainbow({ on: true }, { animate: false }); // the adopted look
    expect(
      document.documentElement.classList.contains(WIPE),
      "an adopted look animated a change nobody made: every hued element walks " +
        "from the flat accent to its own hue across the whole page, right after paint (#228)",
    ).toBe(false);
  });

  it("still applies the adopted value, it only skips the excursion", async () => {
    const a = await freshModule();
    a.applyRainbow({ on: false });
    a.applyRainbow({ on: true }, { animate: false });
    expect(
      document.documentElement.getAttribute("data-rainbow"),
      "the adopted look must still LAND - suppressing the animation must not suppress the value",
    ).toBe("on");
  });

  it("a later real flip still wipes after an adopted one", async () => {
    // Adopting a look must not disarm the gate for the flips after it.
    const a = await freshModule();
    a.applyRainbow({ on: false });
    a.applyRainbow({ on: true }, { animate: false });
    document.documentElement.classList.remove(WIPE);
    a.applyRainbow({ on: false });
    expect(document.documentElement.classList.contains(WIPE)).toBe(true);
  });

  it("a re-apply of the identical value never wipes, animated or not", async () => {
    const a = await freshModule();
    a.applyRainbow({ on: true });
    a.applyRainbow({ on: true });
    expect(document.documentElement.classList.contains(WIPE)).toBe(false);
  });
});
