// @vitest-environment jsdom
// Rule 40 under rainbow: a button lit by the pointer or the keyboard takes the
// colour of its place among the page's buttons, not the one of the card it
// stands in.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { RAINBOW_OFF, applyRainbow, watchPageHue } from "./appearance";

let stop: () => void = () => {};

beforeEach(() => {
  document.body.innerHTML = `
    <button class="glim-btn glim-btn-neutral" id="a"><span class="glim-btn-glyph"></span></button>
    <button class="glim-btn glim-btn-subtle" id="b"></button>
    <button class="glim-btn" id="plain"></button>
    <button class="glim-btn-bare" id="c"></button>
    <button class="glim-btn glim-btn-neutral glim-hue" id="own"></button>`;
  stop = watchPageHue();
});

afterEach(() => {
  stop();
  applyRainbow(RAINBOW_OFF, { animate: false });
});

const tint = (id: string) => document.getElementById(id)!.style.getPropertyValue("--btn-tint");
const hover = (el: Element) => el.dispatchEvent(new Event("pointerover", { bubbles: true }));

describe("watchPageHue", () => {
  it("counts the tinted buttons in the order they stand", () => {
    applyRainbow({ ...RAINBOW_OFF, on: true }, { animate: false });
    hover(document.getElementById("b")!);
    document.getElementById("c")!.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
    expect(tint("b")).toBe("color-mix(in srgb, var(--rb-1) var(--ink-mix), black)");
    expect(tint("c")).toBe("color-mix(in srgb, var(--rb-2) var(--ink-mix), black)");
  });

  it("answers for the button when the pointer lands on its glyph", () => {
    applyRainbow({ ...RAINBOW_OFF, on: true }, { animate: false });
    hover(document.querySelector("#a .glim-btn-glyph")!);
    expect(tint("a")).toBe("color-mix(in srgb, var(--rb-0) var(--ink-mix), black)");
  });

  it("leaves a button with its own palette position alone", () => {
    applyRainbow({ ...RAINBOW_OFF, on: true }, { animate: false });
    hover(document.getElementById("own")!);
    expect(tint("own")).toBe("");
  });

  it("drops the tint again once rainbow is off, so the accent shows", () => {
    applyRainbow({ ...RAINBOW_OFF, on: true }, { animate: false });
    hover(document.getElementById("a")!);
    applyRainbow(RAINBOW_OFF, { animate: false });
    hover(document.getElementById("a")!);
    expect(tint("a")).toBe("");
  });
});
