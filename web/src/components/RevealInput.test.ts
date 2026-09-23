// RevealInput has no hooks, so it is called as a plain function and the
// returned element tree is inspected directly.
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

  it("keeps the eye in the muted text colour, not the accent", () => {
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

  it("reserves trailing padding with important physical utilities that follow the page direction", () => {
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
    // Not pe-8: the input is dir="ltr", so a logical property would always
    // pad the right, whatever the page direction.
    expect(cls).toContain("pr-8!");
    expect(cls).toContain("rtl:pr-0!");
    expect(cls).toContain("rtl:pl-8!");
    expect(cls).not.toMatch(/(?:^|\s)pe-8!/);
    expect(cls).toContain("w-full");
    // The caller's own classes pass through.
    expect(cls).toContain("bg-carbon-surface2");
  });

  it("positions the eye with rtl-gated physical offsets, not end-2", () => {
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
    // end-2 follows the nearest dir ancestor, which can disagree with the page
    // (OffsiteWizard nests the field in a dir="ltr" label).
    expect(cls).toContain("right-2");
    expect(cls).toContain("rtl:right-auto!");
    expect(cls).toContain("rtl:left-2");
    expect(cls).not.toMatch(/(?:^|\s)end-2(?:\s|$)/);
  });

  it("puts the eye and the reserved padding on the same side in both directions", () => {
    // Each class list can look right on its own; both must follow the same
    // direction signal.
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
    // A logical property would follow the element's or an ancestor's dir
    // instead of the page.
    for (const cls of [input, btn]) {
      expect(cls).not.toMatch(/(?:^|\s)(?:p[se]-|inset-inline|start-|end-)/);
    }
  });

  it("puts wrapperClassName on the outer box, not the input", () => {
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

  it("leaves the wrapper a bare relative box without wrapperClassName", () => {
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
