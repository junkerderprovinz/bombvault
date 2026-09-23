// @vitest-environment jsdom
// The dialog's commit wears a glyph and the accent, whether the caller names
// the trigger's key or leaves it to the plain confirm.
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import type { TranslationKey } from "./i18n";
import { useConfirm } from "./useConfirm";

function Asker({ confirmKey }: { confirmKey?: TranslationKey }) {
  const { confirm, confirmDialog } = useConfirm();
  return (
    <>
      <button onClick={() => void confirm("Sure?", confirmKey ? { confirmKey } : undefined)}>ask</button>
      {confirmDialog}
    </>
  );
}

afterEach(cleanup);

it.each([undefined, "vms.removeEntry" as TranslationKey])("gives the commit a glyph and the accent (%s)", async (key) => {
  render(<Asker confirmKey={key} />);
  await act(async () => {
    fireEvent.click(screen.getByText("ask"));
  });
  const buttons = within(screen.getByRole("dialog")).getAllByRole("button");
  const commit = buttons[buttons.length - 1];
  expect(commit.querySelector(".glim-btn-glyph")).not.toBeNull();
  expect(commit.className).toContain("bg-accent");
});
