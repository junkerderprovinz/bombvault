// @vitest-environment jsdom
// The shock the logo's burst sends down the rail. jsdom checks which rows get
// which turn and when the rail carries the class; the shaking itself is CSS.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const bursts: (() => void)[] = [];
vi.mock("../lib/logoBurst", () => ({
  logoBurst: (_img: HTMLImageElement, done: () => void) => {
    bursts.push(done);
    return () => undefined;
  },
}));

import { I18nProvider } from "../lib/i18n";
import { BURST_AT, REBORN_MS } from "../lib/useLogoHold";
import { Sidebar } from "./Sidebar";

function renderSidebar() {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <I18nProvider>
        <Sidebar settings={null} authEnabled />
      </I18nProvider>
    </MemoryRouter>
  );
}

const rail = () => screen.getByTestId("desktop-sidebar");
const rows = () => [...document.querySelectorAll<HTMLElement>("aside .glim-nav-row")];
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

describe("the shock down the rail", () => {
  it("numbers every row from the top, the footer included, and alternates their direction", () => {
    renderSidebar();
    const all = rows();
    expect(all.length).toBeGreaterThan(3);
    all.forEach((row, i) => {
      expect(row.style.getPropertyValue("--rail-i")).toBe(String(i));
      expect(row.style.getPropertyValue("--rail-dir")).toBe(i % 2 ? "-1" : "1");
    });
  });

  it("runs while the burst flies and stops when the mark grows back", () => {
    renderSidebar();
    const logo = screen.getByRole("button", { name: /overview/i });
    fireEvent.pointerDown(logo, { button: 0 });
    advance(BURST_AT - 1);
    expect(rail().className).not.toContain("glim-rail-quake");
    advance(1);
    expect(rail().className).toContain("glim-rail-quake");
    act(() => bursts[0]());
    expect(rail().className).not.toContain("glim-rail-quake");
    advance(REBORN_MS);
    expect(rail().className).not.toContain("glim-rail-quake");
  });
});
