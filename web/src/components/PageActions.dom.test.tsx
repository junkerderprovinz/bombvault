// @vitest-environment jsdom
// jsdom lays nothing out, so that the corner sticks is pinned by its classes
// and by the rule in index.css that lets the page fill the scroller.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { setLabelMode } from "../lib/controls";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import { PageAction, PageActions } from "./PageActions";

// useIsDesktop keeps the first MediaQueryList it gets, so the stub answers
// from a variable instead of being swapped per test.
let desktop = true;
window.matchMedia = ((query: string) => ({
  get matches() {
    return query === DESKTOP_QUERY ? desktop : false;
  },
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

function renderCorner(onAdd = () => {}) {
  render(
    <PageActions>
      <PageAction label="Discover" glyph={<svg />} onClick={() => {}} />
      <PageAction primary label="Add" glyph={<svg />} onClick={onAdd} />
    </PageActions>
  );
  return screen.getByTestId("page-actions");
}

beforeEach(() => {
  desktop = true;
  localStorage.clear();
  setLabelMode("buttons", "textGlyph");
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("PageActions", () => {
  it("sticks to the bottom of the scroller in the flow of the page, never fixed", () => {
    const corner = renderCorner();
    expect(corner.className).toContain("sticky");
    expect(corner.className).toContain("bottom-0");
    expect(corner.className).not.toMatch(/(^|\s)fixed(\s|$)/);
  });

  it("stands at the end of the content, pushed to the bottom of a short page", () => {
    const corner = renderCorner();
    expect(corner.className).toContain("justify-end");
    expect(corner.className).toContain("mt-auto");
    expect(css).toMatch(/\.glim-page-enter > :has\(> \.glim-page-actions\)\s*\{\s*flex-grow:\s*1;/);
  });

  it("tells the toasts how far up it reaches, and takes that back when it leaves", () => {
    const { unmount } = render(
      <main>
        <PageActions>
          <PageAction label="Add" glyph={<svg />} onClick={() => {}} />
        </PageActions>
      </main>
    );
    expect(document.documentElement.style.getPropertyValue("--page-actions-clearance")).toMatch(/^\d+px$/);
    expect(css).toMatch(/\.glim-toasts\s*\{\s*padding-bottom:[^}]*var\(--page-actions-clearance, 0px\)/);
    unmount();
    expect(document.documentElement.style.getPropertyValue("--page-actions-clearance")).toBe("");
  });

  it("lets a press between its buttons reach the row underneath", () => {
    const corner = renderCorner();
    expect(corner.className).toContain("pointer-events-none");
    expect(corner.firstElementChild?.className).toContain("pointer-events-auto");
  });

  it("fills only the action the page exists for", () => {
    renderCorner();
    expect(screen.getByRole("button", { name: "Add" }).className).toContain("bg-accent");
    expect(screen.getByRole("button", { name: "Discover" }).className).not.toContain("bg-accent");
  });

  it("runs the action", () => {
    const onAdd = vi.fn();
    renderCorner(onAdd);
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(onAdd).toHaveBeenCalledTimes(1);
  });

  it("shows words beside the glyph on the desktop", () => {
    renderCorner();
    expect(screen.getByRole("button", { name: "Add" }).querySelector(".glim-btn-label")?.textContent).toBe("Add");
  });

  it("shows the glyph alone below the desktop width, whatever the label mode", () => {
    desktop = false;
    setLabelMode("buttons", "text");
    renderCorner();
    const add = screen.getByRole("button", { name: "Add" });
    expect(add.querySelector(".glim-btn-label")).toBeNull();
    expect([...add.classList]).toEqual(expect.arrayContaining(["glim-btn-icon", "glim-btn-key"]));
  });
});
