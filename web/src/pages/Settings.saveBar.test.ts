// ---------------------------------------------------------------------------
// SaveBar — hueIndex (GlimStone follow-up round, Paths & Storage tab rework,
// point 3: "die Speichern-Buttons sind nicht in der Farbengine").
//
// SaveBar (like ToggleRow — see Settings.toggleRow.test.ts's own header
// comment) is a pure, hookless function component: props in, a plain React
// element tree out. Called directly as a plain function and inspected
// without a renderer, same "node environment, no DOM" footing every other
// pure-component test in this repo uses.
// ---------------------------------------------------------------------------
import { describe, expect, it, vi } from "vitest";
import { SaveBar } from "./Settings";

interface ButtonEl {
  type?: unknown;
  props?: { className?: string; style?: Record<string, unknown>; disabled?: boolean; onClick?: () => void; children?: unknown };
}

function findButton(tree: unknown): ButtonEl {
  // SaveBar renders <div><button>...</button></div> — the button is the
  // div's sole child.
  const div = tree as { props?: { children?: ButtonEl } };
  const button = div.props?.children;
  expect(button).toBeTruthy();
  return button as ButtonEl;
}

describe("SaveBar — hueIndex", () => {
  it("omitting hueIndex renders no .glim-hue class and no rainbow inline style (every pre-existing call site, unchanged)", () => {
    const btn = findButton(SaveBar({ state: "idle", onSave: () => {}, t: (k) => k }));
    expect(String(btn.props?.className)).not.toContain("glim-hue");
    expect(btn.props?.style).toBeUndefined();
  });

  it("a given hueIndex adds .glim-hue and a rainbow custom-property inline style", () => {
    const btn = findButton(SaveBar({ state: "idle", onSave: () => {}, t: (k) => k, hueIndex: 2 }));
    expect(String(btn.props?.className)).toContain("glim-hue");
    expect(btn.props?.style).toBeTruthy();
  });

  it("hueIndex 0 is a real, valid position — not treated as falsy/omitted", () => {
    // 0 is a legitimate list index (the FIRST card on the tab) — a naive
    // `hueIndex &&` check would silently skip it, exactly like Badge.tsx's
    // and ToggleRow's own `hueIndex !== undefined` checks guard against.
    const btn = findButton(SaveBar({ state: "idle", onSave: () => {}, t: (k) => k, hueIndex: 0 }));
    expect(String(btn.props?.className)).toContain("glim-hue");
    expect(btn.props?.style).toBeTruthy();
  });

  it("different hueIndex values resolve to different --item-hue custom properties, so sibling Save buttons don't all render the same colour", () => {
    const a = findButton(SaveBar({ state: "idle", onSave: () => {}, t: (k) => k, hueIndex: 0 }));
    const b = findButton(SaveBar({ state: "idle", onSave: () => {}, t: (k) => k, hueIndex: 1 }));
    expect(a.props?.style).not.toEqual(b.props?.style);
  });

  it("still forwards onSave/disabled/state to the underlying button unchanged", () => {
    const spy = vi.fn();
    const btn = findButton(SaveBar({ state: "saving", onSave: spy, t: (k) => k, disabled: false, hueIndex: 3 }));
    expect(btn.props?.disabled).toBe(true); // state === "saving" disables it
    btn.props?.onClick?.();
    expect(spy).toHaveBeenCalled();
  });
});
