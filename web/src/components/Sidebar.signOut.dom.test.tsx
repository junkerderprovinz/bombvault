// @vitest-environment jsdom
import { render, screen, cleanup, fireEvent, act, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const logout = vi.fn(async () => ({ ok: true }));
vi.mock("../lib/api", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  logout: () => logout(),
}));

const reload = vi.fn();
Object.defineProperty(globalThis, "location", {
  value: { ...globalThis.location, reload },
  writable: true,
});

import { Sidebar } from "./Sidebar";

function draw(authEnabled: boolean) {
  return render(
    <MemoryRouter>
      <Sidebar settings={null} authEnabled={authEnabled} />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  logout.mockClear();
  reload.mockClear();
  localStorage.clear();
});
afterEach(cleanup);

describe("Sidebar sign-out", () => {
  it("is absent while no password is set", () => {
    draw(false);
    expect(screen.queryByRole("button", { name: /sign out/i })).toBeNull();
  });

  it("appears once a password is set", () => {
    draw(true);
    expect(screen.getByRole("button", { name: /sign out/i })).toBeTruthy();
  });

  it("sits above the view toggle", () => {
    draw(true);
    const out = screen.getByRole("button", { name: /sign out/i });
    const view = screen.getByRole("button", { name: /view/i });
    expect(out.compareDocumentPosition(view) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("signs out and reloads to bring back the login screen", async () => {
    draw(true);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /sign out/i }));
    });
    expect(logout).toHaveBeenCalledTimes(1);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("hides its label in glyph mode without losing its accessible name", () => {
    // Storage keys keep the bv- prefix, unlike the glim- CSS classes.
    localStorage.setItem("bv-labels-sidebar", "glyph");
    draw(true);
    const out = screen.getByRole("button", { name: /sign out/i });
    const label = within(out).getByText(/sign out/i);
    // No other mode sets sr-only, so this also fails if glyph mode did not take.
    expect(label.className).toContain("sr-only");
  });
});

describe("Sidebar sign-out and the labelling engine", () => {
  it("drops its glyph in text mode, like every nav row above it", () => {
    localStorage.setItem("bv-labels-sidebar", "text");
    draw(true);
    const out = screen.getByRole("button", { name: /sign out/i });
    expect(out.querySelector("svg")).toBeNull();
    expect(within(out).getByText(/sign out/i).className).not.toContain("sr-only");
  });

  it("keeps its glyph in glyph mode", () => {
    localStorage.setItem("bv-labels-sidebar", "glyph");
    draw(true);
    const out = screen.getByRole("button", { name: /sign out/i });
    expect(out.querySelector("svg")).not.toBeNull();
  });

  // The door's arrow points out along the reading direction, so it turns
  // round with the layout; the power mark it replaced was symmetric.
  it("wears a sign-out glyph that mirrors in a right-to-left layout", () => {
    localStorage.setItem("bv-labels-sidebar", "glyph");
    draw(true);
    const glyph = screen.getByRole("button", { name: /sign out/i }).querySelector("svg");
    expect(glyph?.getAttribute("class")).toContain("rtl:-scale-x-100");
  });

  it("carries glim-hue and a --item-hue of its own", () => {
    draw(true);
    const out = screen.getByRole("button", { name: /sign out/i });
    expect(out.className).toContain("glim-hue");
    expect(out.style.getPropertyValue("--item-hue")).toMatch(/^var\(--rb-[0-7]\)$/);
  });
});
