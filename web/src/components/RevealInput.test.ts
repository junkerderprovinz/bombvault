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

  it("reserves trailing padding with important, direction-aware PHYSICAL utilities so a shared inputCls's px-* can't win the cascade", () => {
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
    // NOT the logical `pe-8` this used to be: this <input> carries its own
    // `dir="ltr"` (see below), and a logical property resolves against the
    // direction of the element it's on — so `pe-8` here would always
    // resolve to padding-right, regardless of the PAGE's direction, landing
    // the reserved room on the wrong side once dir="rtl" is active (a real,
    // shipped regression from form-engine Phase 2 Task 6 — see the
    // follow-up fix). `pr-8!` is the unconditional LTR-default base; the
    // `rtl:` pair swaps it to the left once an ancestor has dir="rtl" —
    // that variant's ancestor-only match clause still fires correctly even
    // though this element's OWN direction is pinned to ltr.
    expect(cls).toContain("pr-8!");
    expect(cls).toContain("rtl:pr-0!");
    expect(cls).toContain("rtl:pl-8!");
    expect(cls).not.toMatch(/(?:^|\s)pe-8!/);
    expect(cls).toContain("w-full");
    // The caller's own visual classes (background, vertical padding) still
    // pass through untouched — only the trailing padding is pinned.
    expect(cls).toContain("bg-carbon-surface2");
  });

  it("positions the eye on the trailing edge with the SAME page-gated physical pattern as the input's padding, never the logical end-2", () => {
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
    // The eye still lands on the field's trailing edge (right in LTR, left in
    // RTL) — but via `rtl:`-gated PHYSICAL offsets, not the logical `end-2`
    // this used to be. `end-2` resolves against the nearest `dir` ANCESTOR
    // while `rtl:` resolves against the PAGE, and a call site is free to nest
    // this component inside its own `dir="ltr"` island (OffsiteWizard's
    // `<label dir="ltr">RESTIC_REST_PASSWORD</label>` does exactly that) —
    // there the two disagreed and the eye ended up on the opposite side from
    // the padding that reserves room for it, secret text under the icon.
    expect(cls).toContain("right-2");
    expect(cls).toContain("rtl:right-auto!");
    expect(cls).toContain("rtl:left-2");
    expect(cls).not.toMatch(/(?:^|\s)end-2(?:\s|$)/);
  });

  it("drives the eye's offset and the input's padding reservation off the SAME direction signal", () => {
    // The regression this guards is not either class list on its own — each
    // looked right in isolation — it is the two halves resolving direction
    // against DIFFERENT things (element/ancestor `dir` vs. the page). Assert
    // the pairing itself: the reserved room and the icon must always be on
    // the same side, so both must be `rtl:`-gated physical properties.
    const tree = RevealInput({
      visible: false,
      onToggleVisible: noop,
      showLabel: "Show value",
      hideLabel: "Hide value",
      value: "",
      onChange: noop,
    });
    const input = findOne(tree, "input").props.className as string;
    const btn = findOne(tree, "button").props.className as string;
    // LTR default: padding reserved on the right, eye on the right.
    expect(input).toContain("pr-8!");
    expect(btn).toContain("right-2");
    // RTL override: padding moves to the left, eye moves to the left.
    expect(input).toContain("rtl:pl-8!");
    expect(btn).toContain("rtl:left-2");
    // Neither half may use a logical property, which would resolve against
    // the element's own / its nearest ancestor's `dir` instead of the page's.
    for (const cls of [input, btn]) {
      expect(cls).not.toMatch(/(?:^|\s)(?:p[se]-|inset-inline|start-|end-)/);
    }
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
