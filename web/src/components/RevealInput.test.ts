// ---------------------------------------------------------------------------
// RevealInput — the reveal-eye affordance (GlimStone form-engine Task 6).
//
// Same test approach as Toggle.test.ts: RevealInput is a pure, hookless
// function component, so it's invoked directly as a plain function and its
// returned element tree inspected as plain objects — no jsdom/renderer.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { RevealInput } from "./RevealInput";

interface ElementNode {
  type?: unknown;
  props?: { children?: unknown; [key: string]: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null;
}

function findAll(node: unknown, pred: (n: ElementNode) => boolean, out: ElementNode[] = []): ElementNode[] {
  if (!isElementNode(node)) return out;
  if (Array.isArray(node)) {
    for (const c of node) findAll(c, pred, out);
    return out;
  }
  if (pred(node)) out.push(node);
  if (node.props?.children !== undefined) findAll(node.props.children, pred, out);
  return out;
}

function findOne(tree: unknown, type: string): ElementNode {
  const found = findAll(tree, (n) => n.type === type);
  expect(found.length).toBe(1);
  return found[0];
}

const noop = () => {};

describe("RevealInput", () => {
  it("renders a password input by default (visible=false)", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "secret",
      onChange: noop,
    });
    const input = findOne(tree, "input");
    expect(input.props.type).toBe("password");
  });

  it("switches to a plain text input when visible=true", () => {
    const tree = RevealInput({
      visible: true,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "secret",
      onChange: noop,
    });
    const input = findOne(tree, "input");
    expect(input.props.type).toBe("text");
  });

  it("exposes exactly one eye button with the show-label while hidden", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    const btn = findOne(tree, "button");
    expect(btn.props["aria-label"]).toBe("Show value");
    expect(btn.props["aria-pressed"]).toBe(false);
  });

  it("swaps the eye button's label to hide-label once revealed", () => {
    const tree = RevealInput({
      visible: true,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    const btn = findOne(tree, "button");
    expect(btn.props["aria-label"]).toBe("Hide value");
    expect(btn.props["aria-pressed"]).toBe(true);
  });

  it("calls onToggleVisible when the eye is clicked", () => {
    let calls = 0;
    const tree = RevealInput({
      visible: false,
      onToggleVisible: () => {
        calls++;
      },
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    findOne(tree, "button").props.onClick();
    expect(calls).toBe(1);
  });

  it("never colours the eye with the accent — neutral text/opacity treatment only", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    const btn = findOne(tree, "button");
    const cls = btn.props.className as string;
    expect(cls).toContain("text-carbon-textMuted");
    expect(cls).not.toContain("accent");
  });

  it("reserves trailing padding with an important, LOGICAL pe-8 so a shared inputCls's px-* can't win the cascade", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
      className: "px-3 py-1.5 bg-carbon-surface2",
    });
    const input = findOne(tree, "input");
    const cls = input.props.className as string;
    // pe-8 (logical "padding-end"), not pr-8 — the eye sits on the field's
    // TRAILING edge, which flips sides between LTR and RTL, so a PHYSICAL
    // pr-8 would reserve the wrong side's room once dir="rtl" is active.
    expect(cls).toContain("pe-8!");
    expect(cls).not.toMatch(/(?:^|\s)pr-8!/);
    expect(cls).toContain("w-full");
    // The caller's own visual classes (background, vertical padding) still
    // pass through untouched — only the trailing padding is pinned.
    expect(cls).toContain("bg-carbon-surface2");
  });

  it("positions the eye on the LOGICAL trailing edge (end-2), not the physical right, so it flips correctly under dir=\"rtl\"", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    const btn = findOne(tree, "button");
    const cls = btn.props.className as string;
    expect(cls).toContain("end-2");
    expect(cls).not.toMatch(/(?:^|\s)right-2(?:\s|$)/);
  });

  it("wrapperClassName carries the field's layout footprint on the outer box, not the input", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
      wrapperClassName: "flex-1 min-w-0",
    });
    expect((tree as ElementNode).props?.className).toBe("relative flex-1 min-w-0");
  });

  it("defaults the wrapper to a bare relative box with no shrink-to-content sizing when wrapperClassName is omitted", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    expect((tree as ElementNode).props?.className).toBe("relative");
  });

  it("passes through arbitrary input props (readOnly, autoComplete, placeholder) unchanged", () => {
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "tok_abc",
      onChange: noop,
      readOnly: true,
      autoComplete: "new-password",
      placeholder: "secret set",
    });
    const input = findOne(tree, "input");
    expect(input.props.readOnly).toBe(true);
    expect(input.props.autoComplete).toBe("new-password");
    expect(input.props.placeholder).toBe("secret set");
    expect(input.props.value).toBe("tok_abc");
  });
});
