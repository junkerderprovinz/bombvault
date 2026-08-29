// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// PathModeSwitch — icon-only Local/Remote control (GlimStone follow-up round,
// Paths & Storage tab rework, points 2/5). Selector.dom.test.tsx already
// covers the generic iconOnly/tip/roving-tabindex mechanics in isolation;
// this file covers the INTEGRATION — that PathModeSwitch actually wires those
// features up correctly (real Selector, real icons, real mode switch), and
// that the label/Selector share one row with FolderBrowser's own label
// suppressed rather than duplicated.
//
// `settings`/`setSettings`/`save` are only read by OffsiteWizard, which this
// suite never opens (no test here sets a remote value AND clicks the wizard
// toggle) — a minimal stub cast is enough, matching this repo's own
// precedent for tests that construct a partial Settings for a prop the code
// path under test never actually touches.
// ---------------------------------------------------------------------------
import { describe, expect, it, vi } from "vitest";
import { setLabelMode } from "../lib/controls";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach } from "vitest";
import { PathModeSwitch } from "./PathModeSwitch";
import type { Settings } from "../lib/api";

afterEach(() => {
  cleanup();
});

const STUB_SETTINGS = {} as unknown as Settings;
const noopSetSettings = () => {};
const stubSave = async () => true;

function renderSwitch(value = "", onChange = vi.fn()) {
  render(
    <PathModeSwitch
      label="Containers path"
      domain="containers"
      value={value}
      hostMountRoot="/mnt/user"
      onChange={onChange}
      settings={STUB_SETTINGS}
      setSettings={noopSetSettings}
      save={stubSave}
    />
  );
  return onChange;
}

describe("PathModeSwitch — icon-only Selector integration", () => {
  it("renders a real tablist of two tabs, accessible by their plain-language names", () => {
    renderSwitch();
    const list = screen.getByRole("tablist");
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(2);
    expect(list.getAttribute("aria-label")).toBe("Containers path");
    expect(screen.getByRole("tab", { name: "Local" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Remote" })).toBeTruthy();
  });

  // These two segments used to be pinned to a square glyph with `iconOnly`, in
  // every mode, which is the square badge jdp asked to abolish: neither the
  // label mode nor the width system could reach them. They now follow the
  // "tabs" axis like every other selector, and the accessible name survives
  // the glyph-only mode, which is the part that must never regress.
  it("follows the tabs label mode instead of being pinned to a glyph", () => {
    setLabelMode("tabs", "textGlyph");
    renderSwitch();
    expect(screen.getByRole("tab", { name: "Local" }).querySelector("span.truncate")).toBeTruthy();
    cleanup();

    setLabelMode("tabs", "glyph");
    renderSwitch();
    for (const tab of screen.getAllByRole("tab")) {
      expect(tab.querySelector("span.truncate")).toBeNull();
    }
    expect(screen.getByRole("tab", { name: "Local" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Remote" })).toBeTruthy();
    setLabelMode("tabs", "textGlyph");
  });

  it("starts in Local mode for a plain relative path, Remote mode for a restic remote URL", () => {
    renderSwitch("user/appdata/containers");
    expect(screen.getByRole("tab", { name: "Local" }).getAttribute("aria-selected")).toBe("true");
    cleanup();
    renderSwitch("rest:http://host:8000/repo");
    expect(screen.getByRole("tab", { name: "Remote" }).getAttribute("aria-selected")).toBe("true");
  });

  it("hovering the Local/Remote icons reveals the InfoBubble-style tooltip explaining each", () => {
    renderSwitch();
    expect(document.querySelector(".glim-bubble")).toBeNull();
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Local" }));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Local path on this host");
    fireEvent.mouseLeave(screen.getByRole("tab", { name: "Local" }));
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Remote" }));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Remote restic repository");
  });

  it("arrow-key navigation moves the roving tab stop AND switches mode (select=\"one\" activates on move) — proves this is a real Selector, not a hand-rolled pair", () => {
    renderSwitch("user/appdata/containers");
    const local = screen.getByRole("tab", { name: "Local" });
    local.focus();
    fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Remote" }));
    expect((screen.getByRole("tab", { name: "Remote" }) as HTMLElement).tabIndex).toBe(0);
    expect((screen.getByRole("tab", { name: "Local" }) as HTMLElement).tabIndex).toBe(-1);
    // Now in Remote mode: the URL field replaces FolderBrowser's input.
    expect(screen.getByPlaceholderText("s3:bucket/path or rest:http://host:8000/repo")).toBeTruthy();
  });

  it("clicking Remote then Local clears a remote-shaped value (switchToLocal's existing behaviour, unchanged)", () => {
    const onChange = renderSwitch("rest:http://host:8000/repo");
    fireEvent.click(screen.getByRole("tab", { name: "Local" }));
    // Already local-shaped active state at this value would keep it — but
    // this value IS remote-shaped, so switching back to Local clears it.
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("renders exactly one copy of the label — FolderBrowser's own label row is suppressed, not duplicated", () => {
    renderSwitch("user/appdata/containers");
    expect(screen.getAllByText("Containers path")).toHaveLength(1);
  });

  it("the label and the Selector sit in the same row (label at the start, Selector at the end of one flex container)", () => {
    renderSwitch();
    const label = screen.getByText("Containers path");
    const tablist = screen.getByRole("tablist");
    expect(label.parentElement).toBe(tablist.parentElement);
  });
});
