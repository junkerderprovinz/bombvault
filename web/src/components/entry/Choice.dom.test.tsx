// @vitest-environment jsdom
// A picked choice turns into the accent, the way every selection in the app is
// marked, and carries no ring, frame or dot. The other choices keep the plain
// tile. The tiles stay a radio group: one tab stop, and the arrow keys pick.
import { useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Choice, Choices } from "./Choice";

afterEach(cleanup);

const WAYS = ["Keep", "Move", "Drop"] as const;

function Ways({ first = "Keep" }: { first?: (typeof WAYS)[number] }) {
  const [picked, setPicked] = useState<string>(first);
  return (
    <Choices label="Where">
      {WAYS.map((way) => (
        <Choice
          key={way}
          on={picked === way}
          title={way}
          text={`${way} them.`}
          picture={<svg data-testid={`picture-${way}`} aria-hidden="true" />}
          onPick={() => setPicked(way)}
        />
      ))}
    </Choices>
  );
}

const radio = (way: string) => screen.getByRole("radio", { name: new RegExp(`^${way}`) });
const classes = (el: HTMLElement) => el.className.split(/\s+/);

describe("Choice", () => {
  it("marks the picked choice with the accent surface and no ring", () => {
    render(<Ways />);
    const keep = radio("Keep");
    expect(keep.getAttribute("aria-checked")).toBe("true");
    expect(classes(keep)).toEqual(expect.arrayContaining(["glim-active", "bg-accent", "text-accentContrast"]));
    expect(keep.className).not.toMatch(/\b(ring|outline|border)-/);
  });

  it("keeps the other choices on the plain tile", () => {
    render(<Ways />);
    const drop = radio("Drop");
    expect(drop.getAttribute("aria-checked")).toBe("false");
    expect(classes(drop)).toContain("bg-carbon-surface2");
    expect(classes(drop)).not.toContain("bg-accent");
    expect(drop.className).not.toMatch(/\b(ring|outline|border)-/);
  });

  it("draws its text in the accent's ink once picked", () => {
    render(<Ways />);
    for (const line of [screen.getByText("Keep"), screen.getByText("Keep them.")]) {
      expect(radio("Keep").contains(line)).toBe(true);
      expect(line.className).not.toMatch(/\btext-carbon-/);
    }
  });

  it("shows no radio dot, since the surface says what is picked", () => {
    render(<Ways />);
    for (const way of WAYS) {
      expect(radio(way).querySelector(".rounded-full")).toBeNull();
      expect(radio(way).querySelector('[class*="border-"]')).toBeNull();
    }
  });

  it("puts the picture before the text", () => {
    render(<Ways />);
    const picture = screen.getByTestId("picture-Move");
    expect(picture.compareDocumentPosition(screen.getByText("Move")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("is one tab stop, on the picked choice", () => {
    render(<Ways first="Move" />);
    expect(WAYS.map((way) => radio(way).tabIndex)).toEqual([-1, 0, -1]);
  });

  it("moves the pick with the arrow keys and wraps at the ends", () => {
    render(<Ways />);
    radio("Keep").focus();
    fireEvent.keyDown(radio("Keep"), { key: "ArrowDown" });
    expect(radio("Move").getAttribute("aria-checked")).toBe("true");
    expect(document.activeElement).toBe(radio("Move"));
    fireEvent.keyDown(radio("Move"), { key: "ArrowRight" });
    expect(radio("Drop").getAttribute("aria-checked")).toBe("true");
    fireEvent.keyDown(radio("Drop"), { key: "ArrowDown" });
    expect(radio("Keep").getAttribute("aria-checked")).toBe("true");
    fireEvent.keyDown(radio("Keep"), { key: "ArrowUp" });
    expect(radio("Drop").getAttribute("aria-checked")).toBe("true");
    expect(document.activeElement).toBe(radio("Drop"));
  });
});
