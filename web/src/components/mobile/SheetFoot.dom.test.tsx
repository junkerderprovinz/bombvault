// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Button } from "../Button";
import { en, I18nProvider } from "../../lib/i18n";
import { SheetFoot } from "./SheetFoot";

afterEach(cleanup);

const action = (label: string, onClick = () => undefined) => (
  <Button key={label} label={label} labelKey={null} onClick={onClick} />
);

const foot = () => screen.getByTestId("foot");
const footLabels = () => within(foot()).getAllByRole("button").map((b) => b.textContent);

function renderFoot(node: React.ReactNode) {
  render(
    <I18nProvider>
      <div data-testid="foot">{node}</div>
    </I18nProvider>,
  );
}

describe("SheetFoot", () => {
  it("shows three buttons as they are", () => {
    renderFoot(
      <SheetFoot close={action("Close")}>
        {action("Open item")}
        {action("Restore")}
      </SheetFoot>,
    );
    expect(footLabels()).toEqual(["Open item", "Restore", "Close"]);
  });

  it("keeps the window's close and the last action, and folds the rest behind More", () => {
    renderFoot(
      <SheetFoot close={action("Close")}>
        <>
          {action("Compare")}
          {false}
          {action("Acknowledge")}
        </>
        {action("Restore")}
      </SheetFoot>,
    );
    expect(footLabels()).toEqual([en["nav.more"], "Close", "Restore"]);
    expect(screen.queryByText("Compare")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: en["nav.more"] }));
    const sheet = screen.getByRole("dialog");
    expect(within(sheet).getByRole("button", { name: "Compare" })).toBeTruthy();
    expect(within(sheet).getByRole("button", { name: "Acknowledge" })).toBeTruthy();
  });

  it("takes the first button as the way out when the window brings no close of its own", () => {
    renderFoot(
      <SheetFoot>
        {action("Cancel")}
        {action("Test")}
        {action("Back")}
        {action("Save")}
      </SheetFoot>,
    );
    expect(footLabels()).toEqual([en["nav.more"], "Cancel", "Save"]);
  });

  it("closes the More sheet once an action in it is pressed", () => {
    const compare = vi.fn();
    renderFoot(
      <SheetFoot close={action("Close")}>
        {action("Compare", compare)}
        {action("Acknowledge")}
        {action("Restore")}
      </SheetFoot>,
    );
    fireEvent.click(screen.getByRole("button", { name: en["nav.more"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Compare" }));
    expect(compare).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
