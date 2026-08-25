// ---------------------------------------------------------------------------
// icon-badge-needs-tooltip — settled convention 3.
//
// "An icon-only badge automatically gets colour/shape-engine integration AND a
// hover tooltip carrying the label that disappeared."
//
// Every round that converted a text button into a square glyph badge had to be
// told again that the words did not just vanish, they moved into a bubble.
// IconTipButton.tsx exists because one of those conversions reached for the
// native `title=` balloon instead (jdp, live review: "beim Ordnersymbol ist die
// Hover-Infobubble nicht im GlimStone"), and the two most recent commits on
// this branch are still that same fix on two more controls. This rule is that
// review comment, written down once.
//
// WHAT COUNTS AS ICON-ONLY is derived from what the element RENDERS — at least
// one child element and no text anywhere in its subtree — not from a prop the
// author could forget. So `<Badge shape="square"><IconTrash /></Badge>` is
// icon-only whether or not anyone remembered to say so.
//
// WHAT COUNTS AS A TOOLTIP is `tip` — the prop that routes a Badge through
// IconTipButton and a Selector segment through its own bubble, i.e. the real
// `.glim-bubble`. `title` is deliberately NOT accepted: it is the native OS
// balloon this app has spent three commits removing, it never appears on
// keyboard focus, and IconTipButton.tsx's own header calls it "exactly the
// anti-pattern that file exists to replace".
//
// TWO CASES, and the boundary between them is where this rule earns its keep:
//
//   1. An icon-only BADGE has no tooltip at all. This is the convention
//      verbatim — a Badge is the thing a labelled text button gets converted
//      INTO, so a Badge with a glyph and no `tip` is always a label someone
//      dropped on the way.
//
//   2. ANY icon-only control (Badge or plain <button>) explains itself with
//      `title`. The native balloon is unambiguously wrong wherever it appears
//      — no keyboard focus, no GlimStone styling — and needs no judgement
//      about what the control is.
//
// What is deliberately NOT flagged: a plain icon-only <button> that carries an
// `aria-label` and no tooltip. That set is not "labels someone dropped", it is
// the app's structural affordances — a dialog's close ×, a tree disclosure
// chevron, RevealInput's 15px eye, and Toggle's own switch — and no mechanical
// signal separates them from a real action button that lost its words. Toggle
// is the proof that guessing here would be actively wrong: the commit
// immediately before this one REMOVED its balloon on purpose, and a rule
// demanding a tooltip on every icon-only button would demand it straight back.
// See lint-rules/README.md, "What is not checked".
//
// Also skipped: an element that already renders its own <InfoBubble> or
// <IconTipButton> child — the explanation is present, just nested.
// ---------------------------------------------------------------------------
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
          // Case 1 only: an icon-only BADGE with nothing at all. A plain
          // <button> with an aria-label and no balloon is a structural
          // affordance, not a dropped label — see the header.
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
