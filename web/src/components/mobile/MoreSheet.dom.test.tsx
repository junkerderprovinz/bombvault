// @vitest-environment jsdom
/**
 * MoreSheet's dom contract.
 *
 * The sheet is thin by design; the stateful half (portal, Escape, scrim,
 * Tab trap, focus restore) is BottomSheet's, already proven there; so what
 * is left to prove here is exactly the content contract:
 *   - rows are moreDestinations(settings), in registry order, never a second
 *     hand-written list (the drift the one registry exists to kill), and bar
 *     destinations never leak into the sheet;
 *   - every row sits on the colour engine (glim-hue + its own --item-hue) and
 *     the current route's row is filled (accent fill + contrast ink); on
 *     /vms or /flash no bar slot is active, this row is, so a user is never
 *     lost (the locked Interaction Contract);
 *   - the Simple/Advanced view toggle is always rendered with its pressed
 *     state and current-view label; the phone's only path to the
 *     advanced-only settings cards, and the permanent content that lets the
 *     bar's More trigger render unconditionally;
 *   - the sign-out row is gated by authEnabled exactly like the desktop
 *     footer, sits last set off by spacing alone, is muted, and fires the
 *     Sidebar sign-out mechanism verbatim; best-effort logout then a
 *     location reload, with no confirmation dialog anywhere in the flow;
 *   - the rows continue the caller's hue rotation (hueOffset) instead of
 *     restarting the palette at the sheet's first row.
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
import { AdvancedProvider } from "../../lib/advanced";

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
import { hueVars } from "../../lib/appearance";

// A fresh-DB-plus-two-tabs fixture, the navModel.test.ts idiom (`as Settings`
// partials): only vms and flash are switched on, so moreDestinations() must
// yield exactly VMs, Flash and the always-on Settings row; in that registry
// order (Recovery rides the phone bar, not this sheet).
const VMS_FLASH_ON = { vmsEnabled: true, flashEnabled: true } as Settings;

/** Renders the open sheet under a router, plus a live pathname probe so the
 *  navigate assertion reads the router's own state instead of guessing. */
function draw(
  opts: { settings?: Settings | null; authEnabled?: boolean; path?: string; hueOffset?: number } = {},
) {
  const onClose = vi.fn();
  function PathProbe() {
    const location = useLocation();
    return <div data-testid="path-probe">{location.pathname}</div>;
  }
  const view = render(
    <MemoryRouter initialEntries={[opts.path ?? "/dashboard"]}>
      <PathProbe />
      <AdvancedProvider>
        <MoreSheet
          open
          onClose={onClose}
          settings={opts.settings ?? null}
          authEnabled={opts.authEnabled ?? false}
          hueOffset={opts.hueOffset}
        />
      </AdvancedProvider>
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
  localStorage.removeItem("bombvault.advanced");
});
afterEach(cleanup);

describe("MoreSheet rows come from the one registry", () => {
  it("lists exactly moreDestinations(settings), in registry order", () => {
    draw({ settings: VMS_FLASH_ON });
    const labels = within(sheet())
      .getAllByRole("link")
      .map((row) => row.textContent);
    expect(labels).toEqual(["VMs", "Flash", "Settings"]);
  });

  it("hides gated tabs like the desktop Sidebar and never shows bar destinations", () => {
    draw({ settings: null, authEnabled: false });
    // null settings = every gate off, so only the always-on Settings row
    // remains: Recovery rides the phone bar and the gated tabs are off.
    expect(within(sheet()).getAllByRole("link").map((row) => row.textContent)).toEqual(["Settings"]);
    // Bar members (dashboard/containers/recovery on a fresh DB) are the
    // bottom bar's slots; a second copy of them here would be the exact
    // duplicate-destination drift the registry forbids.
    expect(screen.queryByText("Dashboard")).toBeNull();
    expect(screen.queryByText("Containers")).toBeNull();
    expect(screen.queryByText("Recovery")).toBeNull();
  });

  it("gives every row the 52px minimum height and the sheet its testid", () => {
    draw({ settings: VMS_FLASH_ON });
    for (const row of within(sheet()).getAllByRole("link")) {
      expect(row.className).toContain("min-h-[3.25rem]");
    }
  });
});

