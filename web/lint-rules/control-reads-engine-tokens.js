// control-reads-engine-tokens: an interactive element takes its corner radius
// from the shape engine and its colour from the colour engine.
//
// The shape engine works because every rounded corner reads
// --radius-card/--radius-control/--radius-pill, so a control that spells its
// radius out keeps its corners when the user switches to `soft` or `square`.
// An accent written as a hex or in Tailwind's amber/yellow palette likewise
// ignores the user's accent.
//
// On interactive elements only:
//
//   radius  `rounded-card`, `rounded-control`, `rounded-pill` or
//           `rounded-none`; not `rounded-full`, `rounded-lg`, `rounded-[10px]`
//           or a bare `rounded`.
//   colour  no literal `#rrggbb`, `rgb()` or `hsl()` in `style`, no arbitrary
//           colour such as `bg-[#…]`, no `*-amber-N` or `*-yellow-N`. A
//           `var(--token)` is fine, including `var(--heat-ok-1, #a7f0ba)`,
//           where the hex is only the token's fallback.
//
// Non-interactive elements are left alone: the `rounded-full` ones are spinner
// rings and status dots, which are circles by definition.
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

/** An inlined colour: `bg-[#0af]`, `text-[rgb(1,2,3)]`, `border-[hsl(…)]`. */
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
          // A hex inside var() is the token's fallback; only a colour written
          // outside var() is hardcoded.
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

        // Badge and IconTipButton take their radius from the shape engine
        // themselves; one-icon-badge-size checks their className.
        const name = jsxName(node);
        const ownsItsChrome = name === "Badge" || name === "IconTipButton";

        for (const raw of classTokens(node, context)) {
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
