// The toast components use no hooks, so these tests call them as plain
// functions and inspect the returned element trees. Timing lives in
// lib/toastEngine.test.ts.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { ToastCard, ToastViewport } from "./Toast";

interface ElementNode {
  type?: unknown;
  props?: { children?: unknown; [key: string]: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null;
}

// ToastViewport renders ToastCard as JSX so each card keeps its key, which
// leaves unrendered `{ type: ToastCard, props }` nodes in the tree. resolve
// calls such components so the walkers below see what they render.
function resolve(node: unknown): unknown {
  if (isElementNode(node) && !Array.isArray(node) && typeof node.type === "function") {
    return (node.type as (props: unknown) => unknown)(node.props ?? {});
  }
  return node;
}

function findAll(node: unknown, pred: (n: ElementNode) => boolean, out: ElementNode[] = []): ElementNode[] {
  const resolved = resolve(node);
  if (!isElementNode(resolved)) return out;
  if (Array.isArray(resolved)) {
    for (const c of resolved) findAll(c, pred, out);
    return out;
  }
  if (pred(resolved)) out.push(resolved);
  if (resolved.props?.children !== undefined) findAll(resolved.props.children, pred, out);
  return out;
}

// The card's own elements, without calling the components inside it, since
// Button uses hooks.
function findShallow(node: unknown, pred: (n: ElementNode) => boolean, out: ElementNode[] = []): ElementNode[] {
  if (Array.isArray(node)) {
    for (const c of node) findShallow(c, pred, out);
    return out;
  }
  if (!isElementNode(node)) return out;
  if (pred(node)) out.push(node);
  if (node.props?.children !== undefined) findShallow(node.props.children, pred, out);
  return out;
}

function findAllButtons(tree: unknown): ElementNode[] {
  return findAll(tree, (n) => n.type === "button");
}

function visibleText(node: unknown): string {
  const resolved = resolve(node);
  if (resolved == null || typeof resolved === "boolean") return "";
  if (typeof resolved === "string" || typeof resolved === "number") return String(resolved);
  if (Array.isArray(resolved)) return resolved.map(visibleText).join("");
  if (isElementNode(resolved) && resolved.props?.children !== undefined) return visibleText(resolved.props.children);
  return "";
}

const noop = () => {};

describe("ToastCard", () => {
  const baseProps = {
    id: "t1",
    message: "Settings saved",
    severity: "success" as const,
    dismissLabel: "Dismiss notification",
    onDismiss: noop,
    onMouseEnter: noop,
    onMouseLeave: noop,
    onFocus: noop,
    onBlur: noop,
  };

  it("renders the message text", () => {
    const tree = ToastCard(baseProps);
    expect(visibleText(tree)).toContain("Settings saved");
  });

  it("uses role=status for a success toast", () => {
    const tree = ToastCard(baseProps) as ElementNode;
    expect(tree.props?.role).toBe("status");
  });

  it("uses role=alert for a fail toast", () => {
    const tree = ToastCard({ ...baseProps, severity: "fail" }) as ElementNode;
    expect(tree.props?.role).toBe("alert");
  });

  it("uses role=alert for a warn toast", () => {
    const tree = ToastCard({ ...baseProps, severity: "warn" }) as ElementNode;
    expect(tree.props?.role).toBe("alert");
  });

  it("gives the dismiss button its accessible name from dismissLabel", () => {
    const tree = ToastCard(baseProps);
    const buttons = findAllButtons(tree);
    expect(buttons).toHaveLength(1);
    expect(buttons[0].props?.["aria-label"]).toBe("Dismiss notification");
  });

  it("calls onDismiss(id) when the dismiss button is clicked", () => {
    let seen: string | undefined;
    const tree = ToastCard({ ...baseProps, onDismiss: (id) => (seen = id) });
    const buttons = findAllButtons(tree);
    buttons[0].props!.onClick();
    expect(seen).toBe("t1");
  });

  it("calls onDismiss(id) on Escape", () => {
    let seen: string | undefined;
    const tree = ToastCard({ ...baseProps, onDismiss: (id) => (seen = id) }) as ElementNode;
    tree.props!.onKeyDown({ key: "Escape", stopPropagation: () => {} });
    expect(seen).toBe("t1");
  });

  it("ignores other keys", () => {
    let calls = 0;
    const tree = ToastCard({ ...baseProps, onDismiss: () => calls++ }) as ElementNode;
    tree.props!.onKeyDown({ key: "Tab", stopPropagation: () => {} });
    expect(calls).toBe(0);
  });

  it("passes its id to onMouseEnter and onMouseLeave", () => {
    const seen: string[] = [];
    const tree = ToastCard({
      ...baseProps,
      onMouseEnter: (id) => seen.push(`enter:${id}`),
      onMouseLeave: (id) => seen.push(`leave:${id}`),
    }) as ElementNode;
    tree.props!.onMouseEnter();
    tree.props!.onMouseLeave();
    expect(seen).toEqual(["enter:t1", "leave:t1"]);
  });

  it("passes its id to onFocus and onBlur", () => {
    const seen: string[] = [];
    const tree = ToastCard({
      ...baseProps,
      onFocus: (id) => seen.push(`focus:${id}`),
      onBlur: (id) => seen.push(`blur:${id}`),
    }) as ElementNode;
    tree.props!.onFocus();
    tree.props!.onBlur();
    expect(seen).toEqual(["focus:t1", "blur:t1"]);
  });

  it("runs its action and then dismisses itself", () => {
    let ran = 0;
    let dismissed: string | undefined;
    const tree = ToastCard({
      ...baseProps,
      action: { label: "Undo", labelKey: "takeover.undo", onClick: () => ran++ },
      onDismiss: (id) => (dismissed = id),
    }) as ElementNode;
    const [action] = findShallow(tree, (n) => n.props?.label === "Undo");
    (action.props!.onClick as () => void)();
    expect(ran).toBe(1);
    expect(dismissed).toBe("t1");
  });

  it("puts its action under the message, so the message keeps the card's width", () => {
    const tree = ToastCard({
      ...baseProps,
      action: { label: "Undo", labelKey: "takeover.undo", onClick: noop },
    }) as ElementNode;
    const [body] = findShallow(tree, (n) => n.type === "div" && /\bflex-col\b/.test(String(n.props?.className)));
    expect(findShallow(body, (n) => n.type === "p")).toHaveLength(1);
    expect(findShallow(body, (n) => n.props?.label === "Undo")).toHaveLength(1);
  });

  it("lets a path break after each slash", () => {
    const tree = ToastCard({ ...baseProps, message: "Saved to /mnt/user" }) as ElementNode;
    expect(findShallow(tree, (n) => n.type === "wbr")).toHaveLength(2);
    expect(visibleText(tree)).toContain("Saved to /mnt/user");
  });

  it("takes pointer events on the card", () => {
    const tree = ToastCard(baseProps) as ElementNode;
    expect(tree.props?.className).toContain("pointer-events-auto");
  });

  it("has no coloured side border", () => {
    const tree = ToastCard({ ...baseProps, severity: "fail" }) as ElementNode;
    expect(tree.props?.className).not.toMatch(/border-l|border-left/);
  });

  // A line of 1.375em comes out at 19.25px, and every toast stacked beside it
  // lands on a fraction that Chromium rounds differently from paint to paint.
  it("sets its message on whole-pixel lines", () => {
    const tree = ToastCard(baseProps) as ElementNode;
    const [message] = findShallow(tree, (n) => n.type === "p");
    expect(String(message.props?.className)).toMatch(/\bleading-5\b/);
  });
});

describe("the toast's entrance", () => {
  const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");

  // A held end is an identity matrix rather than none, and any transform makes
  // the card the containing block of a fixed element inside it.
  it("holds nothing once the toast is in", () => {
    const rules = [...css.matchAll(/\.glim-toast\s*\{([^}]*)\}/g)].map((m) => m[1]).filter((r) => /animation\s*:/.test(r));
    expect(rules.length).toBeGreaterThanOrEqual(3);
    for (const rule of rules) {
      expect(rule).toMatch(/\bbackwards\b/);
      expect(rule).not.toMatch(/\b(both|forwards)\b/);
    }
  });
});

