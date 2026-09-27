// @vitest-environment jsdom
// The carried row follows the pointer and the others make room as it passes;
// a drop writes the whole new order once, a drop where it started writes
// nothing, and Escape puts everything back. jsdom lays nothing out, so each
// row reports the box a 40px-high column would give it.
import { act, cleanup, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { LIFT, useReorder } from "./dragLift";

const ROW = 40;

function List({ onReorder }: { onReorder: (ids: string[]) => void }) {
  const [ids, setIds] = useState(["a", "b", "c"]);
  const list = useRef<HTMLOListElement>(null);
  const drag = useReorder({
    ids,
    container: list,
    attr: "data-id",
    axis: "y",
    arm: "move",
    enabled: true,
    onReorder: (next) => {
      setIds(next);
      onReorder(next);
    },
  });
  return (
    <ol ref={list} data-testid="list" className={drag.held !== null ? "glim-drag-armed" : ""}>
      {drag.order.map((id) => (
        <li key={id} data-id={id} className={drag.look(id)} onPointerDown={(e) => drag.press(e, id)}>
          {id}
        </li>
      ))}
    </ol>
  );
}

beforeEach(() => {
  // Each row's box follows its place among its siblings, as a column would.
  const place = (el: HTMLElement) => Array.prototype.indexOf.call(el.parentElement?.children ?? [], el);
  vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockImplementation(function (this: HTMLElement) {
    return this.dataset["id"] ? place(this) * ROW : 0;
  });
  vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockReturnValue(0);
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(300);
  vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(ROW - 4);
  // Only what the gesture reads.
  vi.spyOn(window, "getComputedStyle").mockImplementation(
    (el: Element) =>
      ({
        translate: (el as HTMLElement).style.getPropertyValue("translate") || "none",
        transitionDuration: "0s",
        direction: "ltr",
        getPropertyValue: (name: string) => (name === "--drag-lift-scale" ? "1.03" : ""),
      }) as unknown as CSSStyleDeclaration
  );
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function pointer(type: string, target: EventTarget, y: number) {
  const event = new MouseEvent(type, { bubbles: true, clientX: 10, clientY: y, button: 0 });
  Object.assign(event, { pointerId: 1, pointerType: "mouse" });
  act(() => {
    target.dispatchEvent(event);
  });
}

function order(): string[] {
  return Array.from(screen.getByTestId("list").children).map((li) => li.textContent ?? "");
}

it("lifts the carried row and lets the others make room as it passes", () => {
  const onReorder = vi.fn();
  render(<List onReorder={onReorder} />);

  pointer("pointerdown", screen.getByText("a"), 10);
  pointer("pointermove", document, 20);
  expect(screen.getByText("a").className).toBe(LIFT);
  expect(screen.getByTestId("list").className).toBe("glim-drag-armed");

  // Past the middle of b: a now stands after it.
  pointer("pointermove", document, 10 + ROW + 5);
  expect(order()).toEqual(["b", "a", "c"]);
  expect(onReorder).not.toHaveBeenCalled();

  pointer("pointerup", document, 10 + ROW + 5);
  expect(onReorder).toHaveBeenCalledTimes(1);
  expect(onReorder).toHaveBeenCalledWith(["b", "a", "c"]);
});

it("writes nothing for a drop where the row started", () => {
  const onReorder = vi.fn();
  render(<List onReorder={onReorder} />);

  pointer("pointerdown", screen.getByText("b"), 50);
  pointer("pointermove", document, 58);
  pointer("pointerup", document, 58);

  expect(onReorder).not.toHaveBeenCalled();
  expect(order()).toEqual(["a", "b", "c"]);
});

it("puts every row back on Escape", () => {
  const onReorder = vi.fn();
  render(<List onReorder={onReorder} />);

  pointer("pointerdown", screen.getByText("a"), 10);
  pointer("pointermove", document, 20);
  pointer("pointermove", document, 10 + 2 * ROW + 5);
  expect(order()).toEqual(["b", "c", "a"]);

  act(() => {
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
  });
  pointer("pointerup", document, 10 + 2 * ROW + 5);

  expect(onReorder).not.toHaveBeenCalled();
  expect(order()).toEqual(["a", "b", "c"]);
});
