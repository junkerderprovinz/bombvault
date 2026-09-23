// @vitest-environment jsdom
// A wheel notch steps a picker and is clamped rather than wrapped, so one notch
// too many cannot land a value from the other end of the list. While the
// pointer is on the control the handler is the scroll, and the page underneath
// stays put.
import { afterEach, expect, it } from "vitest";
import { enableSelectScrollHere, enableWheelStep, stepIndex } from "./selectScroll";

const detachers: Array<() => void> = [];

afterEach(() => {
  while (detachers.length) detachers.pop()?.();
  document.body.innerHTML = "";
});

function wheel(deltaY: number): WheelEvent {
  return new WheelEvent("wheel", { deltaY, bubbles: true, cancelable: true });
}

it("clamps at both ends instead of wrapping", () => {
  expect(stepIndex(5, 0, 1)).toBe(1);
  expect(stepIndex(5, 4, 1)).toBe(4); // not back to 0
  expect(stepIndex(5, 0, -1)).toBe(0); // not round to 4
  // A list of one has nowhere to go, and a wheel notch on it is not an error.
  expect(stepIndex(1, 0, 1)).toBe(0);
  expect(stepIndex(0, 0, -1)).toBe(0);
});

it("steps a trigger and swallows the page scroll", () => {
  const trigger = document.createElement("button");
  document.body.append(trigger);
  const seen: number[] = [];
  detachers.push(enableWheelStep(trigger, (delta) => seen.push(delta)));

  const down = wheel(120);
  trigger.dispatchEvent(down);
  const up = wheel(-120);
  trigger.dispatchEvent(up);

  expect(seen).toEqual([1, -1]);
  // Otherwise the page scrolls out from under the pointer while the value
  // changes.
  expect(down.defaultPrevented).toBe(true);
  expect(up.defaultPrevented).toBe(true);
});

it("ignores a horizontal wheel", () => {
  const trigger = document.createElement("button");
  document.body.append(trigger);
  const seen: number[] = [];
  detachers.push(enableWheelStep(trigger, (delta) => seen.push(delta)));
  trigger.dispatchEvent(wheel(0));
  expect(seen).toEqual([]);
});

function makeSelect(options: string[], disabled = false): HTMLSelectElement {
  const select = document.createElement("select");
  select.disabled = disabled;
  for (const value of options) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    select.append(option);
  }
  document.body.append(select);
  return select;
}

it("steps a native select rendered after the listener was attached", () => {
  // Selects come and go with every route, which is why the listener is
  // delegated rather than attached per element at boot.
  detachers.push(enableSelectScrollHere());
  const select = makeSelect(["a", "b", "c"]);

  let changes = 0;
  select.addEventListener("change", () => changes++);

  select.dispatchEvent(wheel(120));
  expect(select.selectedIndex).toBe(1);
  // A real change event, so every onChange handler picks it up the same way a
  // click on an <option> would.
  expect(changes).toBe(1);

  select.dispatchEvent(wheel(120));
  select.dispatchEvent(wheel(120)); // past the end
  expect(select.selectedIndex).toBe(2);
  expect(changes).toBe(2);
});

it("leaves a disabled select and a one-option select alone", () => {
  detachers.push(enableSelectScrollHere());
  const off = makeSelect(["a", "b"], true);
  off.dispatchEvent(wheel(120));
  expect(off.selectedIndex).toBe(0);

  const lonely = makeSelect(["only"]);
  const event = wheel(120);
  lonely.dispatchEvent(event);
  expect(lonely.selectedIndex).toBe(0);
  // And it does not eat the page's own scrolling on the way past.
  expect(event.defaultPrevented).toBe(false);
});

it("does not touch a wheel anywhere else on the page", () => {
  detachers.push(enableSelectScrollHere());
  const div = document.createElement("div");
  document.body.append(div);
  const event = wheel(120);
  div.dispatchEvent(event);
  expect(event.defaultPrevented).toBe(false);
});
