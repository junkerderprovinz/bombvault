// ConfirmDialog is a pure component, so these tests call it as a function and
// inspect the returned element tree without a DOM. Escape, the focus trap, the
// portal and focus return live in lib/useConfirm.tsx.
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
  // A raw <button>, or the shared <Button>, whose node type is the component
  // function because the tree is never rendered.
  return findAll(
    tree,
    (n) => n.type === "button" || (typeof n.type === "function" && n.type.name === "Button")
  );
}

function visibleText(node: unknown): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(visibleText).join("");
  if (!isElementNode(node)) return "";
  // An unrendered <Button> carries its words in the `label` prop.
  if (typeof node.type === "function" && node.type.name === "Button") {
    return String(node.props?.label ?? "");
  }
  if (node.props?.children !== undefined) return visibleText(node.props.children);
  return "";
}

const noop = () => {};

const baseProps = {
  title: "Confirm",
  message: "Delete this backup? This cannot be undone.",
  confirmLabel: "Confirm",
  cancelLabel: "Cancel",
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

  it("shows the message unchanged", () => {
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

  it("calls onCancel when the backdrop itself is clicked", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onCancel: () => calls++ }) as ElementNode;
    const target = {};
    tree.props!.onClick({ target, currentTarget: target });
    expect(calls).toBe(1);
  });

  it("does not call onCancel for a click inside the card", () => {
    let calls = 0;
    const tree = ConfirmDialog({ ...baseProps, onCancel: () => calls++ }) as ElementNode;
    tree.props!.onClick({ target: {}, currentTarget: {} });
    expect(calls).toBe(0);
  });

  it("focuses Cancel, the non-destructive action, on open", () => {
    const tree = ConfirmDialog(baseProps);
    const buttons = findAllButtons(tree);
    const cancelBtn = buttons.find((b) => visibleText(b) === "Cancel");
    expect(cancelBtn?.props?.autoFocus).toBe(true);
  });

  it("paints both answers in the accent, never a status colour", () => {
    // The question states the stakes; a red button on every delete teaches
    // people to read past it.
    const tree = ConfirmDialog(baseProps);
    const buttons = findAllButtons(tree);
    const confirmBtn = buttons.find((b) => visibleText(b) === "Confirm" && b.props?.autoFocus !== true);
    const cancelBtn = buttons.find((b) => b.props?.autoFocus === true);

    expect(confirmBtn?.props?.tone).toBe("accent");
    expect(confirmBtn?.props?.tone).toBe(cancelBtn?.props?.tone);
  });

  it("ignores a tone prop", () => {
    // The props type has no tone; one passed anyway must change nothing.
    const tree = ConfirmDialog({ ...baseProps, ...({ tone: "warn" } as object) });
    const buttons = findAllButtons(tree);
    const confirmBtn = buttons.find((b) => visibleText(b) === "Confirm" && b.props?.autoFocus !== true);
    expect(confirmBtn?.props?.tone).toBe("accent");
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

  it("describes the dialog via aria-describedby pointing at the message paragraph's id", () => {
    const tree = ConfirmDialog(baseProps) as ElementNode;
    const dialogs = findAll(tree, (n) => n.props?.role === "dialog");
    const describedby = dialogs[0].props?.["aria-describedby"] as string;
    expect(describedby).toBeTruthy();
    const described = findAll(tree, (n) => n.type === "p" && n.props?.id === describedby);
    expect(described.length).toBe(1);
    expect(visibleText(described[0])).toBe(baseProps.message);
  });
});

describe("ConfirmDialog layout and optional parts", () => {
  function classNames(node: unknown, out: string[] = []): string[] {
    if (!isElementNode(node)) return out;
    if (Array.isArray(node)) {
      for (const c of node) classNames(c, out);
      return out;
    }
    const cn = node.props?.className;
    if (typeof cn === "string") out.push(cn);
    if (node.props?.children !== undefined) classNames(node.props.children, out);
    return out;
  }

  it("draws no divider under the title and none above the buttons", () => {
    const all = classNames(ConfirmDialog(baseProps)).join(" ");
    expect(all).not.toContain("border-b");
    expect(all).not.toContain("border-t");
  });

  it("offers the two footer answers and no close button in the corner", () => {
    // Counted rather than found by text, since a button with an undefined label
    // has no text but is still there.
    const buttons = findAll(
      ConfirmDialog(baseProps),
      (n) => typeof n.type === "function" && (n.type as { name?: string }).name === "Button",
    );
    expect(buttons).toHaveLength(2);
    expect(buttons.some((b) => b.props?.labelKey === "common.close")).toBe(false);
  });

  it("puts an `extra` control under the message, not inside it", () => {
    const tree = ConfirmDialog({ ...baseProps, extra: "ALSO REMOVE THE FOLDER" });
    expect(visibleText(tree)).toContain("ALSO REMOVE THE FOLDER");
    // The described paragraph stays the message alone.
    const described = findAll(
      tree,
      (n) => n.props?.id === "confirmdialog-message",
    );
    expect(described).toHaveLength(1);
    expect(visibleText(described[0].props?.children)).not.toContain("ALSO REMOVE THE FOLDER");
  });

  it("hands the confirm button an explicit glyph when one is given", () => {
    const glyph = "GLYPH";
    const buttons = findAll(
      ConfirmDialog({ ...baseProps, confirmGlyph: glyph }),
      (n) => typeof n.type === "function" && (n.type as { name?: string }).name === "Button",
    );
    const confirm = buttons.find((b) => b.props?.label === "Confirm");
    expect(confirm?.props?.glyph).toBe(glyph);
  });

  it("leaves the glyph undefined when the caller gives none, so the key still decides", () => {
    const buttons = findAll(
      ConfirmDialog(baseProps),
      (n) => typeof n.type === "function" && (n.type as { name?: string }).name === "Button",
    );
    const confirm = buttons.find((b) => b.props?.label === "Confirm");
    expect(confirm?.props?.glyph).toBeUndefined();
  });
});
