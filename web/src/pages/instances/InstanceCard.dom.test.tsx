// @vitest-environment jsdom
import { afterEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { InstanceCard } from "./InstanceCard";

afterEach(cleanup);

const nameColumn = () => screen.getByText("Tower").closest("[data-instance-text]");

it("keeps the actions beside the mark when nothing stands under the text, so the card stays as tall as its mark", () => {
  render(<InstanceCard cardKey="t" name="Tower" eyebrow="Container" actions={<button type="button">Remove</button>} index={0} />);
  expect(nameColumn()?.contains(screen.getByRole("button", { name: "Remove" }))).toBe(true);
});

it("puts the actions along the foot under what the card holds", () => {
  render(
    <InstanceCard cardKey="t" name="Tower" actions={<button type="button">Remove</button>} index={0}>
      <p>roles</p>
    </InstanceCard>,
  );
  expect(nameColumn()?.contains(screen.getByRole("button", { name: "Remove" }))).toBe(false);
});
