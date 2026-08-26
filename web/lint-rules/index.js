// ---------------------------------------------------------------------------
// bombvault/lint-rules — the settled UI conventions, as lint rules.
//
// Why these live in ESLint rather than in a test or a shell script:
//
//   * They are all statements about the SHAPE of a JSX call site, and ESLint
//     is the tool this repo already runs that has a parser. The alternative —
//     grepping — was tried: commit d336e532 swept the bespoke-red destructive
//     controls by "grepping the whole tree for statusFail on an interactive
//     element", and missed four controls that carried the identical red
//     through `tone="fail"` instead of a class. A grep also cannot tell a call
//     site from the enormous comment blocks in this codebase, which quote
//     `shape="square"`, `tone="fail"` and `rounded-full` dozens of times while
//     explaining why a past round did or did not use them.
//   * `npm run lint` already runs in .github/workflows/lint.yml. A convention
//     enforced there is enforced on every push and every PR with no new job,
//     no new tool, and no new install.
//   * A violation gets a file, a line, a column and an editor squiggle in the
//     same place the author is typing, which is the only kind of feedback that
//     changes behaviour before a reviewer has to.
//   * Suppression is a first-class, auditable act. Each rule takes a
//     `bv-convention-exception: <rule> -- <reason>` marker comment whose reason
//     text is mandatory (helpers.js's hasException rejects a shrug), which names
//     the rule it suppresses and must sit within eight lines above the element.
//     Every exception in the app is therefore one command away, reasoning
//     attached: `grep -rn "bv-convention-exception" web/src`.
//
//     What the marker does NOT get, said plainly because this header used to
//     claim otherwise: `reportUnusedDisableDirectives: "error"` does not cover
//     it. That setting governs real `eslint-disable` directives, which ESLint
//     parses and tracks. The marker is an ordinary comment matched by a regex in
//     helpers.js, and ESLint has no idea it is meant to mean anything — so a
//     marker left behind after the element it excused stopped violating the rule
//     goes on quietly exempting whatever else lands in its eight-line window,
//     and the build says nothing. The grep is the audit; keeping the count low
//     is what keeps the audit cheap. (lint-rules/README.md scopes the same
//     setting correctly — this header was the one that overstated it.)
//
// The one convention that is NOT here is "explanations live in an InfoBubble":
// see lint-rules/README.md for the measurements behind that decision.
// ---------------------------------------------------------------------------
import controlReadsEngineTokens from "./control-reads-engine-tokens.js";
import iconBadgeNeedsTooltip from "./icon-badge-needs-tooltip.js";
import noStatusColorOnControl from "./no-status-color-on-control.js";
import oneIconBadgeSize from "./one-icon-badge-size.js";
import pageUsesPageShell from "./page-uses-page-shell.js";

export default {
  meta: { name: "bombvault", version: "1.0.0" },
  rules: {
    "control-reads-engine-tokens": controlReadsEngineTokens,
    "icon-badge-needs-tooltip": iconBadgeNeedsTooltip,
    "no-status-color-on-control": noStatusColorOnControl,
    "one-icon-badge-size": oneIconBadgeSize,
    "page-uses-page-shell": pageUsesPageShell,
  },
};
