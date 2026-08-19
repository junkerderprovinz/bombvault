// ---------------------------------------------------------------------------
// Toast / ToastViewport — pure-component tests (GlimStone form-engine Task 9).
//
// Same test approach as Toggle.test.ts/ConfirmDialog.test.ts: both are pure,
// hookless function components, invoked directly as plain functions and
// their returned element trees inspected as plain objects — no jsdom. The
// stateful timing behaviour (pause/resume math, quiet-mode filtering) is
// covered separately in lib/toastEngine.test.ts, which this file's
// ToastViewport wiring tests confirm is actually reachable from the UI
// (pause/resume handlers fire with the right id).
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { ToastCard, ToastViewport } from "./Toast";

interface ElementNode {
  type?: unknown;
  props?: { children?: unknown; [key: string]: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null;
}

// ToastViewport composes ToastCard via real JSX (`<ToastCard key={t.id} .../>`
// inside .map()) rather than calling it as a plain function, specifically so
// React keeps its normal per-item `key` reconciliation for a live list whose
// items can be dismissed from the middle (a plain function call has no way
// to carry a JSX `key`, and losing it risks React reusing the wrong DOM node
// — and with it, the wrong toast's hover/focus pause state). That means a
// node in this tree can be `{ type: ToastCard, props }` — a reference to the
// pure component, not yet invoked. Since ToastCard is itself pure/hookless,
// it's just as safe to invoke here as calling it directly (same thing
// Toggle.test.ts/ConfirmDialog.test.ts do at the top level) — `resolve`
// does exactly that recursively, so the walkers below still see the real
// rendered tree underneath.
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

  it("uses role=status (implicit polite live region) for a routine success toast", () => {
    const tree = ToastCard(baseProps) as ElementNode;
    expect(tree.props?.role).toBe("status");
  });

  it("uses role=alert (implicit assertive live region) for a fail toast — failures interrupt", () => {
    const tree = ToastCard({ ...baseProps, severity: "fail" }) as ElementNode;
    expect(tree.props?.role).toBe("alert");
  });

  it("uses role=alert for a warn toast too (blocking, not routine)", () => {
    const tree = ToastCard({ ...baseProps, severity: "warn" }) as ElementNode;
    expect(tree.props?.role).toBe("alert");
  });

  it("gives the dismiss button its accessible name from dismissLabel", () => {
    const tree = ToastCard(baseProps);
    const buttons = findAllButtons(tree);
    expect(buttons).toHaveLength(1);
    expect(buttons[0].props?.["aria-label"]).toBe("Dismiss notification");
  });

  it("calls onDismiss(id) when the dismiss button is clicked — keyboard-activatable via a real <button>", () => {
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

  it("does NOT dismiss on a non-Escape key", () => {
    let calls = 0;
    const tree = ToastCard({ ...baseProps, onDismiss: () => calls++ }) as ElementNode;
    tree.props!.onKeyDown({ key: "Tab", stopPropagation: () => {} });
    expect(calls).toBe(0);
  });

  it("wires onMouseEnter/onMouseLeave to pause/resume with this toast's id (hover pauses)", () => {
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

  it("wires onFocus/onBlur to pause/resume with this toast's id (focus pauses too, not just hover)", () => {
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

  it("marks the card pointer-events-auto so it can be clicked (only the empty viewport space ignores clicks)", () => {
    const tree = ToastCard(baseProps) as ElementNode;
    expect(tree.props?.className).toContain("pointer-events-auto");
  });

  it("never uses a coloured left rail/border-left for severity (rule 5: no vertical marks)", () => {
    const tree = ToastCard({ ...baseProps, severity: "fail" }) as ElementNode;
    expect(tree.props?.className).not.toMatch(/border-l|border-left/);
  });
});

describe("ToastViewport", () => {
  const dismissLabel = "Dismiss notification";

  const noopHandlers = { onMouseEnter: noop, onMouseLeave: noop, onFocus: noop, onBlur: noop };

  it("renders nothing but the (pointer-events-none) wrapper when there are no toasts", () => {
    const tree = ToastViewport({ toasts: [], dismissLabel, onDismiss: noop, ...noopHandlers }) as ElementNode;
    expect(tree.props?.className).toContain("pointer-events-none");
    const cards = findAll(tree, (n) => typeof n.props?.role === "string");
    expect(cards).toHaveLength(0);
  });

  it("caps the viewport height and scrolls as a defensive backstop, so a stack can never spill fully off-screen unreachable", () => {
    const tree = ToastViewport({ toasts: [], dismissLabel, onDismiss: noop, ...noopHandlers }) as ElementNode;
    expect(tree.props?.className).toContain("max-h-screen");
    expect(tree.props?.className).toContain("overflow-y-auto");
  });

  it("insets the corner with padding, not with a flush offset — the scroll backstop must not clip what paints outside each card", () => {
    const tree = ToastViewport({ toasts: [], dismissLabel, onDismiss: noop, ...noopHandlers }) as ElementNode;
    const className = String(tree.props?.className ?? "");
    // overflow-y-auto forces overflow-x to compute to auto as well, so the clip
    // boundary is the padding box. p-4 keeps each card's --elevation drop shadow
    // AND the translateX(12px) start of its entrance animation inside it; a
    // flush bottom-4/end-4 box with no padding sliced both off (live-measured:
    // scrollWidth 332 vs clientWidth 320 mid-entrance). Same 1rem corner gap
    // either way — see ToastViewport's comment in Toast.tsx.
    expect(className).toContain("p-4");
    expect(className).toContain("bottom-0");
    expect(className).toContain("end-0");
    expect(className).not.toMatch(/\bbottom-4\b/);
    expect(className).not.toMatch(/\bend-4\b/);
  });

  it("stacks multiple toasts — all render simultaneously, not one replacing another", () => {
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

  it("each stacked toast is independently dismissible — dismissing one calls onDismiss with only ITS id", () => {
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
    buttons[1].props!.onClick(); // dismiss only the second toast
    expect(calls).toEqual(["b"]);
  });

  it("passes the four hover/focus events through to each card independently — NOT collapsed into a single pause/resume pair", () => {
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
    // All four are distinct callbacks reaching the caller — a caller can
    // track hover/focus as two independent flags and decide for itself
    // whether "leave" or "blur" should actually resume the countdown.
    expect(seen).toEqual(["enter:a", "focus:a", "leave:a", "blur:a"]);
  });

  it("the viewport wrapper itself never carries an onClick/backdrop handler (never modal, unlike ConfirmDialog)", () => {
    const tree = ToastViewport({
      toasts: [{ id: "a", message: "A", severity: "success" }],
      dismissLabel,
      onDismiss: noop,
      ...noopHandlers,
    }) as ElementNode;
    expect(tree.props?.onClick).toBeUndefined();
  });
});
