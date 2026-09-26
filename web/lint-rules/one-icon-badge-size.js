// one-icon-badge-size: every square icon badge is 32px, Badge size="icon".
//
// Badges sized by role end up side by side at different sizes, so there is one
// size. Three ways to get it wrong are checked:
//
//   1. `size` on a square icon Badge is anything but "icon".
//   2. A sizing utility in a square icon Badge's `className` (`h-9`, `w-7`,
//      `size-10`, `p-2`, `text-[10px]`). Badge sets height, width, padding and
//      font size together in SIZE_TOKENS, and a className that reaches past it
//      lets them drift apart.
//   3. A hand-rolled square icon tile that never became a Badge: an
//      interactive element with a glyph, no text, and an equal h-N/w-N pair
//      other than h-8/w-8.
//
// Non-square badges (a pill's width follows its text) and anything that
// renders text (a labelled control) are not checked.
import {
  attrStringValue,
  baseUtility,
  classTokens,
  escapeHatch,
  getAttr,
  hasException,
  isIconOnly,
  isInteractive,
  jsxName,
} from "./helpers.js";

const RULE_ID = "one-icon-badge-size";

/** Badge.tsx SIZE_TOKENS.icon. */
const CANONICAL_SIZE = "icon";
/** h-8 / w-8, 32px. */
const CANONICAL_STEP = "8";

/** Utilities that fight Badge's own stage table if they appear on a Badge. */
const SIZING_UTILITY =
  /^(?:h|w|size|min-h|min-w|p|px|py|leading)-(?!full$|fit$|auto$)[^:]+$|^text-\[[^\]]*(?:px|rem)\]$/;

/** The numeric step of `h-8`, `w-12` or `size-9`, else undefined. */
function boxStep(token, prefix) {
  const m = new RegExp(`^${prefix}-(\\d+(?:\\.\\d+)?)$`).exec(token);
  return m ? m[1] : undefined;
}

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "Every square icon-only badge in the app renders at the one canonical 32px stage (Badge size=\"icon\"); nothing re-sizes it from a call site.",
    },
    schema: [],
    messages: {
      wrongSize:
        'This square icon-only Badge asks for size="{{size}}". Square icon badges have one size, size="icon" (32px), whatever the badge does or sits next to. Drop the size prop or set size="icon".{{hatch}}',
      missingSize:
        'This square icon-only Badge does not state its size, so it falls back to the "medium" text-chip stage. Add size="icon", the one square icon badge size (32px).{{hatch}}',
      classNameOverride:
        'This square icon-only Badge re-sizes itself from the call site with `{{token}}`. Badge sets height, width, padding and font size together in SIZE_TOKENS so they cannot drift apart; take `{{token}}` out and let size="icon" decide.{{hatch}}',
      handRolled:
        'This is a hand-rolled {{step}}px square icon control (`{{token}}`). Every square icon badge in the app is a `<Badge shape="square" size="icon">` at 32px; use that instead of a bespoke tile, and it picks up the colour and shape engines as well.{{hatch}}',
    },
  },

  create(context) {
    function isSquareIconBadge(node) {
      return jsxName(node) === "Badge" && attrStringValue(node, "shape") === "square" && isIconOnly(node);
    }

    return {
      JSXElement(node) {
        const opening = node.openingElement;

        if (isSquareIconBadge(node)) {
          if (hasException(context, opening, RULE_ID)) return;

          const sizeAttr = getAttr(node, "size");
          const size = attrStringValue(node, "size");
          if (sizeAttr === undefined) {
            context.report({
              node: opening,
              messageId: "missingSize",
              data: { hatch: escapeHatch(RULE_ID) },
            });
            return;
          }
          // A computed size such as `size={ROW_BADGE_SIZE}` cannot be read
          // and passes.
          if (size !== undefined && size !== CANONICAL_SIZE) {
            context.report({
              node: sizeAttr,
              messageId: "wrongSize",
              data: { size, hatch: escapeHatch(RULE_ID) },
            });
            return;
          }

          const offender = classTokens(node, context)
            .map((t) => baseUtility(t))
            .find((t) => SIZING_UTILITY.test(t));
          if (offender) {
            context.report({
              node: getAttr(node, "className") ?? opening,
              messageId: "classNameOverride",
              data: { token: offender, hatch: escapeHatch(RULE_ID) },
            });
          }
          return;
        }

        // A square icon control that never became a Badge.
        if (jsxName(node) === "Badge") return;
        if (!isInteractive(node) || !isIconOnly(node)) return;

        const tokens = classTokens(node, context).map((t) => baseUtility(t));
        let step;
        let token;
        for (const t of tokens) {
          const s = boxStep(t, "size");
          if (s !== undefined) {
            step = s;
            token = t;
            break;
          }
        }
        if (step === undefined) {
          const h = tokens.find((t) => boxStep(t, "h") !== undefined);
          const w = tokens.find((t) => boxStep(t, "w") !== undefined);
          if (h && w && boxStep(h, "h") === boxStep(w, "w")) {
            step = boxStep(h, "h");
            token = `${h} ${w}`;
          }
        }
        if (step === undefined || step === CANONICAL_STEP) return;
        if (hasException(context, opening, RULE_ID)) return;

        context.report({
          node: opening,
          messageId: "handRolled",
          data: { step: String(Number(step) * 4), token, hatch: escapeHatch(RULE_ID) },
        });
      },
    };
  },
};
