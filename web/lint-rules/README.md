# `lint-rules/` — the settled UI conventions, enforced

A handful of house rules in this app have been broken, reported by jdp, fixed,
and then broken again by a later round. Not because anyone disagreed with them
— every one was accepted immediately — but because they lived only in prose and
in a reviewer's memory, and neither of those runs in CI.

These are those rules, written once, as ESLint rules.

## Where they run

They are ordinary rules in `web/eslint.config.js`, so `npm run lint` runs them
locally and `.github/workflows/lint.yml` runs the same command on every push
and pull request. No new tool, no new job, no new install.

Their own tests are `web/src/lib/uiConventions.test.ts`, in the existing vitest
suite that `npm test` and the same workflow already run. Every rule has invalid
cases proving it fires, and — more importantly — valid cases pinning the real
call sites it must NOT fire on, each named after the file it came from.

`web/src/app/routedPages.test.ts` guards the one thing a lint rule cannot see
about itself: that `page-uses-page-shell`'s scope still covers every page the
router actually routes to.

## Why ESLint and not a script

* Every rule is a statement about the shape of a JSX call site, and ESLint is
  the tool this repo already runs that has a parser.
* Grepping was tried. Commit `d336e532` swept the bespoke-red destructive
  controls by "grepping the whole tree for `statusFail` on an interactive
  element", and walked past four controls carrying the identical red through
  `tone="fail"` instead of a class. Same treatment, different spelling,
  invisible to the tool. `no-status-color-on-control` found all four on its
  first run.
* A grep also cannot tell a call site from a paragraph about a call site, and
  the comment blocks in this codebase are longer than its code. `shape="square"`,
  `tone="fail"`, `size="icon"` and `rounded-full` each appear dozens of times
  inside prose explaining why some past round did or did not use them.
* A violation arrives as a squiggle on the line being typed, which is the only
  feedback that changes anything before a reviewer has to.

## The escape hatch

A house convention occasionally has a real exception. Instead of a bare
`eslint-disable` — which says nothing about why and is invisible to anyone
auditing the conventions — every rule honours a marker comment placed directly
above the offending element:

```jsx
{/* bv-convention-exception: control-reads-engine-tokens -- an 11px heat-map
    cell; the shape engine's 10px control radius turns it into a disc and the
    grid stops reading as a grid. */}
<button className="w-[11px] h-[11px] rounded-xs" />
```

* The rule name must match — a marker for one rule never silences another.
* The reason is mandatory (at least 12 characters). `-- no` is not a reason.
* The marker must end within 8 lines above the element it excuses, so one
  marker cannot quietly cover a whole file.
* Consecutive `//` lines count as one block, so a reason may span lines.

Every exception in the app is one command away:

```
grep -rn "bv-convention-exception" web/src
```

There is exactly one today (`pages/Dashboard.tsx`, the heat-map cell).

`reportUnusedDisableDirectives: "error"` is already on in this config, so an
`eslint-disable` that stops being needed fails the build rather than rotting.

## The rules

| Rule | Enforces |
| --- | --- |
| `icon-badge-needs-tooltip` | An icon-only Badge carries a real `tip` bubble; no icon-only control explains itself with the native `title` balloon. |
| `one-icon-badge-size` | Every square icon badge is the one canonical 32px stage, not re-sized by prop, by className, or by hand-rolling the tile. |
| `page-uses-page-shell` | A routed page's root is `PAGE_SHELL`; nothing hand-rolls the shell in literal width/gap classes. |
| `no-status-color-on-control` | Status green/amber/red is a readout, never control chrome — and a destructive action gets no bespoke red either. |
| `control-reads-engine-tokens` | An interactive control reads its radius from the shape engine and its colour from the colour engine. |

Each rule file opens with the history that made it necessary; read that before
changing one.

### Calibration notes

These are the shapes that made a first draft noisy, and are now pinned as
`valid` test cases so they stay quiet:

* **`icon-badge-needs-tooltip`** does not require a tooltip on a plain
  icon-only `<button>` that already has an `aria-label`. That set is the app's
  structural affordances — a dialog's close ×, a tree disclosure chevron,
  RevealInput's 15px eye, and `Toggle`'s own switch — not labels someone
  dropped, and no mechanical signal separates the two. `Toggle` is the proof
  that guessing would be actively wrong: the commit immediately before these
  rules landed *removed* its balloon on purpose. What the rule does check
  instead is unambiguous: an icon-only **Badge** with no tooltip at all, and
  **any** icon-only control using `title`.
* **`page-uses-page-shell`** only treats `max-w-*xl` + `flex-col` + a gap of
  `gap-5` or larger as a hand-rolled shell. A first draft accepted any
  `max-w-*` and any `gap-*` and flagged four small label columns in
  `Settings.tsx` (`flex flex-col gap-1 max-w-40`), which are not pages.
