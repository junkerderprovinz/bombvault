// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Checkbox } from "./Checkbox";

afterEach(cleanup);

it("flips on Space and keeps Enter from flipping it, as a native checkbox does", () => {
  const onChange = vi.fn();
  render(<Checkbox checked={false} onChange={onChange} label="Don't ask again" />);
  const box = screen.getByRole("checkbox", { name: "Don't ask again" });
  // A button clicks on Enter by default, so the box has to refuse it.
  expect(fireEvent.keyDown(box, { key: "Enter" })).toBe(false);
  expect(fireEvent.keyDown(box, { key: " " })).toBe(true);
  fireEvent.click(box);
  expect(onChange).toHaveBeenCalledWith(true);
});
