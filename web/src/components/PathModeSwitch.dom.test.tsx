// @vitest-environment jsdom
// Selector.dom.test.tsx covers the Selector itself; these check how
// PathModeSwitch wires it up. settings, setSettings and save only reach
// OffsiteWizard, which no test here opens, and the compression row, so stubs
// are enough.
import { describe, expect, it, vi } from "vitest";
import { setLabelMode } from "../lib/controls";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach } from "vitest";
import { PathModeSwitch } from "./PathModeSwitch";
import type { Settings } from "../lib/api";

afterEach(() => {
  cleanup();
});

const STUB_SETTINGS = { compression: { containers: "max", vms: "auto" } } as unknown as Settings;
const noopSetSettings = () => {};
let stubSave = vi.fn(async () => true);

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

describe("PathModeSwitch, Selector integration", () => {
  it("renders a real tablist of two tabs, accessible by their plain-language names", () => {
    renderSwitch();
    const list = screen.getByRole("tablist", { name: "Containers path" });
    const tabs = within(list).getAllByRole("tab");
    expect(tabs).toHaveLength(2);
    expect(list.getAttribute("aria-label")).toBe("Containers path");
    expect(screen.getByRole("tab", { name: "Local" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Remote" })).toBeTruthy();
  });

  // The segments keep their accessible names in glyph-only mode. They follow
  // the "buttons" axis, not "tabs", because the pair is a control in a path
  // row beside a text field; Selector derives the axis from its size (see
  // Selector.dom.test.tsx).
  it("follows the buttons label mode instead of being pinned to a glyph", () => {
    setLabelMode("buttons", "textGlyph");
    renderSwitch();
    expect(screen.getByRole("tab", { name: "Local" }).querySelector("[data-sel-label]")).toBeTruthy();
    cleanup();

    setLabelMode("buttons", "glyph");
    renderSwitch();
    for (const tab of within(screen.getByRole("tablist", { name: "Containers path" })).getAllByRole("tab")) {
      expect(tab.querySelector("[data-sel-label]")).toBeNull();
    }
    expect(screen.getByRole("tab", { name: "Local" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Remote" })).toBeTruthy();
    setLabelMode("buttons", "textGlyph");
  });

  it("starts in Local mode for a plain relative path, Remote mode for a restic remote URL", () => {
    renderSwitch("user/appdata/containers");
    expect(screen.getByRole("tab", { name: "Local" }).getAttribute("aria-selected")).toBe("true");
    cleanup();
    renderSwitch("rest:http://host:8000/repo");
    expect(screen.getByRole("tab", { name: "Remote" }).getAttribute("aria-selected")).toBe("true");
  });

  it("shows each segment's tooltip on hover", () => {
    renderSwitch();
    expect(document.querySelector(".glim-bubble")).toBeNull();
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Local" }));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Local path on this host");
    fireEvent.mouseLeave(screen.getByRole("tab", { name: "Local" }));
    fireEvent.mouseEnter(screen.getByRole("tab", { name: "Remote" }));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Remote restic repository");
  });

  it("moves the roving tab stop and switches mode on an arrow key", () => {
    renderSwitch("user/appdata/containers");
    const local = screen.getByRole("tab", { name: "Local" });
    local.focus();
    fireEvent.keyDown(screen.getByRole("tablist", { name: "Containers path" }), { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Remote" }));
    expect((screen.getByRole("tab", { name: "Remote" }) as HTMLElement).tabIndex).toBe(0);
    expect((screen.getByRole("tab", { name: "Local" }) as HTMLElement).tabIndex).toBe(-1);
    // In Remote mode the URL field replaces FolderBrowser's input.
    expect(screen.getByPlaceholderText("s3:bucket/path or rest:http://host:8000/repo")).toBeTruthy();
  });

  it("clears a remote URL when switched to Local", () => {
    const onChange = renderSwitch("rest:http://host:8000/repo");
    fireEvent.click(screen.getByRole("tab", { name: "Local" }));
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("renders the label once, with FolderBrowser's own label turned off", () => {
    renderSwitch("user/appdata/containers");
    expect(screen.getAllByText("Containers path")).toHaveLength(1);
  });

  it("puts the label and the Selector in the same row", () => {
    renderSwitch();
    const label = screen.getByText("Containers path");
    const tablist = screen.getByRole("tablist", { name: "Containers path" });
    expect(label.parentElement).toBe(tablist.parentElement);
  });

  it("shows the domain repository's compression and saves a new choice", async () => {
    stubSave = vi.fn(async () => true);
    renderSwitch();
    const modes = screen.getByRole("tablist", { name: "Compression" });
    expect(within(modes).getByRole("tab", { name: "Maximum" }).getAttribute("aria-selected")).toBe("true");
    await act(async () => {
      fireEvent.click(within(modes).getByRole("tab", { name: "Off" }));
    });
    expect(stubSave).toHaveBeenCalledTimes(1);
    expect(stubSave.mock.calls[0][0]).toEqual({ compression: { containers: "off", vms: "auto" } });
  });
});