describe("ToastViewport", () => {
  const dismissLabel = "Dismiss notification";

  const noopHandlers = { onMouseEnter: noop, onMouseLeave: noop, onFocus: noop, onBlur: noop };

  it("renders only the click-through wrapper when there are no toasts", () => {
    const tree = ToastViewport({ toasts: [], dismissLabel, onDismiss: noop, ...noopHandlers }) as ElementNode;
    expect(tree.props?.className).toContain("pointer-events-none");
    const cards = findAll(tree, (n) => typeof n.props?.role === "string");
    expect(cards).toHaveLength(0);
  });

  it("caps the viewport height and scrolls", () => {
    const tree = ToastViewport({ toasts: [], dismissLabel, onDismiss: noop, ...noopHandlers }) as ElementNode;
    expect(tree.props?.className).toContain("max-h-screen");
    expect(tree.props?.className).toContain("overflow-y-auto");
  });

  it("insets the corner with padding so the scroll box does not clip the cards", () => {
    const tree = ToastViewport({ toasts: [], dismissLabel, onDismiss: noop, ...noopHandlers }) as ElementNode;
    const className = String(tree.props?.className ?? "");
    // The clip boundary is the padding box, so p-4 keeps each card's shadow
    // and the offset start of its slide-in inside it.
    expect(className).toContain("p-4");
    expect(className).toContain("bottom-0");
    expect(className).toContain("end-0");
    expect(className).not.toMatch(/\bbottom-4\b/);
    expect(className).not.toMatch(/\bend-4\b/);
  });

  it("renders several toasts at once", () => {
    const tree = ToastViewport({
      toasts: [
        { id: "a", message: "First", severity: "success" },
        { id: "b", message: "Second", severity: "fail" },
        { id: "c", message: "Third", severity: "warn" },
      ],
      dismissLabel,
      onDismiss: noop,
      ...noopHandlers,
    });
    const text = visibleText(tree);
    expect(text).toContain("First");
    expect(text).toContain("Second");
    expect(text).toContain("Third");
  });

  it("dismisses a stacked toast by its own id", () => {
    const calls: string[] = [];
    const tree = ToastViewport({
      toasts: [
        { id: "a", message: "First", severity: "success" },
        { id: "b", message: "Second", severity: "success" },
      ],
      dismissLabel,
      onDismiss: (id) => calls.push(id),
      ...noopHandlers,
    });
    const buttons = findAllButtons(tree);
    expect(buttons).toHaveLength(2);
    buttons[1].props!.onClick();
    expect(calls).toEqual(["b"]);
  });

  it("passes hover and focus events through separately", () => {
    const seen: string[] = [];
    const tree = ToastViewport({
      toasts: [{ id: "a", message: "A", severity: "success" }],
      dismissLabel,
      onDismiss: noop,
      onMouseEnter: (id) => seen.push(`enter:${id}`),
      onMouseLeave: (id) => seen.push(`leave:${id}`),
      onFocus: (id) => seen.push(`focus:${id}`),
      onBlur: (id) => seen.push(`blur:${id}`),
    });
    const [card] = findAll(tree, (n) => typeof n.props?.role === "string");
    card.props!.onMouseEnter();
    card.props!.onFocus();
    card.props!.onMouseLeave();
    card.props!.onBlur();
    expect(seen).toEqual(["enter:a", "focus:a", "leave:a", "blur:a"]);
  });

  it("has no click handler on the wrapper", () => {
    const tree = ToastViewport({
      toasts: [{ id: "a", message: "A", severity: "success" }],
      dismissLabel,
      onDismiss: noop,
      ...noopHandlers,
    }) as ElementNode;
    expect(tree.props?.onClick).toBeUndefined();
  });
});
