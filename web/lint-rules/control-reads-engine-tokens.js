// ---------------------------------------------------------------------------
// control-reads-engine-tokens — settled convention 1.
//
// "Everything interactive is in the colour and shape engines."
//
// The shape engine's contract, from index.css: "every rounded corner reads
// --radius-card/--radius-control/--radius-pill, nothing hard-codes a literal
// radius, so switching the attribute recolors". One control that spells its
// radius out breaks that silently — it simply keeps its corners while the rest
// of the app squares off, and nobody notices until someone switches to
// `square` mode and looks carefully. That is exactly how the accent and rainbow
// palette swatches stayed perfect circles in every shape (Settings.tsx records
// the fix: "the one pair of controls on this page that silently ignored the
// shape engine").
//
// The same applies to colour: an accent written as a hex, or as Tailwind's own
// amber/yellow palette, is an accent that never follows the user's chosen one.
//
// So, on interactive elements only:
//
//   RADIUS   only `rounded-card` / `rounded-control` / `rounded-pill`
//            (the three shape-engine tokens) and `rounded-none`. Not
//            `rounded-full`, `rounded-lg`, `rounded-[10px]`, or bare `rounded`.
//
//   COLOUR   no literal `#rrggbb` / `rgb()` / `hsl()` in `style`, no
//            `bg-[#…]`-style arbitrary colour, no `*-amber-N`/`*-yellow-N`.
//            `var(--token)` is the whole point and is always fine — including
//            `var(--heat-ok-1, #a7f0ba)`, where the hex is the token's own
//            fallback rather than a second source of truth.
//
// Non-interactive elements are untouched on purpose. Every `rounded-full` in
// the tree today is a spinner ring or a 6px status dot, and those ARE circles
// by definition, not controls whose corners should follow a preference.
// ---------------------------------------------------------------------------
import {
  baseUtility,
  classTokens,
  escapeHatch,
  getAttr,
  hasException,
  isInteractive,
  jsxName,
} from "./helpers.js";

const RULE_ID = "control-reads-engine-tokens";

/** The shape engine's three tokens (index.css @theme --radius-*), plus none. */
const SHAPE_TOKEN_RADII = new Set([
  "rounded-card",
  "rounded-control",
  "rounded-pill",
  "rounded-none",
]);

const ANY_RADIUS = /^rounded(?:-|$)/;

