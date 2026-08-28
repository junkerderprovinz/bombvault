// ---------------------------------------------------------------------------
// ConfirmDialog — the styled window.confirm() replacement (GlimStone
// form-engine Task 7).
//
// Same test approach as Toggle.test.ts/RevealInput.test.ts: ConfirmDialog is
// a pure, hookless function component, so it's invoked directly as a plain
// function and its returned element tree inspected as plain objects — no
// jsdom/renderer. Escape, the Tab focus-trap, the createPortal(...,
// document.body) call, and returning focus to the trigger on close all live
// in the companion hook (lib/useConfirm.tsx) instead of here, precisely
// because they need `document`/a real hook lifecycle that this node-
// environment, call-it-directly test style cannot provide — see that file's
// header comment. They're covered by live Playwright verification instead.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { ConfirmDialog } from "./ConfirmDialog";

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

function findAllButtons(tree: unknown): ElementNode[] {
  // Both kinds count: a raw <button> element, and the shared <Button>
  // component (#178), whose node type is the function itself rather than the
  // string "button". This tree is never rendered, only inspected, so a
  // component node stays a component node.
  return findAll(
    tree,
    (n) => n.type === "button" || (typeof n.type === "function" && n.type.name === "Button")
  );
}

function visibleText(node: unknown): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(visibleText).join("");
  if (isElementNode(node) && node.props?.children !== undefined) return visibleText(node.props.children);
  return "";
}

const noop = () => {};

const baseProps = {
  title: "Confirm",
  message: "Delete this backup? This cannot be undone.",
  confirmLabel: "Confirm",
  cancelLabel: "Cancel",
  closeLabel: "Close",
  onConfirm: noop,
  onCancel: noop,
};

describe("ConfirmDialog", () => {
  it("renders the title, the message, and both button labels", () => {
    const tree = ConfirmDialog(baseProps);
    const text = visibleText(tree);
    expect(text).toContain("Confirm");
    expect(text).toContain("Delete this backup? This cannot be undone.");
    expect(text).toContain("Cancel");
  });

  it("uses the SAME message copy passed in — never rewrites or truncates it", () => {
    const longMessage =
      "Delete ALL backups of this container? The snapshots are permanently removed from the repository and cannot be undone.";
    const tree = ConfirmDialog({ ...baseProps, message: longMessage });
    expect(visibleText(tree)).toContain(longMessage);
  });

  it("calls onConfirm when the confirm button is clicked", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onConfirm: () => calls++ });
    const buttons = findAllButtons(tree);
    const confirmBtn = buttons.find((b) => visibleText(b) === "Confirm" && b.props?.autoFocus !== true);
    expect(confirmBtn).toBeDefined();
    confirmBtn!.props!.onClick();
    expect(calls).toBe(1);
  });

  it("calls onCancel when the footer cancel button is clicked", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onCancel: () => calls++ });
    const buttons = findAllButtons(tree);
    const cancelBtn = buttons.find((b) => visibleText(b) === "Cancel");
    expect(cancelBtn).toBeDefined();
    cancelBtn!.props!.onClick();
    expect(calls).toBe(1);
  });

  it("calls onCancel when the header close (X) button is clicked", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onCancel: () => calls++ });
    const buttons = findAllButtons(tree);
    // Since #178 the close control is a <Button> like any other, so its name
    // is its LABEL rather than an aria-label on a glyph-only square. It is
    // still the only button carrying the close label, which is the property
    // that matters: the header's X and the footer's Cancel stay distinct.
    const closeBtn = buttons.find((b) => b.props?.label === "Close");
    expect(closeBtn).toBeDefined();
    closeBtn!.props!.onClick();
    expect(calls).toBe(1);
  });

  it("gives the header close (X) button its OWN accessible name, distinct from the footer Cancel button", () => {
    const tree = ConfirmDialog(baseProps);
    const buttons = findAllButtons(tree);
    const closeBtn = buttons.find((b) => b.props?.label === "Close");
    expect(closeBtn?.props?.label).toBe("Close");
    // The point of this test survives the #178 conversion unchanged: the two
    // controls must not share a name, or "Cancel" and "Close" become one
    // thing to anyone navigating by name.
    expect(closeBtn?.props?.label).not.toBe(baseProps.cancelLabel);
  });

  it("calls onCancel when the backdrop itself is clicked (not a click inside the card)", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onCancel: () => calls++ }) as ElementNode;
    const target = {};
    tree.props!.onClick({ target, currentTarget: target });
    expect(calls).toBe(1);
  });

  it("does NOT call onCancel when the click originated inside the card (target !== currentTarget)", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onCancel: () => calls++ }) as ElementNode;
    tree.props!.onClick({ target: {}, currentTarget: {} });
    expect(calls).toBe(0);
  });

  it("auto-focuses the Cancel button (default focus lands on the non-destructive action)", () => {
    const tree = ConfirmDialog(baseProps);
    const buttons = findAllButtons(tree);
    const cancelBtn = buttons.find((b) => visibleText(b) === "Cancel");
    expect(cancelBtn?.props?.autoFocus).toBe(true);
  });

  it("defaults to fail (fault-colour) tone on the confirm button", () => {
    const tree = ConfirmDialog(baseProps);
    const buttons = findAllButtons(tree);
    const confirmBtn = buttons.find((b) => visibleText(b) === "Confirm" && b.props?.autoFocus !== true);
    expect(confirmBtn?.props?.className).toContain("bg-statusFailSolid");
  });

  it("renders a warn (amber) tone confirm button when tone=\"warn\" (RestoreCancelButton's light-warning branch)", () => {
    const tree = ConfirmDialog({ ...baseProps, tone: "warn" });
    const buttons = findAllButtons(tree);
    const confirmBtn = buttons.find((b) => visibleText(b) === "Confirm" && b.props?.autoFocus !== true);
    expect(confirmBtn?.props?.className).toContain("bg-statusWarnSolid");
    expect(confirmBtn?.props?.className).not.toContain("bg-statusFailSolid");
  });

  it("labels the dialog via aria-labelledby pointing at the title heading's id", () => {
    const tree = ConfirmDialog(baseProps) as ElementNode;
    const dialogs = findAll(tree, (n) => n.props?.role === "dialog");
    const labelledby = dialogs[0].props?.["aria-labelledby"] as string;
    const headings = findAll(tree, (n) => n.type === "h2" && n.props?.id === labelledby);
    expect(headings.length).toBe(1);
    expect(visibleText(headings[0])).toBe("Confirm");
  });

  it("marks the dialog aria-modal", () => {
    const tree = ConfirmDialog(baseProps) as ElementNode;
    const dialogs = findAll(tree, (n) => n.props?.role === "dialog");
    expect(dialogs[0].props?.["aria-modal"]).toBe("true");
  });

  it("describes the dialog via aria-describedby pointing at the message paragraph's id (the stake-bearing text is announced, not just shown)", () => {
    const tree = ConfirmDialog(baseProps) as ElementNode;
    const dialogs = findAll(tree, (n) => n.props?.role === "dialog");
    const describedby = dialogs[0].props?.["aria-describedby"] as string;
    expect(describedby).toBeTruthy();
    const described = findAll(tree, (n) => n.type === "p" && n.props?.id === describedby);
    expect(described.length).toBe(1);
    expect(visibleText(described[0])).toBe(baseProps.message);
  });
});
