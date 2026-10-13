// @vitest-environment jsdom
// The shock the overview's logo sends through the phone's bottom bar. jsdom
// checks which slots get which turn and when the bar carries the class; the
// shaking itself is CSS.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const bursts: (() => void)[] = [];
vi.mock("../../lib/logoBurst", () => ({
  logoBurst: (_img: HTMLImageElement, done: () => void) => {
    bursts.push(done);
    return () => undefined;
  },
}));

import { I18nProvider } from "../../lib/i18n";
import { BURST_AT, useLogoHold } from "../../lib/useLogoHold";
import { BottomNav } from "./BottomNav";

function Logo() {
  const hold = useLogoHold();
  return (
    <div data-testid="logo" {...hold.handlers}>
      <img ref={hold.markRef} src="/logo.svg" alt="" />
    </div>
  );
}

function draw() {
  return render(
    <MemoryRouter initialEntries={["/dashboard"]}>
      <I18nProvider>
        <Logo />
        <BottomNav settings={null} authEnabled={false} scrollMainToTop={() => undefined} />
      </I18nProvider>
    </MemoryRouter>,
  );
}

const card = () => screen.getByTestId("bottom-nav").querySelector<HTMLElement>("div.rounded-card")!;
const slots = () => [...document.querySelectorAll<HTMLElement>("[data-testid=bottom-nav] .glim-bar-slot")];
const advance = (ms: number) => act(() => vi.advanceTimersByTime(ms));

beforeEach(() => {
  vi.useFakeTimers();
  bursts.length = 0;
  localStorage.clear();
  document.documentElement.setAttribute("data-motion", "wild");
  vi.spyOn(window, "matchMedia").mockImplementation(
    (query: string) =>
      ({ matches: false, media: query, addEventListener: () => undefined, removeEventListener: () => undefined }) as unknown as MediaQueryList,
  );
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
  localStorage.clear();
});

describe("the shock through the bottom bar", () => {
  it("numbers every slot from the start, More included, and alternates their direction", () => {
    draw();
    const all = slots();
    expect(all.length).toBeGreaterThan(3);
    all.forEach((slot, i) => {
      expect(slot.style.getPropertyValue("--rail-i")).toBe(String(i));
      expect(slot.style.getPropertyValue("--rail-dir")).toBe(i % 2 ? "-1" : "1");
    });
  });

  it("runs while the overview's logo flies apart and stops when the debris has settled", () => {
    draw();
    fireEvent.pointerDown(screen.getByTestId("logo"), { button: 0 });
    advance(BURST_AT - 1);
    expect(card().className).not.toContain("glim-rail-quake");
    advance(1);
    expect(card().className).toContain("glim-rail-quake");
    act(() => bursts[0]());
    expect(card().className).not.toContain("glim-rail-quake");
  });
});
