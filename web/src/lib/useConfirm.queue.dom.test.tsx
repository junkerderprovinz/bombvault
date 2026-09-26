// @vitest-environment jsdom
// A question asked while another is open waits its turn, and each answer
// reaches the caller that asked.
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider, en } from "./i18n";
import { useConfirm } from "./useConfirm";

function Asker({ onAnswer }: { onAnswer: (question: string, ok: boolean) => void }) {
  const { confirm, confirmDialog } = useConfirm();
  const ask = (question: string) => void confirm(question).then((ok) => onAnswer(question, ok));
  return (
    <>
      <button type="button" onClick={() => ask("Keep fewer?")}>
        retention
      </button>
      <button type="button" onClick={() => ask("Allow deletes?")}>
        append-only
      </button>
      {confirmDialog}
    </>
  );
}

async function askBoth() {
  const onAnswer = vi.fn();
  render(
    <I18nProvider>
      <Asker onAnswer={onAnswer} />
    </I18nProvider>
  );
  const trigger = screen.getByRole("button", { name: "append-only" });
  trigger.focus();
  await act(async () => {
    fireEvent.click(trigger);
  });
  // The page is covered by now; this stands in for a save timer that asks.
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "retention" }));
  });
  return { onAnswer, trigger };
}

// A real click focuses the button it lands on; jsdom leaves that to the test.
async function answer(name: string) {
  const button = within(screen.getByRole("dialog")).getByRole("button", { name });
  button.focus();
  await act(async () => {
    fireEvent.click(button);
  });
}

describe("useConfirm with two questions", () => {
  afterEach(cleanup);

  it("shows the second question once the first is answered and settles each with its own answer", async () => {
    const { onAnswer } = await askBoth();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(screen.getByText("Allow deletes?")).toBeTruthy();

    await answer(en["common.cancel"]);
    expect(onAnswer.mock.calls).toEqual([["Allow deletes?", false]]);
    expect(screen.getByText("Keep fewer?")).toBeTruthy();

    await answer(en["common.confirm"]);
    expect(onAnswer.mock.calls).toEqual([
      ["Allow deletes?", false],
      ["Keep fewer?", true],
    ]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("starts the second question on Cancel and hands focus back once both are answered", async () => {
    const { trigger } = await askBoth();
    await answer(en["common.confirm"]);
    expect(document.activeElement).toBe(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.cancel"] }));

    await answer(en["common.cancel"]);
    expect(document.activeElement).toBe(trigger);
  });
});
