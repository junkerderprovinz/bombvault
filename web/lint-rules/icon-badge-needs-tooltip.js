// icon-badge-needs-tooltip: an icon-only badge carries a hover tooltip with the
// label it does not show.
//
// Icon-only is derived from what the element renders (at least one child
// element and no text in its subtree), not from a prop, so
// `<Badge shape="square"><IconTrash /></Badge>` counts whether or not anyone
// said so.
//
// The tooltip is `tip`, which routes a Badge through IconTipButton and a
// Selector segment through its own bubble, the real `.glim-bubble`. `title`
// does not count: it is the native OS balloon, which never appears on
// keyboard focus and ignores the GlimStone styling.
//
// Two cases are reported:
//
//   1. An icon-only Badge with no tooltip at all. A Badge is what a labelled
//      button gets converted into, so a glyph Badge without `tip` is a label
//      dropped on the way.
//   2. Any icon-only control, Badge or plain <button>, that uses `title`.
//
// A plain icon-only <button> with an `aria-label` and no tooltip is not
// reported. Those are structural affordances (a dialog's close button, a tree
// chevron, RevealInput's eye, Toggle's switch), and nothing mechanical tells
// them apart from an action button that lost its words. See
// lint-rules/README.md.
//
// An element that renders its own <InfoBubble> or <IconTipButton> child is
// skipped too: the explanation is there, just nested.
import {
  attrStringValue,
  escapeHatch,
  getAttr,
  hasAttr,
  hasException,
  isIconOnly,
  isInteractive,
  jsxName,
} from "./helpers.js";

const RULE_ID = "icon-badge-needs-tooltip";

/** The two Badge shapes that are an icon silhouette rather than a text chip. */
const ICON_SHAPES = new Set(["square", "circle"]);

/** Components that carry their own tooltip and satisfy the convention. */
const SELF_EXPLAINING = new Set(["InfoBubble", "IconTipButton", "Tooltip"]);

function rendersOwnBubble(node) {
  const stack = [...(node.children ?? [])];
  while (stack.length) {
    const c = stack.pop();
    if (!c || typeof c !== "object") continue;
    if (c.type === "JSXElement") {
      if (SELF_EXPLAINING.has(jsxName(c))) return true;
      stack.push(...(c.children ?? []));
    } else if (c.type === "JSXFragment") {
      stack.push(...(c.children ?? []));
    } else if (c.type === "JSXExpressionContainer") {
      const e = c.expression;
      if (e && (e.type === "JSXElement" || e.type === "JSXFragment")) stack.push(e);
      if (e && e.type === "LogicalExpression" && e.right) stack.push(e.right);
      if (e && e.type === "ConditionalExpression") stack.push(e.consequent, e.alternate);
    }
  }
  return false;
}

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "An icon-only badge or icon-only button must carry a real hover tooltip (`tip`), not a native title balloon and not nothing at all.",
    },
    schema: [],
    messages: {
      missing:
        "This {{what}} renders a glyph and no text, so nothing on screen says what it does. Give it `tip={t(\"…\")}` — the same real .glim-bubble every other icon-only control uses (Badge routes `tip` through IconTipButton).{{hatch}}",
      nativeTitle:
        "This {{what}} explains itself with the native `title` balloon. `title` never appears on keyboard focus and is the OS tooltip IconTipButton.tsx exists to replace — use `tip={t(\"…\")}` instead.{{hatch}}",
    },
  },

  create(context) {
    return {
      JSXElement(node) {
        const name = jsxName(node);
        const isBadge = name === "Badge";
        const isPlainButton = name === "button";
        if (!isBadge && !isPlainButton) return; // IconTipButton's own type makes `tip` required

        if (!isIconOnly(node)) return;
        if (!isInteractive(node) && !isBadge) return;
        if (rendersOwnBubble(node)) return;
        if (hasAttr(node, "tip")) return;

        const opening = node.openingElement;
        const titleAttr = getAttr(node, "title");

        if (titleAttr === undefined) {
          // Without a title only a Badge is reported (case 1).
          if (!isBadge) return;
          // A Badge in a text-chip silhouette that happens to hold a glyph (a
          // "✓ Proven" pill whose tick is an element) is not an icon badge.
          const shape = attrStringValue(node, "shape");
          if (shape !== undefined && !ICON_SHAPES.has(shape)) return;
          // A non-interactive glyph chip with no shape stated is a decoration.
          if (shape === undefined && !isInteractive(node)) return;
        }

        if (hasException(context, opening, RULE_ID)) return;

        context.report({
          node: titleAttr ?? opening,
          messageId: titleAttr ? "nativeTitle" : "missing",
          data: {
            what: isBadge ? "icon-only Badge" : "icon-only <button>",
            hatch: escapeHatch(RULE_ID),
          },
        });
      },
    };
  },
};