describe("MoreSheet rotation continuity", () => {
  it("starts the rows at the caller's offset, not at the palette's first colour", () => {
    // 4 = a bar of three destination slots plus the More trigger: the sheet
    // continues where the trigger left off (BottomNav passes
    // slots.length + 1).
    draw({ settings: VMS_FLASH_ON, hueOffset: 4 });
    const rows = within(sheet()).getAllByRole("link");
    const hues = rows.map((row) => (row as HTMLElement).style.getPropertyValue("--item-hue"));
    for (let i = 0; i < hues.length; i++) {
      expect(hues[i]).toBe(String(hueVars(4 + i)["--item-hue"]));
    }
    // The regression this pins: restarting at position 0 would give the
    // sheet's first row the bar's first slot's colour again.
    expect(hues[0]).not.toBe(String(hueVars(0)["--item-hue"]));
  });

  it("defaults to position 0 when mounted standalone", () => {
    draw({ settings: VMS_FLASH_ON });
    const first = within(sheet()).getAllByRole("link")[0] as HTMLElement;
    expect(first.style.getPropertyValue("--item-hue")).toBe(String(hueVars(0)["--item-hue"]));
  });
});

describe("MoreSheet active row accents (the Interaction Contract)", () => {
  it("fills the current route's row and not the others, the bar's own idiom", () => {
    draw({ settings: VMS_FLASH_ON, path: "/vms" });
    const active = within(sheet()).getByRole("link", { name: /vms/i });
    expect(active.className).toContain("bg-accent");
    expect(active.className).toContain("text-accentContrast");
    expect(active.className).toContain("glim-active");
    // Resting rows carry neither the fill nor the active marker; the accent
    // belongs to the one row whose route is current.
    const resting = within(sheet()).getByRole("link", { name: /flash/i });
    expect(resting.className).not.toContain("bg-accent");
    expect(resting.className).not.toContain("glim-active");
  });
});

describe("MoreSheet colour engine and view toggle", () => {
  it("every destination row sits on the engine: glim-hue plus its own --item-hue", () => {
    draw({ settings: VMS_FLASH_ON });
    const rows = within(sheet()).getAllByRole("link");
    const hues = rows.map((row) => (row as HTMLElement).style.getPropertyValue("--item-hue"));
    for (const hue of hues) {
      expect(hue).toMatch(/^var\(--rb-[0-7]\)$/);
    }
    // Consecutive palette positions are pairwise distinct.
    expect(new Set(hues).size).toBe(hues.length);
    for (const row of rows) {
      expect(row.className).toContain("glim-hue");
    }
  });

  it("carries the Simple/Advanced view toggle: pressed state, current-view label, always present", () => {
    draw({ settings: null });
    const toggle = screen.getByRole("button", { name: "Simple view" });
    expect(toggle.getAttribute("aria-pressed")).toBe("false");
    expect(toggle.className).toContain("glim-hue");
    fireEvent.click(toggle);
    // Clicking flips the view; the label shows the view it just switched to.
    expect(screen.getByRole("button", { name: "Advanced view" }).getAttribute("aria-pressed")).toBe("true");
    // With every gate off (no destination rows beyond Settings) the toggle is
    // still rendered; the permanent content the More trigger counts on.
    expect(within(sheet()).getAllByRole("link").map((r) => r.textContent)).toEqual(["Settings"]);
  });
});

describe("MoreSheet sign-out (the desktop mechanism, muted, last)", () => {
  it("is absent while no password is set", () => {
    draw({ authEnabled: false });
    expect(screen.queryByRole("button", { name: /sign out/i })).toBeNull();
  });

  it("sits last in the sheet, set off by spacing alone, muted, with its glyph", () => {
    draw({ settings: VMS_FLASH_ON, authEnabled: true });
    const out = screen.getByRole("button", { name: /sign out/i });
    const content = sheet();
    expect(within(content).getByRole("button", { name: /sign out/i })).toBeTruthy();
    // Last row group: the sign-out group is the content root's final child
    // (destination rows above it, never below; the locked placement).
    const group = out.parentElement as HTMLElement;
    expect(group.lastElementChild).toBe(out);
    expect(content.lastElementChild).toBe(group);
    // Separation by spacing and the muted text token, never a line: the
    // sheets carry no borders anywhere (the no-lines rule).
    expect(group.className).not.toMatch(/border-\S+/);
    expect(out.className).toContain("text-carbon-textMuted");
    // The power glyph is present (muted styling is the classes above; the
    // glyph itself is what makes the row scannable as an action, not a link).
    expect(out.querySelector("svg")).not.toBeNull();
  });

  it("signs out and reloads without asking anywhere in the flow", async () => {
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
      fireEvent.click(within(sheet()).getByRole("link", { name: /vms/i }));
    });
    expect(screen.getByTestId("path-probe").textContent).toBe("/vms");
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
