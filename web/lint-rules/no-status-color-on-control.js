// ---------------------------------------------------------------------------
// no-status-color-on-control — settled convention 5.
//
// "Status colours (green/amber/red) stay OUTSIDE the accent/rainbow engine.
// Destructive actions get no special red badge treatment either."
//
// jdp has now said this three separate ways on three separate controls:
//   * "Der Löschen-Badge ist auch anders eingefärbt, soll nicht so sein, ganz
//     normal in die Farbmodi integrieren."  (RestorePanel's delete badge)
//   * "Keine Sonderfarbe für den Entfernen-Badge."
//   * and the same for "Deaktivieren" buttons.
//
// Commit d336e532 then swept eight controls across six files — and found them
// by "grepping the whole tree for statusFail on an interactive element". That
// grep could only see Tailwind CLASSES, so it swept every hand-written
// `bg-statusFailBg`/`text-statusFail` button and walked straight past four
// controls carrying the identical bespoke red through Badge's own `tone="fail"`
// prop instead. Same treatment, different spelling, invisible to the tool.
//
// This rule sees both spellings, because it reads the element rather than the
// text of the line.
//
// The line it draws is READOUT vs CONTROL, which is the line the convention
// itself draws:
//   * A status SURFACE keeps its colour — a fault callout, a red "·" on a poll
//     line, a `<Badge tone="fail">Fehlgeschlagen</Badge>` state chip. Not
//     interactive, not touched.
//   * A CONTROL — a button, a link, a Badge with `as="button"`, anything with
//     an onClick — takes the same neutral chrome as the controls beside it.
//     The label already names the action, and every one of these routes
//     through a confirm dialog or a two-click inline confirm anyway.
//
// ConfirmDialog's own destructive-variant button is the real exception the
// escape hatch exists for: that dialog IS the status surface, stating severity
// is its whole job. It says so at the call site.
// ---------------------------------------------------------------------------
import {
  attrStringValue,
  baseUtility,
  classTokens,
  escapeHatch,
  getAttr,
  hasException,
  isInteractive,
  jsxName,
} from "./helpers.js";

const RULE_ID = "no-status-color-on-control";

/** Badge tones that are load-bearing STATUS signals rather than chrome. */
const STATUS_TONES = new Set(["fail", "warn", "ok"]);

/** `text-statusFail`, `hover:bg-statusWarnBg`, `border-statusOkSolid`, … */
const STATUS_UTILITY =
  /^(?:bg|text|border|ring|outline|fill|stroke|decoration|divide|shadow|accent|caret)-status(?:Fail|Warn|Ok)[A-Za-z]*$/;

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "An interactive control does not paint itself in a status colour — status green/amber/red is a readout, and a destructive action gets no bespoke red of its own.",
    },
    schema: [],
    messages: {
      tone: 'This is a CONTROL (`{{el}}`), and it paints itself with tone="{{tone}}". A status colour is a readout, not chrome, and a destructive action gets no special red either — use the same tone as the controls it shares a row with (tone="neutral", or tone="active" to join the colour engine). The label and the confirm step already carry the meaning.{{hatch}}',
      utility:
        'This is a CONTROL (`{{el}}`), and `{{token}}` gives it a bespoke status colour. Status green/amber/red is a readout, not control chrome — use the neutral secondary chrome its siblings use (`bg-carbon-surface2` / `text-carbon-text` / `hover:bg-carbon-hover`).{{hatch}}',
    },
  },

  create(context) {
    return {
      JSXElement(node) {
        if (!isInteractive(node)) return;

        const opening = node.openingElement;
        const el = jsxName(node);

        const tone = attrStringValue(node, "tone");
        if (tone !== undefined && STATUS_TONES.has(tone)) {
          if (!hasException(context, opening, RULE_ID)) {
            context.report({
              node: getAttr(node, "tone") ?? opening,
              messageId: "tone",
              data: { el, tone, hatch: escapeHatch(RULE_ID) },
            });
          }
          return;
        }

        const offender = classTokens(node).find((t) => STATUS_UTILITY.test(baseUtility(t)));
        if (!offender) return;
        if (hasException(context, opening, RULE_ID)) return;

        context.report({
          node: getAttr(node, "className") ?? opening,
          messageId: "utility",
          data: { el, token: offender, hatch: escapeHatch(RULE_ID) },
        });
      },
    };
  },
};
