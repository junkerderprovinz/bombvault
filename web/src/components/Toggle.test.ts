// Toggle uses no hooks, so these tests call it as a plain function and walk
// the returned element tree without a renderer.
import { describe, expect, it } from "vitest";
import { Toggle } from "./Toggle";

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

function findOneButton(tree: unknown): ElementNode {
  const btns = findAll(tree, (n) => n.type === "button");
  expect(btns.length).toBe(1);
  return btns[0];
}

function visibleText(node: unknown): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(visibleText).join("");
  if (isElementNode(node) && node.props?.children !== undefined) return visibleText(node.props.children);
  return "";
}

describe("Toggle", () => {
  it("renders a visible label by default", () => {
    const tree = Toggle({ checked: false, onChange: () => {}, label: "Weekly digest" });
    expect(visibleText(tree)).toContain("Weekly digest");
  });

  it("hideLabel renders no visible text but still exposes the label as aria-label", () => {
    const tree = Toggle({ checked: false, onChange: () => {}, label: "Weekly digest", hideLabel: true });
    expect(visibleText(tree)).not.toContain("Weekly digest");
    const btn = findOneButton(tree);
    expect(btn.props["aria-label"]).toBe("Weekly digest");
  });

  it("sets aria-label when the label is visible too", () => {
    const tree = Toggle({ checked: false, onChange: () => {}, label: "Weekly digest" });
    const btn = findOneButton(tree);
    expect(btn.props["aria-label"]).toBe("Weekly digest");
  });

  it("renders the on state as a checked switch with the accent fill", () => {
    const tree = Toggle({ checked: true, onChange: () => {}, label: "X" });
    const btn = findOneButton(tree);
    expect(btn.props.role).toBe("switch");
    expect(btn.props["aria-checked"]).toBe(true);
    expect(btn.props.className).toContain("bg-accent");
    expect(btn.props.className).not.toContain("bg-carbon-surface3");
  });

  it("renders the off state with the neutral track", () => {
    const tree = Toggle({ checked: false, onChange: () => {}, label: "X" });
    const btn = findOneButton(tree);
    expect(btn.props["aria-checked"]).toBe(false);
    expect(btn.props.className).toContain("bg-carbon-surface3");
    expect(btn.props.className).not.toContain("bg-accent");
  });

  it("slides the thumb between the off and on x-offsets", () => {
    const offThumb = findOneButton(Toggle({ checked: false, onChange: () => {}, label: "X" })).props.children;
    const onThumb = findOneButton(Toggle({ checked: true, onChange: () => {}, label: "X" })).props.children;
    expect(offThumb.props.className).toContain("translate-x-[3px]");
    expect(onThumb.props.className).toContain("translate-x-[18px]");
  });

  it("propagates disabled to the underlying control", () => {
    const enabled = findOneButton(Toggle({ checked: false, onChange: () => {}, label: "X" }));
    const disabled = findOneButton(Toggle({ checked: false, onChange: () => {}, label: "X", disabled: true }));
    expect(enabled.props.disabled).toBeFalsy();
    expect(disabled.props.disabled).toBe(true);
  });

  it("calls onChange with the flipped value when clicked", () => {
    let seen: boolean | undefined;
    const tree = Toggle({
      checked: false,
      onChange: (v: boolean) => {
        seen = v;
      },
      label: "X",
    });
    findOneButton(tree).props.onClick();
    expect(seen).toBe(true);
  });
});
