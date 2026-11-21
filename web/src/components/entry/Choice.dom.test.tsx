// @vitest-environment jsdom
// A picked choice turns into the accent, the way every selection in the app is
// marked, and carries no ring or frame. The other choices keep the plain tile.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Choice, Choices } from "./Choice";

afterEach(cleanup);

function renderChoices(picked: "keep" | "drop") {
  render(
    <Choices label="Where">
      <Choice on={picked === "keep"} title="Keep" text="Leave them where they are." onPick={() => undefined} />
      <Choice on={picked === "drop"} title="Drop" text="Delete them." onPick={() => undefined} />
    </Choices>,
  );
  return { keep: screen.getByRole("radio", { name: /^Keep/ }), drop: screen.getByRole("radio", { name: /^Drop/ }) };
}

const classes = (el: HTMLElement) => el.className.split(/\s+/);

describe("Choice", () => {
  it("marks the picked choice with the accent surface and no ring", () => {
    const { keep } = renderChoices("keep");
    expect(keep.getAttribute("aria-checked")).toBe("true");
    expect(classes(keep)).toEqual(expect.arrayContaining(["glim-active", "bg-accent", "text-accentContrast"]));
    expect(keep.className).not.toMatch(/\b(ring|outline|border)-/);
  });

  it("keeps the other choices on the plain tile", () => {
    const { drop } = renderChoices("keep");
    expect(classes(drop)).toContain("bg-carbon-surface2");
    expect(classes(drop)).not.toContain("bg-accent");
    expect(drop.className).not.toMatch(/\b(ring|outline|border)-/);
  });

  it("draws its text in the accent's ink once picked", () => {
    const { keep } = renderChoices("keep");
    for (const line of [screen.getByText("Keep"), screen.getByText("Leave them where they are.")]) {
      expect(keep.contains(line)).toBe(true);
      expect(line.className).not.toMatch(/\btext-carbon-/);
    }
  });
});
