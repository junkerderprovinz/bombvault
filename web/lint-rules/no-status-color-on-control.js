// no-status-color-on-control: status colours (green, amber, red) are readouts
// and stay off controls, and a destructive action gets no red of its own.
//
// The rule reads the element, so it sees both spellings of the same red: a
// `bg-statusFailBg`/`text-statusFail` class and Badge's `tone="fail"`.
//
// The line is readout versus control:
//   * A status surface keeps its colour: a fault callout, a red dot on a poll
//     line, a `<Badge tone="fail">Fehlgeschlagen</Badge>` state chip.
//   * A control (a button, a link, a Badge with `as="button"`, anything with
//     an onClick) takes the same neutral chrome as the controls beside it. Its
//     label names the action, and destructive ones confirm first anyway.
//
// There are no exceptions, ConfirmDialog's commit button included: in a
// confirm dialog the question does the warning, not the button.
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

/** Badge tones that signal a status rather than being chrome. */
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

        const offender = classTokens(node, context).find((t) => STATUS_UTILITY.test(baseUtility(t)));
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
