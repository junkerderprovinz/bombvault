// @vitest-environment jsdom
// The README button is the MCP card's client picker, so its accessible name
// has to carry both lines and the lit unit has to be one group.
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReadmeButton } from "./ReadmeButton";

const indexCss = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

afterEach(cleanup);

describe("ReadmeButton", () => {
  it("is one button named by both lines that lights up as one unit", () => {
    const onClick = vi.fn();
    const { container } = render(
      <ReadmeButton tile="glim-tile-house" parts={[{ name: "Cursor", sub: "Editor", onClick }]} mark={<svg />} />
    );
    const button = screen.getByRole("button", { name: "Cursor Editor" });
    fireEvent.click(button);
    expect(onClick).toHaveBeenCalledOnce();

    const unit = container.firstElementChild!;
    expect(unit.className).toContain("group");
    expect(unit.className).toContain("glim-tile-house");
    expect(button.className).toContain("glim-brand-tile");
    expect(button.querySelector(".glim-readme-btn-mark")?.getAttribute("aria-hidden")).toBe("true");
    expect(unit.querySelector(".glim-readme-btn-sheen")).not.toBeNull();
  });

  it("says a part without a target is still to come", () => {
    render(<ReadmeButton tile="glim-tile-house" parts={[{ name: "Zed" }]} soonLabel="Soon" />);
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByLabelText("Zed Soon").getAttribute("aria-disabled")).toBe("true");
  });

  it("keeps the (i) outside the button", () => {
    render(<ReadmeButton tile="glim-tile-house" parts={[{ name: "Jan", onClick: () => {} }]} hint="Local models" />);
    const button = screen.getByRole("button", { name: "Jan" });
    expect(button.querySelector("[aria-label='Local models']")).toBeNull();
    expect(screen.getByLabelText("Local models")).toBeTruthy();
  });
});

describe("the README button's styles", () => {
  it("puts the mark 0.875rem from the edge and the text at 3.3125rem", () => {
    const mark = /\.glim-readme-btn-mark \{([^}]*)\}/.exec(indexCss)?.[1] ?? "";
    const text = /\.glim-readme-btn-text \{([^}]*)\}/.exec(indexCss)?.[1] ?? "";
    expect(mark).toMatch(/inset-inline-start:\s*0\.875rem/);
    expect(text).toMatch(/inset-inline:\s*3\.3125rem 0\.625rem/);
  });

  it("lights every tile of a unit under the pointer", () => {
    expect(indexCss).toMatch(/\.group:hover > \.glim-brand-tile,\s*\.glim-brand-tile:hover \{/);
  });
});
