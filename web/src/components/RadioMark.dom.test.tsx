// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { RadioMark } from "./RadioMark";

afterEach(cleanup);

describe("the mark of a choice of one", () => {
  it("is a ring that takes no corner from the shape, so it never reads as a box", () => {
    const { container } = render(<RadioMark on={false} />);
    const ring = container.firstElementChild!;
    expect(ring.className).toContain("rounded-full");
    expect(ring.className).not.toContain("rounded-pill");
    expect(ring.childElementCount).toBe(0);
  });

  it("holds a round dot once the choice is made", () => {
    const { container } = render(<RadioMark on />);
    expect(container.firstElementChild!.firstElementChild!.className).toContain("rounded-full");
  });
});
