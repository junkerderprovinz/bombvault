// @vitest-environment jsdom
// A question asked from inside a window is a second window on top of it, and
// the keys belong to the one on top.
import { useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InfoBubble } from "../components/InfoBubble";
import { I18nProvider, en } from "./i18n";
import { useConfirm, useDialogKeys } from "./useConfirm";

function Window({
  onClose,
  onAnswer,
  extra,
  hint,
}: {
  onClose: () => void;
  onAnswer: (ok: boolean) => void;
  extra?: ReactNode;
  hint?: string;
}) {
  const cardRef = useRef<HTMLDivElement>(null);
  const { confirm, confirmDialog } = useConfirm();
  useDialogKeys(true, cardRef, onClose);
  return createPortal(
    <div ref={cardRef} role="dialog" aria-modal="true" aria-label="Add a place">
      {hint && <InfoBubble tip={hint} />}
      <button type="button" onClick={() => void confirm("Use it?", { extra }).then(onAnswer)}>
        Accept
      </button>
      <button type="button">Back</button>
      {confirmDialog}
    </div>,
    document.body
  );
}

const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

// jsdom moves no focus on Tab, so a Tab no trap took moves on in document
// order, the way a browser does.
function tab(shift = false) {
  const from = document.activeElement as HTMLElement;
  if (!fireEvent.keyDown(from, { key: "Tab", shiftKey: shift })) return;
  const all = [...document.querySelectorAll<HTMLElement>(FOCUSABLE)];
  const at = all.indexOf(from);
  (shift ? all[at - 1] : all[at + 1])?.focus();
}

async function ask(extra?: ReactNode) {
  const onClose = vi.fn();
  const onAnswer = vi.fn();
  render(
    <I18nProvider>
      <Window onClose={onClose} onAnswer={onAnswer} extra={extra} />
    </I18nProvider>
  );
  // A real click focuses the button, and the question hands focus back to it.
  const accept = screen.getByRole("button", { name: "Accept" });
  accept.focus();
  await act(async () => {
    fireEvent.click(accept);
  });
  return { onClose, onAnswer, question: screen.getByRole("dialog", { name: en["confirmDialog.title"] }) };
}

const SWITCHES = (
  <>
    <input type="checkbox" aria-label="Leave these out here too" />
    <input type="checkbox" aria-label="Leave the default out too" />
  </>
);

describe("a question over a window", () => {
  afterEach(cleanup);

  it("answers Escape alone and leaves the window open", async () => {
    const { onClose, onAnswer } = await ask();
    await act(async () => {
      fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    });
    expect(onAnswer).toHaveBeenCalledWith(false);
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog", { name: en["confirmDialog.title"] })).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Accept" }));

    fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("takes the keys past a card that says it is not modal", async () => {
    const { onAnswer } = await ask();
    const panel = document.createElement("div");
    panel.setAttribute("aria-modal", "false");
    document.body.append(panel);
    try {
      await act(async () => {
        fireEvent.keyDown(document.activeElement!, { key: "Escape" });
      });
      expect(onAnswer).toHaveBeenCalledWith(false);
    } finally {
      panel.remove();
    }
  });

  it("leaves alone an Escape that a window above it has answered", async () => {
    const { onAnswer } = await ask();
    const above = (e: KeyboardEvent) => e.preventDefault();
    document.addEventListener("keydown", above, true);
    try {
      await act(async () => {
        fireEvent.keyDown(document.activeElement!, { key: "Escape" });
      });
      expect(onAnswer).not.toHaveBeenCalled();
      expect(screen.getByRole("dialog", { name: en["confirmDialog.title"] })).toBeTruthy();
    } finally {
      document.removeEventListener("keydown", above, true);
    }
  });

  it.each([
    ["without switches", undefined],
    ["with two switches", SWITCHES],
  ])("walks every control of the question on Tab and Shift+Tab (%s)", async (_, extra) => {
    const { question } = await ask(extra);
    const controls = [...question.querySelectorAll<HTMLElement>(FOCUSABLE)];
    const cancel = within(question).getByRole("button", { name: en["common.cancel"] });
    expect(document.activeElement).toBe(cancel);

    const start = controls.indexOf(cancel);
    const forward: HTMLElement[] = [];
    for (let i = 0; i < controls.length; i++) {
      tab();
      forward.push(document.activeElement as HTMLElement);
    }
    expect(forward).toEqual(controls.map((_, i) => controls[(start + 1 + i) % controls.length]));

    const back: HTMLElement[] = [];
    for (let i = 0; i < controls.length; i++) {
      tab(true);
      back.push(document.activeElement as HTMLElement);
    }
    expect(back).toEqual(controls.map((_, i) => controls[(start - 1 - i + 2 * controls.length) % controls.length]));
  });
});

describe("an info bubble open in a window", () => {
  afterEach(cleanup);

  const HINT = "Copies here stay on this server.";

  it("takes the first Escape, and the window the next", () => {
    const onClose = vi.fn();
    render(
      <I18nProvider>
        <Window onClose={onClose} onAnswer={vi.fn()} hint={HINT} />
      </I18nProvider>
    );
    fireEvent.mouseEnter(screen.getByLabelText(HINT));
    expect(screen.getByRole("tooltip").textContent).toBe(HINT);

    fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("takes the first Escape in a question, and the question the next", async () => {
    const { onClose, onAnswer, question } = await ask(<InfoBubble tip={HINT} />);
    fireEvent.mouseEnter(within(question).getByLabelText(HINT));

    await act(async () => {
      fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    });
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(onAnswer).not.toHaveBeenCalled();

    await act(async () => {
      fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    });
    expect(onAnswer).toHaveBeenCalledWith(false);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("keeps the first Escape from a window that listens for it on its own", () => {
    const own = vi.fn();
    document.addEventListener("keydown", own, true);
    try {
      render(<InfoBubble tip={HINT} />);
      fireEvent.mouseEnter(screen.getByLabelText(HINT));
      fireEvent.keyDown(document.body, { key: "Escape" });
      expect(screen.queryByRole("tooltip")).toBeNull();
      expect(own).not.toHaveBeenCalled();

      fireEvent.keyDown(document.body, { key: "Escape" });
      expect(own).toHaveBeenCalledTimes(1);
    } finally {
      document.removeEventListener("keydown", own, true);
    }
  });
});
