// @vitest-environment jsdom
import { describe, expect, it, vi, afterEach } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { IconTipButton } from "./IconTipButton";

afterEach(() => {
  cleanup();
});

describe("IconTipButton", () => {
  it("uses the tip as its accessible name and sets no native title", () => {
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

  it("opens on focus and closes on blur", () => {
    render(
      <IconTipButton tip="Download recovery kit" onClick={() => {}}>
        <svg aria-hidden="true" />
      </IconTipButton>
    );
    const button = screen.getByRole("button", { name: "Download recovery kit" });
    fireEvent.keyDown(document.body, { key: "Tab" });
    act(() => button.focus());
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Download recovery kit");
    act(() => button.blur());
    expect(document.querySelector(".glim-bubble")).toBeNull();
  });

  it("closes an open tooltip on Escape", () => {
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

  it("does not fire onClick while disabled", () => {
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
