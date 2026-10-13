// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { BURST_SECONDS, logoBurst } from "./logoBurst";

// jsdom has no 2D context; this one accepts every call and records the shards.
let shardsDrawn = 0;
function fakeContext(): CanvasRenderingContext2D {
  const noop = () => undefined;
  return new Proxy({} as CanvasRenderingContext2D, {
    get: (_, key) => {
      if (key === "createRadialGradient") return () => ({ addColorStop: noop });
      if (key === "drawImage") return () => (shardsDrawn += 1);
      return noop;
    },
    set: () => true,
  });
}

function mark(natural: { w: number; h: number }): HTMLImageElement {
  const img = document.createElement("img");
  Object.defineProperty(img, "complete", { value: true });
  Object.defineProperty(img, "naturalWidth", { value: natural.w });
  Object.defineProperty(img, "naturalHeight", { value: natural.h });
  img.getBoundingClientRect = () => ({ left: 60, top: 24, width: 104, height: 104, right: 164, bottom: 128, x: 60, y: 24, toJSON: () => ({}) });
  document.body.appendChild(img);
  return img;
}

const canvases = () => document.querySelectorAll("canvas.glim-logo-burst");

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "requestAnimationFrame", "cancelAnimationFrame", "performance"] });
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(fakeContext as never);
  shardsDrawn = 0;
});

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("logoBurst", () => {
  it("draws over the page and is gone once the debris has settled", () => {
    const done = vi.fn();
    logoBurst(mark({ w: 845, h: 814 }), done);
    expect(canvases()).toHaveLength(1);
    vi.advanceTimersByTime(500);
    expect(shardsDrawn).toBeGreaterThan(14 * 14);
    expect(done).not.toHaveBeenCalled();

    vi.advanceTimersByTime(BURST_SECONDS * 1000);
    expect(canvases()).toHaveLength(0);
    expect(done).toHaveBeenCalledOnce();
  });

  it("removes the canvas without finishing when stopped early", () => {
    const done = vi.fn();
    const stop = logoBurst(mark({ w: 845, h: 814 }), done);
    vi.advanceTimersByTime(200);
    stop();
    expect(canvases()).toHaveLength(0);
    vi.advanceTimersByTime(BURST_SECONDS * 2000);
    expect(done).not.toHaveBeenCalled();
  });

  it("draws the window-sized canvas on the CPU", () => {
    logoBurst(mark({ w: 845, h: 814 }), () => undefined);
    const asked = vi.mocked(HTMLCanvasElement.prototype.getContext).mock.calls[0];
    expect(asked).toEqual(["2d", { willReadFrequently: true }]);
  });

  it("caps the canvas at twice the window's size", () => {
    vi.spyOn(window, "devicePixelRatio", "get").mockReturnValue(3);
    logoBurst(mark({ w: 845, h: 814 }), () => undefined);
    const canvas = canvases()[0] as HTMLCanvasElement;
    expect(canvas.width).toBe(innerWidth * 2);
    expect(canvas.height).toBe(innerHeight * 2);
  });

  it("still goes off without shards when the picture has no natural size", () => {
    const done = vi.fn();
    logoBurst(mark({ w: 0, h: 0 }), done);
    vi.advanceTimersByTime(BURST_SECONDS * 1000 + 50);
    expect(shardsDrawn).toBe(0);
    expect(done).toHaveBeenCalledOnce();
    expect(canvases()).toHaveLength(0);
  });
});
