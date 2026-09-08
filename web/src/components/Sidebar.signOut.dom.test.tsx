// @vitest-environment jsdom
/**
 * The sidebar's sign-out row (jdp).
 *
 * Two things, and the second is the one worth a test: the row appears when a
 * login password is set, and it is absent when there is none. An instance
 * without a password has nothing to sign out of, and a control that offers to
 * do nothing is worse than no control.
 *
 * It also has to sit ABOVE the view toggle, because that is where it was asked
 * for and because the footer's order is the only thing distinguishing it from
 * every other row in that column.
 */
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
    // Node.compareDocumentPosition: FOLLOWING (4) means `view` comes after.
    expect(out.compareDocumentPosition(view) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("signs out and reloads, which is what puts the login screen back", async () => {
    draw(true);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /sign out/i }));
    });
    expect(logout).toHaveBeenCalledTimes(1);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("hides its label in glyph mode without losing its accessible name", () => {
    // The storage key is `bv-labels-sidebar`, NOT `glim-`: the prefix sweep
    // renamed the CSS classes and deliberately left the storage keys alone. The
    // first version of this test used glim-, never entered glyph mode, and
    // therefore passed for the wrong reason.
    localStorage.setItem("bv-labels-sidebar", "glyph");
    draw(true);
    const out = screen.getByRole("button", { name: /sign out/i });
    const label = within(out).getByText(/sign out/i);
    // Hidden, never removed: sr-only is what makes the row readable to a screen
    // reader while the words are off the screen. In any other mode this class is
    // absent, so the assertion fails if the mode did not take.
    expect(label.className).toContain("sr-only");
  });
});