/** `bg-[#0af]`, `text-[rgb(1,2,3)]`, `border-[hsl(…)]` — an inlined colour. */
const ARBITRARY_COLOUR = /^[a-z-]+-\[(#[0-9a-fA-F]{3,8}|rgba?\(|hsla?\()/;

/** Tailwind's own palette standing in for the accent. */
const PALETTE_ACCENT = /^(?:bg|text|border|ring|outline|fill|stroke)-(?:amber|yellow)-\d{2,3}$/;

/** CSS properties in an inline `style` that must come from a token. */
const COLOUR_PROPS = new Set([
  "color",
  "background",
  "backgroundColor",
  "borderColor",
  "borderTopColor",
  "borderRightColor",
  "borderBottomColor",
  "borderLeftColor",
  "outlineColor",
  "fill",
  "stroke",
  "accentColor",
  "caretColor",
  "boxShadow",
]);

const LITERAL_COLOUR = /#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(/;

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "An interactive control reads its radius from the shape engine and its colour from the colour engine, instead of hardcoding either.",
    },
    schema: [],
    messages: {
      radius:
        "`{{token}}` pins this control's corner radius, so it stops following the shape engine — it will keep these corners in `soft` and `square` mode while everything around it changes. Use `rounded-control` (or `rounded-card` / `rounded-pill`), which read --radius-* from index.css.{{hatch}}",
      radiusInline:
        "`borderRadius` in an inline style pins this control's corners outside the shape engine. Use the `rounded-control` / `rounded-card` / `rounded-pill` classes, or `var(--radius-control)` if it really has to be inline.{{hatch}}",
      colourClass:
        "`{{token}}` hardcodes a colour on a control instead of reading the colour engine. Use the accent tokens (`bg-accent`, `text-accentContrast`, `bg-accentSoft`, `text-accentText`) or the carbon surface tokens, so this control follows the user's accent and rainbow.{{hatch}}",
      colourInline:
        "`{{prop}}: {{value}}` hardcodes a colour on a control. Inline colours here must read a token — `var(--accent)`, `var(--accent-contrast)`, `var(--carbon-border)` — so the control follows the colour engine.{{hatch}}",
    },
  },

  create(context) {
    function checkStyle(node, opening) {
      const attr = getAttr(node, "style");
      const v = attr?.value;
      if (!v || v.type !== "JSXExpressionContainer") return;
      const objects = [];
      (function collect(e) {
        if (!e || typeof e !== "object") return;
        if (e.type === "ObjectExpression") objects.push(e);
        if (e.type === "ConditionalExpression") {
          collect(e.consequent);
          collect(e.alternate);
        }
        if (e.type === "TSAsExpression" || e.type === "TSSatisfiesExpression") collect(e.expression);
        if (e.type === "LogicalExpression") collect(e.right);
      })(v.expression);

      for (const obj of objects) {
        for (const prop of obj.properties) {
          if (prop.type !== "Property") continue;
          const key =
            prop.key.type === "Identifier"
              ? prop.key.name
              : prop.key.type === "Literal"
                ? String(prop.key.value)
                : "";
          const val = prop.value;
          if (val.type !== "Literal" || typeof val.value !== "string") continue;

          if (key === "borderRadius" && !val.value.includes("var(--radius")) {
            if (hasException(context, opening, RULE_ID)) return;
            context.report({
              node: prop,
              messageId: "radiusInline",
              data: { hatch: escapeHatch(RULE_ID) },
            });
            continue;
          }
          if (!COLOUR_PROPS.has(key)) continue;
          if (!LITERAL_COLOUR.test(val.value)) continue;
          // `var(--heat-ok-1, #a7f0ba)`: the hex is the token's fallback, not
          // a competing source of truth. Only a colour written OUTSIDE a
          // var() is a hardcoded colour.
          const outsideVar = val.value.replace(/var\([^)]*\)/g, "");
          if (!LITERAL_COLOUR.test(outsideVar)) continue;
          if (hasException(context, opening, RULE_ID)) return;
          context.report({
            node: prop,
            messageId: "colourInline",
            data: { prop: key, value: val.value, hatch: escapeHatch(RULE_ID) },
          });
        }
      }
    }

    return {
      JSXElement(node) {
        if (!isInteractive(node)) return;
        const opening = node.openingElement;

        // Badge/IconTipButton own their own radius from the shape engine
        // already; a className on them is checked by one-icon-badge-size.
        const name = jsxName(node);
        const ownsItsChrome = name === "Badge" || name === "IconTipButton";

        for (const raw of classTokens(node)) {
          const token = baseUtility(raw);
          if (!ownsItsChrome && ANY_RADIUS.test(token) && !SHAPE_TOKEN_RADII.has(token)) {
            if (hasException(context, opening, RULE_ID)) return;
            context.report({
              node: getAttr(node, "className") ?? opening,
              messageId: "radius",
              data: { token: raw, hatch: escapeHatch(RULE_ID) },
            });
            return;
          }
          if (ARBITRARY_COLOUR.test(token) || PALETTE_ACCENT.test(token)) {
            if (hasException(context, opening, RULE_ID)) return;
            context.report({
              node: getAttr(node, "className") ?? opening,
              messageId: "colourClass",
              data: { token: raw, hatch: escapeHatch(RULE_ID) },
            });
            return;
          }
        }

        checkStyle(node, opening);
      },
    };
  },
};
