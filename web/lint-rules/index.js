// bombvault/lint-rules: the house UI conventions as ESLint rules.
//
// Each rule is a statement about the shape of a JSX call site, which takes a
// parser: a text search cannot tell a call site from a comment that quotes it,
// and cannot see that `tone="fail"` and a statusFail class are the same red.
// `npm run lint` runs in CI, and a violation shows up where the author is
// typing.
//
// A real exception is marked with a `bv-convention-exception: <rule> -- <why>`
// comment ending at most eight lines above the element, and the reason is
// mandatory (see hasException in helpers.js). `grep -rn
// "bv-convention-exception" web/src` lists them all. ESLint does not know the
// marker, so reportUnusedDisableDirectives does not catch a stale one: a marker
// left behind keeps exempting whatever lands in its window.
//
// "Explanations live in an InfoBubble" is not a rule; lint-rules/README.md has
// the measurements behind that.
import controlReadsEngineTokens from "./control-reads-engine-tokens.js";
import iconBadgeNeedsTooltip from "./icon-badge-needs-tooltip.js";
import noEmDashInUserText from "./no-em-dash-in-user-text.js";
import noStatusColorOnControl from "./no-status-color-on-control.js";
import oneIconBadgeSize from "./one-icon-badge-size.js";
import pageUsesPageShell from "./page-uses-page-shell.js";
import userMessageIsTranslated from "./user-message-is-translated.js";

export default {
  meta: { name: "bombvault", version: "1.0.0" },
  rules: {
    "control-reads-engine-tokens": controlReadsEngineTokens,
    "icon-badge-needs-tooltip": iconBadgeNeedsTooltip,
    "no-em-dash-in-user-text": noEmDashInUserText,
    "no-status-color-on-control": noStatusColorOnControl,
    "one-icon-badge-size": oneIconBadgeSize,
    "page-uses-page-shell": pageUsesPageShell,
    "user-message-is-translated": userMessageIsTranslated,
  },
};
