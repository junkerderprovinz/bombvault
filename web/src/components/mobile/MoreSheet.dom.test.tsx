// @vitest-environment jsdom
/**
 * MoreSheet's dom contract.
 *
 * The sheet is thin by design — the stateful half (portal, Escape, scrim,
 * Tab trap, focus restore) is BottomSheet's, already proven there — so what
 * is left to prove here is exactly the content contract:
 *   - rows ARE moreDestinations(settings), in registry order, never a second
 *     hand-written list (the drift the one registry exists to kill), and bar
 *     destinations never leak into the sheet;
 *   - the current route's row carries the accent pair (accentSoft backdrop +
 *     accentText label) — on /vms or /flash no bar slot is active, this row
 *     is, so a user is never lost (the locked Interaction Contract);
 *   - the sign-out row is gated by authEnabled exactly like the desktop
 *     footer, sits LAST behind a hairline, is muted, and fires the Sidebar
 *     sign-out mechanism verbatim — best-effort logout then a location
 *     reload, with NO confirmation dialog anywhere in the flow.
 *
 * The api mock follows Sidebar.signOut.dom.test.tsx's pattern: logout is
 * intercepted so the real network layer never loads in jsdom, and
 * globalThis.location gets a stubbed reload so the reload assertion cannot
 * actually navigate the test runner.
 */
import { render, screen, cleanup, fireEvent, act, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Settings } from "../../lib/api";

const logout = vi.fn(async () => ({ ok: true }));
vi.mock("../../lib/api", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  logout: () => logout(),
}));

const reload = vi.fn();
Object.defineProperty(globalThis, "location", {
  value: { ...globalThis.location, reload },
  writable: true,
});

import { MoreSheet } from "./MoreSheet";

// A fresh-DB-plus-two-tabs fixture, the navModel.test.ts idiom (`as Settings`
// partials): only vms and flash are switched on, so moreDestinations() must
// yield exactly Recovery, VMs, Flash — in that registry order.
const VMS_FLASH_ON = { vmsEnabled: true, flashEnabled: true } as Settings;

/** Renders the open sheet under a router, plus a live pathname probe so the
 *  navigate assertion reads the router's own state instead of guessing. */
function draw(opts: { settings?: Settings | null; authEnabled?: boolean; path?: string } = {}) {
  const onClose = vi.fn();
  function PathProbe() {
    const location = useLocation();
    return <div data-testid="path-probe">{location.pathname}</div>;
  }
  const view = render(
    <MemoryRouter initialEntries={[opts.path ?? "/dashboard"]}>
      <PathProbe />
      <MoreSheet open onClose={onClose} settings={opts.settings ?? null} authEnabled={opts.authEnabled ?? false} />
    </MemoryRouter>,
  );
  return { onClose, view };
}

function sheet() {
  return screen.getByTestId("more-sheet");
}

beforeEach(() => {
  logout.mockClear();
  reload.mockClear();
});
afterEach(cleanup);

describe("MoreSheet rows are the ONE registry", () => {
  it("lists exactly moreDestinations(settings), in registry order", () => {
    draw({ settings: VMS_FLASH_ON });
    const labels = within(sheet())
      .getAllByRole("link")
      .map((row) => row.textContent);
    expect(labels).toEqual(["Recovery", "VMs", "Flash"]);
  });

  it("hides gated tabs like the desktop Sidebar and never shows bar destinations", () => {
    draw({ settings: null, authEnabled: false });
    // null settings = every gate off: only always-on Recovery remains.
    expect(within(sheet()).getAllByRole("link").map((row) => row.textContent)).toEqual(["Recovery"]);
    // Bar members (dashboard/containers/settings on a fresh DB) are the
    // bottom bar's slots — a second copy of them here would be the exact
    // duplicate-destination drift the registry forbids.
    expect(screen.queryByText("Dashboard")).toBeNull();
    expect(screen.queryByText("Containers")).toBeNull();
    expect(screen.queryByText("Settings")).toBeNull();
  });

  it("gives every row the 52px minimum height and the sheet its testid", () => {
    draw({ settings: VMS_FLASH_ON });
    for (const row of within(sheet()).getAllByRole("link")) {
      expect(row.className).toContain("min-h-[3.25rem]");
    }
  });
});

describe("MoreSheet active row accents (the Interaction Contract)", () => {
  it("reads the accent pair on the current route's row and not on the others", () => {
    draw({ settings: VMS_FLASH_ON, path: "/vms" });
    const active = within(sheet()).getByRole("link", { name: /vms/i });
    expect(active.className).toContain("bg-accentSoft");
    expect(active.className).toContain("text-accentText");
    // Resting rows carry neither accent class — the accent belongs to the
    // one row whose route is current.
    const resting = within(sheet()).getByRole("link", { name: /recovery/i });
    expect(resting.className).not.toContain("bg-accentSoft");
    expect(resting.className).not.toContain("text-accentText");
  });
});

describe("MoreSheet sign-out (the desktop mechanism, muted, last)", () => {
  it("is absent while no password is set", () => {
    draw({ authEnabled: false });
    expect(screen.queryByRole("button", { name: /sign out/i })).toBeNull();
  });

  it("sits last in the sheet, behind a hairline, muted, with its glyph", () => {
    draw({ settings: VMS_FLASH_ON, authEnabled: true });
    const out = screen.getByRole("button", { name: /sign out/i });
    const content = sheet();
    expect(within(content).getByRole("button", { name: /sign out/i })).toBeTruthy();
    // LAST row group: the sign-out group is the content root's final child
    // (destination rows above it, never below — the locked placement).
    const group = out.parentElement as HTMLElement;
    expect(group.lastElementChild).toBe(out);
    expect(content.lastElementChild).toBe(group);
    // Hairline separation via the border TOKEN, and the muted text token on
    // the row — the visually-quiet treatment the desktop footer gets.
    expect(group.className).toContain("border-carbon-border");
    expect(out.className).toContain("text-carbon-textMuted");
    // The power glyph is present (muted styling is the classes above; the
    // glyph itself is what makes the row scannable as an action, not a link).
    expect(out.querySelector("svg")).not.toBeNull();
  });

  it("signs out and reloads with NO confirmation anywhere in the flow", async () => {
    draw({ authEnabled: true });
    // One click, no dialog between: logout fires (best-effort) and the
    // reload that puts the login screen back happens immediately after.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /sign out/i }));
    });
    expect(logout).toHaveBeenCalledTimes(1);
    expect(reload).toHaveBeenCalledTimes(1);
  });
});

describe("MoreSheet row navigation closes the sheet", () => {
  it("navigates to the row's route and closes so the destination is visible", async () => {
    const { onClose } = draw({ settings: VMS_FLASH_ON });
    await act(async () => {
      fireEvent.click(within(sheet()).getByRole("link", { name: /recovery/i }));
    });
    expect(screen.getByTestId("path-probe").textContent).toBe("/recovery");
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
