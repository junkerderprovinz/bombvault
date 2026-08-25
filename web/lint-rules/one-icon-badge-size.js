// ---------------------------------------------------------------------------
// one-icon-badge-size — settled convention 4.
//
// "Square icon badges are ONE size app-wide: 32px, Badge size='icon'. A
// role-based split was tried and rejected."
//
// Badge.tsx's own header block spells this out at length ("ONE SIZE FOR SQUARE
// ICON BADGES — 32px (`size="icon"`, h-8/w-8). Full stop.") because the split
// it warns about had already happened twice: 28px `icon` + 32px `compact` +
// 36px `field`, grouped by what each badge DID rather than by what it looked
// like, which is how a settings row ended up with the reset badge visibly
// bigger than the colour swatches beside it (jdp, twice: "der Reset-Badge ist
// größer als die Farbfelder").
//
// A prose block cannot stop the third attempt. This can. Three ways to get the
// wrong size, all three checked:
//
//   1. `size` on a square icon Badge is anything but "icon".
//   2. A sizing utility in a square icon Badge's `className` overriding the
//      stage table (`h-9`, `w-7`, `size-10`, `p-2`, `text-[10px]`). Badge owns
//      height, width, padding and font-size together — that pairing is the
//      whole point of SIZE_TOKENS, and a className that reaches past it
//      re-creates exactly the px-2/px-1.5/no-size-class drift Badge replaced.
//   3. A hand-rolled square icon tile that never became a Badge at all: an
//      interactive element with a glyph, no text, and an equal h-N/w-N pair
//      that is not the canonical h-8/w-8.
//
// Deliberately NOT checked: non-square badges (a pill chip's width follows its
// text, which is the point), and any element that renders text (that is a
// labelled control, not an icon badge).
// ---------------------------------------------------------------------------
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

/** The one canonical square-icon-badge stage. Badge.tsx SIZE_TOKENS.icon. */
const CANONICAL_SIZE = "icon";
/** …and the px it resolves to, as Tailwind utilities: h-8 / w-8 = 32px. */
const CANONICAL_STEP = "8";

/** Utilities that fight Badge's own stage table if they appear on a Badge. */
const SIZING_UTILITY =
  /^(?:h|w|size|min-h|min-w|p|px|py|leading)-(?!full$|fit$|auto$)[^:]+$|^text-\[[^\]]*(?:px|rem)\]$/;

/** `h-8`, `w-12`, `size-9` → the numeric step, else undefined. */
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
        'This square icon-only Badge asks for size="{{size}}". There is exactly ONE square-icon-badge size in this app — size="icon" (32px) — regardless of what the badge does or what it sits next to (Badge.tsx, "ONE SIZE FOR SQUARE ICON BADGES"). Drop the size prop or set size="icon".{{hatch}}',
      missingSize:
        'This square icon-only Badge does not state its size, so it falls back to the "medium" text-chip stage. Add size="icon" — the one square-icon-badge size (32px).{{hatch}}',
      classNameOverride:
        'This square icon-only Badge re-sizes itself from the call site with `{{token}}`. Badge pins height, width, padding and font-size together in SIZE_TOKENS so they cannot drift apart — take `{{token}}` out and let size="icon" decide.{{hatch}}',
      handRolled:
        'This is a hand-rolled {{step}}px square icon control (`{{token}}`). Every square icon badge in the app is a `<Badge shape="square" size="icon">` at 32px — use that instead of a bespoke tile, so it also picks up the colour and shape engines for free.{{hatch}}',
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
          // A computed size (`size={ROW_BADGE_SIZE}`) is not something this
          // rule can read; the shared constant is the good pattern anyway.
          if (size !== undefined && size !== CANONICAL_SIZE) {
            context.report({
              node: sizeAttr,
              messageId: "wrongSize",
              data: { size, hatch: escapeHatch(RULE_ID) },
            });
            return;
          }

          const offender = classTokens(node)
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

        const tokens = classTokens(node).map((t) => baseUtility(t));
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