* **`control-reads-engine-tokens`** only looks at interactive elements. Every
  one of the 43 `rounded-full` uses in this tree is a spinner ring or a status
  dot — genuinely circles, not controls whose corners should follow a
  preference — and none of them is flagged.

### Known limits

Every rule reads only values it can *see* in the AST, and treats "cannot read
it" as "not a violation". A prop written as a variable is therefore not
checked: `<Badge shape="square" size={SOME_CONST}>` is skipped rather than
guessed at. This costs nothing today — the only such constant,
`OffsiteTargetsSection`'s `ROW_BADGE_SIZE`, is `"medium"` and is used solely on
text badges — and the alternative (following identifiers across modules) would
mean type-aware linting, which `eslint.config.js` deliberately avoids.

`className` used to be the same, and that limit was the expensive one. A dozen
interactive call sites in this tree keep their class list in a local
(`const inputCls = "rounded-control bg-carbon-surface2 …"` in
`OffsiteTargetsSection`, `OffsiteWizard`, `RestorePanel`, `Containers` and
`Fleet`), so every one of them was compliant by luck and unchecked in fact:
factoring a literal out of JSX switched the guard off, silently. `classTokens`
now follows a `className` identifier to the **`const`** it names in the same
module, when that const has exactly one definition and is never reassigned. A
`let` that is written to, a function parameter, an import — all still decline
and read as unknown, because unknown must never become a guess. The first run
after the change found a real one: `ConfirmDialog`'s destructive confirm button,
whose class list lives in a `CONFIRM_BUTTON_TONE` lookup. It now carries an
explicit `bv-convention-exception` marker instead of being invisible.

Still not followed: an identifier imported from another module, and any value
that does not resolve to a literal. That would need cross-module analysis, which
`eslint.config.js` deliberately avoids.

## What is *not* checked, and why

**"A permanent explanatory paragraph where an InfoBubble belongs."** No rule
ships for this. There is no mechanical signal that separates the violation from
the two things the convention itself exempts, and shipping one anyway would
have meant either 39 false-or-out-of-scope errors on a clean tree or 39
suppressions. A check that noisy gets switched off by the first person it
annoys, which is worse than no check.

Measured, rather than assumed:

* The obvious signal is an explanatory i18n key rendered as element text
  instead of passed to a bubble. Scanning `web/src` for
  `<p className="…">{t("…Hint"|"…Note"|"…Why"|"…Help")}</p>` finds **39**
  paragraphs on the clean tree.
* They are not one category. Some are explanations that should move
  (`receiver.repoLocationHint` under a field). Some are exactly what the
  convention exempts — "Live status readouts and conditional warnings are NOT
  explanations" — such as `files.noPathHint`, which only renders when a preset
  has no path, and `settings.dashTileInstalledHint`. Telling those apart needs
  the meaning of the sentence, which the parser never sees.
* The key name does not disambiguate either: `*Hint` keys are already used for
  bubbles (`<InfoBubble tip={t("source.hint")} />`), for `Card hint=` and
  `ToggleRow hint=` props (which *route to* an InfoBubble and are therefore
  already compliant), for field descriptions, and for conditional warnings.
* A narrower variant — "the same key must not be both bubbled and rendered as
  text" — was also tested, and is not better. Scanning for it turns up **5**
  keys, and **3 of those are the scanner reading a comment**: `files.empty`,
  `fleet.empty` and `receiver.empty` each match only because a `{/* … */}`
  block *describes* the conversion in prose ("The old permanent
  `<p>{t("files.empty")}</p>` … moved verbatim onto the new heading Badge as an
  `onAccent` InfoBubble"). The remaining 2 (`settings.offsiteHint`,
  `settings.limitHint`) are genuine dual uses and both are *deliberate*, with
  the reasoning written at the call site — `Settings.tsx` explains that its
  paragraph "stays a ONE-TIME read rather than repeating verbatim in every new
  Card — shown once, in the first (Containers) Card only". So the narrow
  variant flags two decisions someone already made on purpose and three
  comments. Not shippable either.

The compliant half of this convention *is* mechanically enforced, from the
other direction: `Card`, `ToggleRow` and `StepCard` take a `hint` prop that
renders an `InfoBubble`, so the good path is the easy one.

**Two adjacent things also left alone deliberately**, both for the same "no
reliable signal" reason:

* Whether a toggle's visible label is *good*, only that `Toggle` requires a
  `label` prop — which TypeScript already enforces.
* Whether a `hueIndex` is the caller's correct list position. It is a number;
  nothing in the AST knows which number is right.
