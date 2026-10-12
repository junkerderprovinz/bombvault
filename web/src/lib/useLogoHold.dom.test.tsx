// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const bursts: { done: () => void; stop: ReturnType<typeof vi.fn> }[] = [];
vi.mock("./logoBurst", () => ({
  logoBurst: (_img: HTMLImageElement, done: () => void) => {
    const stop = vi.fn();
    bursts.push({ done, stop });
    return stop;
  },
}));

import { BURST_AT, QUAKE_AT, REBORN_MS, useLogoHold } from "./useLogoHold";

const navigate = vi.fn();

function Logo() {
  const hold = useLogoHold(navigate);
  return (
    <button type="button" {...hold.handlers} data-stage={hold.stage} data-phase={hold.phase}>
      <img ref={hold.markRef} src="/logo.svg" alt="BombVault" />
    </button>
  );
}

const logo = () => screen.getByRole("button");
const stage = () => logo().getAttribute("data-stage");
const phase = () => logo().getAttribute("data-phase");
const advance = (ms: number) => act(() => vi.advanceTimersByTime(ms));
const press = () => fireEvent.pointerDown(logo(), { button: 0 });
const release = () => fireEvent.pointerUp(logo());
// A pointer click carries its click count in `detail`; a keyboard one has 0.
const pointerClick = () => fireEvent.click(logo(), { detail: 1 });

function reducedMotion(on: boolean) {
  vi.spyOn(window, "matchMedia").mockImplementation(
    (query: string) =>
      ({
        matches: on && query.includes("prefers-reduced-motion: reduce"),
        media: query,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
      }) as unknown as MediaQueryList,
  );
}

beforeEach(() => {
  vi.useFakeTimers();
  bursts.length = 0;
  navigate.mockClear();
  document.documentElement.setAttribute("data-motion", "wild");
  reducedMotion(false);
  render(<Logo />);
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("holding the logo", () => {
  it("shakes in three growing stages and then goes off", () => {
    press();
    advance(QUAKE_AT[0] - 1);
    expect(stage()).toBe("0");
    advance(1);
    expect(stage()).toBe("1");
    advance(QUAKE_AT[1] - QUAKE_AT[0]);
    expect(stage()).toBe("2");
    advance(QUAKE_AT[2] - QUAKE_AT[1]);
    expect(stage()).toBe("3");
    expect(bursts).toHaveLength(0);

    advance(BURST_AT - QUAKE_AT[2]);
    expect(bursts).toHaveLength(1);
    expect(stage()).toBe("0");
    expect(phase()).toBe("gone");
  });

  it("grows the mark back once the burst has settled, then rests", () => {
    press();
    advance(BURST_AT);
    act(() => bursts[0].done());
    expect(phase()).toBe("reborn");
    advance(REBORN_MS);
    expect(phase()).toBe("idle");
  });

  it("stops shaking and never goes off when released early", () => {
    press();
    advance(QUAKE_AT[1] + 10);
    expect(stage()).toBe("2");
    release();
    expect(stage()).toBe("0");
    advance(BURST_AT);
    expect(bursts).toHaveLength(0);
  });

  it("also lets go when the pointer leaves or is cancelled", () => {
    for (const end of [() => fireEvent.pointerLeave(logo()), () => fireEvent.pointerCancel(logo())]) {
      press();
      advance(QUAKE_AT[0] + 10);
      end();
      advance(BURST_AT);
      expect(stage()).toBe("0");
    }
    expect(bursts).toHaveLength(0);
  });

  it("does not start a second burst while the first is still flying", () => {
    press();
    advance(BURST_AT);
    release();
    press();
    advance(BURST_AT * 2);
    expect(bursts).toHaveLength(1);
  });

  it("stops the burst when the logo goes away mid-flight", () => {
    press();
    advance(BURST_AT);
    cleanup();
    expect(bursts[0].stop).toHaveBeenCalledOnce();
  });

  it("ignores any button but the main one", () => {
    fireEvent.pointerDown(logo(), { button: 2 });
    advance(BURST_AT);
    expect(stage()).toBe("0");
    expect(bursts).toHaveLength(0);
  });
});

describe("with motion stopped", () => {
  it("does nothing at the off level, and a click still navigates", () => {
    document.documentElement.setAttribute("data-motion", "off");
    press();
    advance(BURST_AT * 2);
    expect(stage()).toBe("0");
    expect(bursts).toHaveLength(0);
    release();
    pointerClick();
    expect(navigate).toHaveBeenCalledOnce();
  });

  it("does nothing when the system asks for less motion", () => {
    reducedMotion(true);
    press();
    advance(BURST_AT * 2);
    expect(stage()).toBe("0");
    expect(bursts).toHaveLength(0);
  });

  it("still goes off for the storm, which outranks the system", () => {
    reducedMotion(true);
    document.documentElement.setAttribute("data-motion", "storm");
    press();
    advance(BURST_AT);
    expect(bursts).toHaveLength(1);
  });
});

describe("clicking the logo", () => {
  it("navigates on a short click", () => {
    press();
    advance(QUAKE_AT[0] - 50);
    release();
    pointerClick();
    expect(navigate).toHaveBeenCalledOnce();
  });

  it("swallows the click that ends a hold once the shake has started", () => {
    press();
    advance(QUAKE_AT[0] + 10);
    release();
    pointerClick();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("swallows the release after the burst and clicks while the debris flies", () => {
    press();
    advance(BURST_AT);
    release();
    pointerClick();
    press();
    release();
    pointerClick();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("navigates again once the mark is back", () => {
    press();
    advance(BURST_AT);
    release();
    pointerClick();
    act(() => bursts[0].done());
    advance(REBORN_MS);
    press();
    release();
    pointerClick();
    expect(navigate).toHaveBeenCalledOnce();
  });

  it("always navigates from the keyboard", () => {
    press();
    advance(BURST_AT);
    release();
    fireEvent.click(logo(), { detail: 0 });
    expect(navigate).toHaveBeenCalledOnce();
  });
});
