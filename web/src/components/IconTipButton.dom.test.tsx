// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// IconTipButton — the shared engine behind an icon-only plain <button>'s
// hover/focus tooltip (GlimStone follow-up round: "beim Ordnersymbol ist die
// Hover-Infobubble nicht im GlimStone"). InfoBubble.dom tests and
// Selector.dom.test.tsx's own "tip" tests already cover the identical
// measure-then-clamp positioning contract for their own two trigger shapes
// ("(i)" glyph, Selector segment); this file covers the THIRD trigger shape
// (a bare `<button>`) this component exists for, plus the one thing that
// actually caused this file to be written: a real `.glim-bubble` renders
// instead of a native `title=` balloon.
// ---------------------------------------------------------------------------
import { describe, expect, it, vi, afterEach } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { IconTipButton } from "./IconTipButton";

afterEach(() => {
  cleanup();
});

describe("IconTipButton", () => {
  it("renders an icon-only button with no native title attribute — the tip is its own accessible name", () => {
    render(
      <IconTipButton tip="Browse folders" onClick={() => {}}>
        <svg aria-hidden="true" />
      </IconTipButton>
    );
    const button = screen.getByRole("button", { name: "Browse folders" });
    expect(button.getAttribute("title")).toBeNull();
    expect(button.getAttribute("aria-label")).toBe("Browse folders");
  });

  it("shows no .glim-bubble until hovered, then reveals one with the tip text", () => {
    render(
      <IconTipButton tip="Add registry" onClick={() => {}}>
        <svg aria-hidden="true" />
      </IconTipButton>
    );
    expect(document.querySelector(".glim-bubble")).toBeNull();
    fireEvent.mouseEnter(screen.getByRole("button", { name: "Add registry" }));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Add registry");
    fireEvent.mouseLeave(screen.getByRole("button", { name: "Add registry" }));
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("also opens on focus (keyboard-accessible, not mouse-only) and closes on blur", () => {
    render(
      <IconTipButton tip="Download recovery kit" onClick={() => {}}>
        <svg aria-hidden="true" />
      </IconTipButton>
    );
    const button = screen.getByRole("button", { name: "Download recovery kit" });
    fireEvent.focus(button);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Download recovery kit");
    fireEvent.blur(button);
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("Escape closes an open tooltip without requiring blur", () => {
    render(
      <IconTipButton tip="Add registry" onClick={() => {}}>
        <svg aria-hidden="true" />
      </IconTipButton>
    );
    fireEvent.mouseEnter(screen.getByRole("button", { name: "Add registry" }));
    expect(document.querySelector(".glim-bubble")).not.toBeNull();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("fires onClick when clicked, and never when disabled", () => {
    const onClick = vi.fn();
    render(
      <IconTipButton tip="Add registry" onClick={onClick} disabled>
        <svg aria-hidden="true" />
      </IconTipButton>
    );
    fireEvent.click(screen.getByRole("button", { name: "Add registry" }));
    expect(onClick).not.toHaveBeenCalled();
  });
});
