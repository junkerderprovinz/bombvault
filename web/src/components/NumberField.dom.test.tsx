// @vitest-environment jsdom
/**
 * The two things GlimStone 1.7.4 and 1.7.5 added to this field, and neither of
 * them is visible in the types.
 *
 * The wheel is a behaviour with a condition that IS the design: it steps only
 * while the field has focus, because a field that answers a wheel on hover
 * changes values somebody was scrolling past. A test that only asserts "the
 * wheel steps" would pass on the hazardous version too, so both halves are
 * checked here.
 *
 * The wrapper is a layout rule that typechecks perfectly either way. `block`
 * alone stretches inside a column flex parent, which is where this helper is
 * used most, and that put the steppers at the far edge of the row instead of
 * at the edge of the field.
 */
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { NumberField } from "./NumberField";

afterEach(cleanup);

function wheelOn(el: HTMLElement, deltaY: number) {
  const e = new WheelEvent("wheel", { deltaY, bubbles: true, cancelable: true });
  el.dispatchEvent(e);
  return e;
}

describe("NumberField, the wheel", () => {
  it("steps the value up while the field has focus", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    input.focus();
    wheelOn(input, -100); // up is more, matching the upper arrow and the up key
    expect(input.value).toBe("6");
  });

  it("steps down on the other direction", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    input.focus();
    wheelOn(input, 100);
    expect(input.value).toBe("4");
  });

  it("does NOTHING without focus, which is the whole point", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    expect(input.ownerDocument.activeElement).not.toBe(input);
    wheelOn(input, -100);
    expect(input.value).toBe("5");
  });

  it("prevents the default so the page does not scroll the field away mid-adjustment", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    input.focus();
    const e = wheelOn(input, -100);
    expect(e.defaultPrevented).toBe(true);
  });

  it("leaves the page scrolling when the field is not focused", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    const e = wheelOn(input, -100);
    expect(e.defaultPrevented).toBe(false);
  });

  it("respects the field's own bounds rather than writing past them", () => {
    render(<NumberField defaultValue={10} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    input.focus();
    wheelOn(input, -100);
    expect(input.value).toBe("10");
  });

  it("ignores a wheel event with no vertical delta", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    input.focus();
    const e = wheelOn(input, 0);
    expect(input.value).toBe("5");
    expect(e.defaultPrevented).toBe(false);
  });

  it("does not step a read-only field", () => {
    render(<NumberField defaultValue={5} min={0} max={10} step={1} readOnly aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    input.focus();
    wheelOn(input, -100);
    expect(input.value).toBe("5");
  });
});

describe("NumberField, the wrapper", () => {
  it("is sized by its content, so the steppers sit at the edge of the FIELD", () => {
    render(<NumberField defaultValue={5} aria-label="n" />);
    const input = screen.getByLabelText("n") as HTMLInputElement;
    const wrap = input.parentElement!;
    // w-fit is the half that fixes it; the two alignment utilities stop a grid
    // or flex parent stretching it back out on the cross axis.
    expect(wrap.className).toContain("w-fit");
    expect(wrap.className).toContain("self-start");
    expect(wrap.className).toContain("justify-self-start");
  });

  it("still cannot outgrow a narrow parent", () => {
    render(<NumberField defaultValue={5} aria-label="n" />);
    const wrap = (screen.getByLabelText("n") as HTMLInputElement).parentElement!;
    expect(wrap.className).toContain("max-w-full");
  });
});
