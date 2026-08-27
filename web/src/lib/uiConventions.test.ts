// ---------------------------------------------------------------------------
// The settled UI conventions — rule tests.
//
// web/lint-rules/ turns seven house conventions into lint errors. This file is
// the other half of that: proof that each rule FIRES on the violation it
// targets, and proof that it stays silent on the shapes in the real tree that
// merely look similar.
//
// "A check that has never been seen to fail is not known to work" — so every
// rule here has at least one `invalid` case, and every rule that had to be
// calibrated against a real false positive has that exact false positive
// pinned as a `valid` case, named after the file it came from. Those valid
// cases are the load-bearing half: a rule that starts flagging Toggle's switch
// or the Dashboard heat-map legend again will fail here rather than annoy
// whoever hits it next and get switched off.
//
// espree with `ecmaFeatures.jsx` rather than typescript-eslint's parser: the
// rules only read JSXElement / JSXAttribute / JSXText nodes, which both
// parsers produce identically, and espree needs none of eslint.config.js's
// side-by-side TypeScript 6 shim to run under vitest.
// ---------------------------------------------------------------------------
import { RuleTester } from "eslint";
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import plugin from "../../lint-rules/index.js";

// Report each RuleTester case as its own vitest test.
(RuleTester as unknown as { describe: unknown; it: unknown }).describe = describe;
(RuleTester as unknown as { describe: unknown; it: unknown }).it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const rules = (plugin as any).rules;

const PAGE = "/repo/web/src/pages/Fleet.tsx";
const SETTINGS = "/repo/web/src/pages/Settings.tsx";
const LOGIN = "/repo/web/src/pages/Login.tsx";
const COMPONENT = "/repo/web/src/components/Widget.tsx";

// ---------------------------------------------------------------------------
// 1. An icon-only badge without a tooltip.
// ---------------------------------------------------------------------------
ruleTester.run("icon-badge-needs-tooltip", rules["icon-badge-needs-tooltip"], {
  valid: [
    // The good shape: every square icon badge in the app looks like this.
    `<Badge as="button" shape="square" size="icon" tone="active" tip={t("snapshots.delete")}><IconTrash /></Badge>`,
    // A text chip that happens to contain a glyph is not an icon badge.
    `<Badge tone="ok" shape="pill">{"\\u2713 "}{t("drill.proven")}</Badge>`,
    // Text through an expression is still text.
    `<Badge as="button" shape="square">{t("offsite.targets.edit")}</Badge>`,
    // REAL TREE, components/Toggle.tsx: the switch is icon-only with an
    // aria-label and NO balloon, on purpose — commit 53a1931e removed its
    // native title, and its visible label sits beside it (convention 2).
    // A rule that demanded a tooltip here would undo that commit.
    `<button type="button" role="switch" aria-checked={checked} aria-label={label} onClick={f}><span className="knob" /></button>`,
    // REAL TREE, components/Toast.tsx / ConfirmDialog.tsx / WhatsNewDialog.tsx:
    // a dialog close X, aria-labelled, no balloon.
    `<button type="button" onClick={onClose} aria-label={t("common.close")}><svg /></button>`,
    // REAL TREE, components/SnapshotFileTree.tsx: a disclosure chevron.
    `<button onClick={toggle} aria-label={expanded ? "collapse" : "expand"}><svg /></button>`,
    // A Badge whose explanation is nested rather than passed as a prop.
    `<Badge shape="square"><InfoBubble tip={t("x")} /><IconKey /></Badge>`,
    // The escape hatch, with a reason.
    `
      {/* bv-convention-exception: icon-badge-needs-tooltip -- purely
          decorative rank medal, it is not a control and has no action. */}
      <Badge shape="circle"><IconMedal /></Badge>
    `,
  ],
  invalid: [
    {
      // The regression this rule exists for: a text button converted to a
      // glyph badge, and the words simply vanished.
      code: `<Badge as="button" shape="square" size="icon" onClick={f}><IconTrash /></Badge>`,
      errors: [{ messageId: "missing" }],
    },
    {
      code: `<Badge shape="circle" as="button" onClick={f}><IconGear /></Badge>`,
      errors: [{ messageId: "missing" }],
    },
    {
      // The native OS balloon — wrong on a Badge…
      code: `<Badge as="button" shape="square" size="icon" title={t("x")} onClick={f}><IconPencil /></Badge>`,
      errors: [{ messageId: "nativeTitle" }],
    },
    {
      // …and wrong on a plain icon-only <button>, which is what the nine
      // reorder-arrow / dashboard-card controls in this tree were doing.
      code: `<button onClick={f} aria-label={t("backupOrder.moveUp")} title={t("backupOrder.moveUp")}><svg /></button>`,
      errors: [{ messageId: "nativeTitle" }],
    },
    {
      // A marker with no real reason is not an escape hatch.
      code: `
        {/* bv-convention-exception: icon-badge-needs-tooltip -- no */}
        <Badge as="button" shape="square" onClick={f}><IconTrash /></Badge>
      `,
      errors: [{ messageId: "missing" }],
    },
    {
      // A marker naming a DIFFERENT rule does not silence this one.
      code: `
        {/* bv-convention-exception: one-icon-badge-size -- the wrong rule
            name, so this exception does not apply here at all. */}
        <Badge as="button" shape="square" onClick={f}><IconTrash /></Badge>
      `,
      errors: [{ messageId: "missing" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 2. A square icon badge at any size other than the single canonical one.
// ---------------------------------------------------------------------------
ruleTester.run("one-icon-badge-size", rules["one-icon-badge-size"], {
  valid: [
    `<Badge as="button" shape="square" size="icon" tip={t("x")}><IconTrash /></Badge>`,
    // Non-square badges size themselves to their text; untouched.
    `<Badge tone="neutral" shape="pill" size="small">{t("x")}</Badge>`,
    // A className that is not a sizing utility is fine on an icon badge.
    `<Badge shape="square" size="icon" tip={t("x")} className="glim-shake ms-auto"><IconTrash /></Badge>`,
    // `max-w-full` must not be mistaken for a `w-` utility.
    `<Badge shape="square" size="icon" tip={t("x")} className="max-w-full"><IconTrash /></Badge>`,
    // REAL TREE, the canonical hand-rolled size IS 32px, so h-8/w-8 passes.
    `<IconTipButton tip={t("x")} className="h-8 w-8 rounded-control"><svg /></IconTipButton>`,
    // REAL TREE, components/RevealInput.tsx: a 15px inline eye affordance is
    // an arbitrary-value box, not an h-N/w-N tile, and is not a badge.
    `<button type="button" aria-label={l} onClick={f} className="h-[15px] w-[15px] rounded-pill"><svg /></button>`,
    // REAL TREE, pages/Containers.tsx: the bare "x" remove glyph has no
    // h-/w- pair at all — no size to be wrong.
    `<button onClick={f} aria-label={t("offsite.targets.remove")} className="text-carbon-textMuted px-1"><span /></button>`,
    // A labelled control with a size is not an icon badge.
    `<button className="h-9 w-9" onClick={f}>{t("x")}</button>`,
  ],
  invalid: [
    {
      // The role-based split, trying to come back.
      code: `<Badge as="button" shape="square" size="large" tip={t("x")}><IconTrash /></Badge>`,
      errors: [{ messageId: "wrongSize" }],
    },
    {
      code: `<Badge as="button" shape="square" size="field" tip={t("x")}><IconTrash /></Badge>`,
      errors: [{ messageId: "wrongSize" }],
    },
    {
      // No size at all silently falls back to the "medium" text stage.
      code: `<Badge as="button" shape="square" tip={t("x")}><IconTrash /></Badge>`,
      errors: [{ messageId: "missingSize" }],
    },
    {
      // Right stage, then overridden from the call site anyway.
      code: `<Badge shape="square" size="icon" tip={t("x")} className="h-9 w-9"><IconTrash /></Badge>`,
      errors: [{ messageId: "classNameOverride" }],
    },
    {
      code: `<Badge shape="square" size="icon" tip={t("x")} className={"p-2 " + extra}><IconTrash /></Badge>`,
      errors: [{ messageId: "classNameOverride" }],
    },
    {
      // A square icon control that never became a Badge — 36px, the old
      // "field" stage, hand-rolled.
      code: `<button onClick={f} aria-label={l} className="h-9 w-9 rounded-control"><svg /></button>`,
      errors: [{ messageId: "handRolled" }],
    },
    {
      code: `<IconTipButton tip={t("x")} className="size-7 rounded-control"><svg /></IconTipButton>`,
      errors: [{ messageId: "handRolled" }],
    },
    {
      // The escape hatch is per-rule and needs a reason; this one has both,
      // but the FIRST badge below it is still out of the lookback window.
      code: `
        <Badge shape="square" size="large" tip={t("x")}><IconTrash /></Badge>
        {/* bv-convention-exception: one-icon-badge-size -- a marker BELOW the
            call site is not a marker above it. */}
      `,
      errors: [{ messageId: "wrongSize" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 3. A page component whose root does not use the shared page shell.
// ---------------------------------------------------------------------------
const shellOptions = [
  { exceptions: { "Settings.tsx": "PAGE_SHELL_TABBED", "Login.tsx": null } },
];

ruleTester.run("page-uses-page-shell", rules["page-uses-page-shell"], {
  valid: [
    { code: `export function Fleet() { return <div className={PAGE_SHELL}><h1 /></div>; }`, filename: PAGE, options: shellOptions },
    // An early non-JSX return alongside the real root is fine.
    {
      code: `export function Fleet() { if (!ready) return null; return <div className={PAGE_SHELL}><h1 /></div>; }`,
      filename: PAGE,
      options: shellOptions,
    },
    // A nested row renderer's own return is not the page root.
    {
      code: `export function Fleet() { const row = () => <div className="flex" />; return <div className={PAGE_SHELL}>{row()}</div>; }`,
      filename: PAGE,
      options: shellOptions,
    },
    // The documented exception, allowed BY NAME and only for its own shell.
    { code: `export function SettingsPage() { return <div className={PAGE_SHELL_TABBED}><h1 /></div>; }`, filename: SETTINGS, options: shellOptions },
    // Login is exempt entirely — it never sits under <Outlet />.
    { code: `export function LoginPage() { return <div className="w-full max-w-sm"><h1 /></div>; }`, filename: LOGIN, options: shellOptions },
    // Components outside src/pages are not pages.
    { code: `export function Widget() { return <div className="flex flex-col gap-10 max-w-6xl" />; }`, filename: COMPONENT, options: shellOptions },
    // REAL TREE, pages/Settings.tsx x4: small stacked columns that share the
    // shell's ingredient list at a completely different scale. `max-w-40`
    // and `gap-1` are a label stack, not a page.
    { code: `export function SettingsPage() { return <div className={PAGE_SHELL_TABBED}><div className="flex flex-col gap-1 max-w-40" /></div>; }`, filename: SETTINGS, options: shellOptions },
    { code: `export function SettingsPage() { return <div className={PAGE_SHELL_TABBED}><div className="flex flex-col gap-1 text-xs max-w-xs" /></div>; }`, filename: SETTINGS, options: shellOptions },
  ],
  invalid: [
    {
      // An ARROW with an expression body has no ReturnStatement, so the rule
      // found no returns and skipped the component in silence — a page written
      // this way was never checked at all.
      code: `export const Fleet = () => (<div className="flex flex-col gap-6 max-w-5xl"><h1 /></div>);`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }, { messageId: "handRolled" }],
    },
    {
      // `export default Fleet;` is a bare Identifier, which noteExport dropped —
      // same silent pass, different shape.
      code: `function Fleet() { return <div className="space-y-4"><h1 /></div>; }
export default Fleet;`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }],
    },
    {
      // A VARIANT PREFIX hid the width cap from an ^-anchored pattern, so the
      // second net (a literal shell copy in a nested element) had a hole in it.
      code: `export function Fleet() { return <div className={PAGE_SHELL}><div className="flex flex-col gap-6 md:max-w-6xl"><h1 /></div></div>; }`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "handRolled" }],
    },
    {
      // Page eleven, with its own idea of how wide a page is.
      code: `export function Fleet() { return <div className="flex flex-col gap-6 max-w-5xl"><h1 /></div>; }`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }, { messageId: "handRolled" }],
    },
    {
      // The root simply loses the shell.
      code: `export function Fleet() { return <div className="space-y-4"><h1 /></div>; }`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }],
    },
    {
      // A page helping itself to Settings' exception.
      code: `export function Fleet() { return <div className={PAGE_SHELL_TABBED}><h1 /></div>; }`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }],
    },
    {
      // …and Settings helping itself to something that is neither.
      code: `export function SettingsPage() { return <div className={PAGE_SHELL}><h1 /></div>; }`,
      filename: SETTINGS,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }],
    },
    {
      // A default-exported page (pages/Recovery.tsx's shape).
      code: `export default function Fleet() { return <div className="flex flex-col gap-10 max-w-6xl"><h1 /></div>; }`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "notShelled" }, { messageId: "handRolled" }],
    },
    {
      // A second, literal copy of the shell somewhere inside a shelled page.
      code: `export function Fleet() { return <div className={PAGE_SHELL}><div className="flex flex-col gap-10 max-w-6xl" /></div>; }`,
      filename: PAGE,
      options: shellOptions,
      errors: [{ messageId: "handRolled" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 5. A destructive/delete control carrying a special colour treatment.
// ---------------------------------------------------------------------------
ruleTester.run("no-status-color-on-control", rules["no-status-color-on-control"], {
  valid: [
    // A status READOUT keeps its colour — it is not a control.
    `<Badge tone="fail">{t("receiver.unreachable")}</Badge>`,
    `<Badge tone="warn" wrap title={t("files.noPathHint")}>{t("x")}</Badge>`,
    `<p className="text-statusFail">{err}</p>`,
    `<span className="mt-1 h-2 w-2 rounded-full bg-statusFailSolid" />`,
    // The destructive control, after the fix: same chip as its siblings.
    `<Badge as="button" tone="neutral" onClick={remove}>{t("offsite.targets.remove")}</Badge>`,
    `<button onClick={del} className="rounded-control bg-carbon-surface2 text-carbon-text hover:bg-carbon-hover">{t("x")}</button>`,
    // tone="active" is the accent, not a status colour.
    `<Badge as="button" shape="square" size="icon" tone="active" tip={t("snapshots.delete")}><IconTrash /></Badge>`,
    // The escape hatch: ConfirmDialog's own severity-bearing variant.
    `
      {/* bv-convention-exception: no-status-color-on-control -- this dialog
          IS the status surface; stating the severity is its entire job. */}
      <button onClick={onConfirm} className="bg-statusFailSolid">{t("x")}</button>
    `,
  ],
  invalid: [
    {
      // The four this rule found in the real tree, in their original form.
      code: `<Badge as="button" tone="fail" size="small" onClick={del}>{t("snapshots.deleteAll")}</Badge>`,
      errors: [{ messageId: "tone" }],
    },
    {
      code: `<Badge as="button" tone="fail" onClick={remove}>{t("offsite.targets.remove")}</Badge>`,
      errors: [{ messageId: "tone" }],
    },
    {
      // The spelling commit d336e532's grep COULD see…
      code: `<button onClick={del} className="rounded-control bg-statusFailBg text-statusFail">{t("x")}</button>`,
      errors: [{ messageId: "utility" }],
    },
    {
      // …and a hover-only variant of it, which is just as bespoke.
      code: `<button onClick={del} className="px-1 hover:text-statusFail">{"\\u00d7"}</button>`,
      errors: [{ messageId: "utility" }],
    },
    {
      // Only in one arm of a conditional — still a bespoke red.
      code: `<button onClick={del} className={busy ? "opacity-50" : "text-statusFail"}>{t("x")}</button>`,
      errors: [{ messageId: "utility" }],
    },
    {
      // A link is a control too.
      code: `<a href="/x" className="text-statusWarn">{t("x")}</a>`,
      errors: [{ messageId: "utility" }],
    },
    {
      // Anything wired to a click is a control, whatever tag it wears.
      code: `<div onClick={del} className="bg-statusFailBg">{t("x")}</div>`,
      errors: [{ messageId: "utility" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 6. A control that hardcodes a radius or an accent colour.
// ---------------------------------------------------------------------------
ruleTester.run("control-reads-engine-tokens", rules["control-reads-engine-tokens"], {
  valid: [
    `<button onClick={f} className="rounded-control bg-carbon-surface2">{t("x")}</button>`,
    // A local the rule CAN see, and that is compliant — the widened lookup must
    // not start reporting the eleven call sites it now reads.
    `
      const inputCls = "rounded-control bg-carbon-surface2 px-3";
      export function F() {
        return <input className={inputCls} />;
      }
    `,
    // A local it CANNOT settle on stays unknown, and unknown is never a
    // violation: a rule must not guess at a value it cannot see.
    `
      export function F({ cls }) {
        return <input className={cls} />;
      }
    `,
    `
      let cls = "rounded-control";
      cls = somethingElse();
      export function F() {
        return <input className={cls} />;
      }
    `,
    `<button onClick={f} className="rounded-card">{t("x")}</button>`,
    `<button onClick={f} className="rounded-pill bg-accent text-accentContrast">{t("x")}</button>`,
    // REAL TREE: every rounded-full in this app is a spinner ring or a status
    // dot — non-interactive, genuinely a circle, and left alone.
    `<span className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin" />`,
    `<div className="w-2 h-2 rounded-full bg-statusOkSolid shrink-0" />`,
    // Inline colours that read a token are the whole point.
    `<button onClick={f} style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}>{t("x")}</button>`,
    `<input type="checkbox" style={{ accentColor: "var(--accent)" }} />`,
    // A hex INSIDE var() is that token's own fallback, not a second source.
    `<button onClick={f} style={{ backgroundColor: "var(--heat-ok-1, #a7f0ba)" }}>{t("x")}</button>`,
    // Badge and IconTipButton own their chrome from the engines already.
    `<Badge as="button" shape="pill" onClick={f}>{t("x")}</Badge>`,
    // The escape hatch, with a reason.
    `
      {/* bv-convention-exception: control-reads-engine-tokens -- an 11px
          heat-map cell; the engine's 10px control radius makes it a disc. */}
      <button onClick={f} className="w-[11px] h-[11px] rounded-xs" />
    `,
  ],
  invalid: [
    {
      // The swatch bug, in the form it originally shipped.
      code: `<button onClick={f} className="h-8 w-8 rounded-full border-2">{t("x")}</button>`,
      errors: [{ messageId: "radius" }],
    },
    {
      // THE BLIND SPOT, closed. A class list moved into a local was invisible:
      // the rule read className LITERALS and skipped a bare identifier, so the
      // dozen `const inputCls = "…"` call sites in this tree (five inputs in
      // OffsiteTargetsSection, OffsiteWizard's, two selects in RestorePanel,
      // Containers' two, Fleet's two) were compliant by luck and unchecked in
      // fact. Factoring a literal out must not switch the guard off.
      code: `
        const inputCls = "rounded-lg bg-carbon-surface2 px-3";
        export function F() {
          return <input className={inputCls} />;
        }
      `,
      errors: [{ messageId: "radius" }],
    },
    {
      // …through a template literal that interpolates the local, which is how
      // the real call sites compose a base class list with a state class.
      code: `
        const base = "rounded-lg px-3";
        export function F({ on }) {
          return <select className={\`\${base} \${on ? "opacity-100" : ""}\`}>{o}</select>;
        }
      `,
      errors: [{ messageId: "radius" }],
    },
    {
      code: `<button onClick={f} className="rounded-lg px-3">{t("x")}</button>`,
      errors: [{ messageId: "radius" }],
    },
    {
      code: `<a href="/x" className="rounded-[10px]">{t("x")}</a>`,
      errors: [{ messageId: "radius" }],
    },
    {
      code: `<select className="rounded">{opts}</select>`,
      errors: [{ messageId: "radius" }],
    },
    {
      code: `<button onClick={f} style={{ borderRadius: "8px" }}>{t("x")}</button>`,
      errors: [{ messageId: "radiusInline" }],
    },
    {
      // An accent that never follows the user's accent.
      code: `<button onClick={f} className="bg-[#FCC419] text-carbon-text">{t("x")}</button>`,
      errors: [{ messageId: "colourClass" }],
    },
    {
      code: `<button onClick={f} className="rounded-control bg-amber-400">{t("x")}</button>`,
      errors: [{ messageId: "colourClass" }],
    },
    {
      code: `<button onClick={f} style={{ backgroundColor: "#FCC419" }}>{t("x")}</button>`,
      errors: [{ messageId: "colourInline" }],
    },
    {
      code: `<button onClick={f} style={{ color: "rgb(252, 196, 25)" }}>{t("x")}</button>`,
      errors: [{ messageId: "colourInline" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 7. A user-facing string that never goes through t().
// ---------------------------------------------------------------------------
ruleTester.run("user-message-is-translated", rules["user-message-is-translated"], {
  valid: [
    // The fixed shapes: every message resolved through t() before it is shown.
    `push(t("common.deleteFailed"), "fail");`,
    `setError(res.error ?? t("common.loadBackupsFailed"));`,
    `setError(err instanceof Error ? err.message : t("vms.loadFailed"));`,
    `push(res.error ?? t("common.discoverFailed"), "fail");`,
    // The whole reason for the capital-plus-space test: the tree is full of
    // `??` and `?:` string tails that are values, not sentences. Not one of
    // these may be flagged, or the rule gets disabled and protects nothing.
    `const mode = stored ?? "";`,
    `const kind = res.kind ?? "all";`,
    `const method = vm.method ?? "graceful";`,
    `const cls = open ? "rotate-90" : "rtl:rotate-180";`,
    `const key = sort ?? "name";`,
    `const n = count ?? "0";`,
    // A capital with no space is an identifier, not a sentence.
    `const domain = r.domain ?? "Containers";`,
    // Templates whose STATIC text is punctuation or spacing are formatting, not
    // sentences — the same line looksLikeSentence draws for the rest of the rule.
    "push(`${a}/${b}`, \"info\");",
    "setError(`${n} %`);",
    // Array.prototype.push is not the toast. The toast always takes
    // (message, tone), and requiring that second argument is what keeps every
    // `seen.push(`enter:${id}`)` collector in the test files out of this rule.
    "seen.push(`enter:${id}`);",
    "broken.push(`${key}: want [${want}]`);",
    // The escape hatch, with a real reason.
    `
      // bv-convention-exception: user-message-is-translated -- a developer-only
      // console diagnostic behind an env flag; never rendered in the UI.
      push("Debug probe failed", "fail");
    `,
  ],
  invalid: [
    {
      // `||` is the same fallback written the other way, and only `??` was
      // checked — so this exact line was silent while its `??` twin was
      // reported.
      code: `setError(err || "Failed to load VMs");`,
      errors: [{ messageId: "hardcoded" }],
    },
    {
      // A TEMPLATE literal in a message sink is a string literal too. This is
      // the shape the hard-English VM bulk toast escaped through.
      code: "push(`${ok} ok, ${fail} failed`, \"warn\");",
      errors: [{ messageId: "hardcoded" }],
    },
    {
      // The single most-repeated defect in the sweep: 12 identical call sites.
      code: `push(res.error ?? "Delete failed", "fail");`,
      errors: [{ messageId: "hardcoded" }],
    },
    {
      code: `setError("Failed to load VMs");`,
      errors: [{ messageId: "hardcoded" }],
    },
    {
      // The ternary tail — the shape the grep-based sweep missed six times.
      code: `const msg = err instanceof Error ? err.message : "Check failed";`,
      errors: [{ messageId: "hardcoded" }],
    },
    {
      // Both branches of one ternary, both sentences: two findings, not one.
      code: `const m = isRestore ? "Restore failed" : "Backup failed";`,
      errors: [{ messageId: "hardcoded" }, { messageId: "hardcoded" }],
    },
    {
      // A message sink takes ANY string literal, sentence-shaped or not — its
      // first argument is a user message by construction.
      code: `push("Failed", "fail");`,
      errors: [{ messageId: "hardcoded" }],
    },
    {
      // A marker whose reason is a shrug does not suppress anything.
      code: `
        // bv-convention-exception: user-message-is-translated -- nah
        setError("Failed to load runs");
      `,
      errors: [{ messageId: "hardcoded" }],
    },
    {
      // A marker naming a DIFFERENT rule does not suppress this one either.
      code: `
        // bv-convention-exception: one-icon-badge-size -- this badge is sized
        // by its own container and cannot take the shared stage.
        setError("Failed to load containers");
      `,
      errors: [{ messageId: "hardcoded" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 8. An em dash in text a user reads.
//
// The rule is file-scoped for translation tables, so most cases here carry an
// explicit `filename`. The mark itself is interpolated from one constant
// rather than typed into forty strings: this file is prose ABOUT em dashes as
// much as it is a test of them, and a literal one inside a test string is
// indistinguishable at a glance from a literal one inside a comment.
// ---------------------------------------------------------------------------
const EM = String.fromCharCode(0x2014);
const LOCALE_DE = "/repo/web/src/lib/locales/de.ts";
const LOCALE_JA = "/repo/web/src/lib/locales/ja.ts";
const LOCALE_RU = "/repo/web/src/lib/locales/ru.ts";
const LOCALE_UK = "/repo/web/src/lib/locales/uk.ts";
const LOCALE_BG = "/repo/web/src/lib/locales/bg.ts";
const LOCALE_SR = "/repo/web/src/lib/locales/sr.ts";
const I18N = "/repo/web/src/lib/i18n.ts";

ruleTester.run("no-em-dash-in-user-text", rules["no-em-dash-in-user-text"], {
  valid: [
    // The fixed shapes, one per replacement the sweep actually used: a comma
    // where the dash joined a clause, a full stop where it introduced a
    // standalone explanation, a colon before a definition, a parenthesis
    // around an aside.
    { code: `const de = { "files.hint": "Sichert alles, auch die Konfiguration." };`, filename: LOCALE_DE },
    { code: `const de = { "files.hint": "Sichert alles. Die Konfiguration liegt daneben." };`, filename: LOCALE_DE },
    { code: `const de = { "files.hint": "Zwei Ziele: lokal und offsite." };`, filename: LOCALE_DE },
    { code: `const de = { "files.hint": "Sichert alles (auch die Konfiguration)." };`, filename: LOCALE_DE },
    // CJK takes its own punctuation, never a spaced hyphen.
    { code: `const ja = { "files.hint": "すべて保存します。設定も含みます。" };`, filename: LOCALE_JA },

    // THE EXEMPTION. In ru/uk/bg/sr the em dash stands in for the absent
    // copula and is ordinary punctuation; 561 of these live in the real files
    // and not one is a defect. If this rule ever starts flagging them, the
    // next sweep "fixes" 538 correct sentences into broken ones.
    { code: `const ru = { "x.y": "Мой брат ${EM} врач" };`, filename: LOCALE_RU },
    { code: `const uk = { "x.y": "Це ${EM} важливо" };`, filename: LOCALE_UK },
    { code: `const bg = { "x.y": "Това ${EM} е важно" };`, filename: LOCALE_BG },
    { code: `const sr = { "x.y": "Ово ${EM} је важно" };`, filename: LOCALE_SR },
    // The exemption is by LOCALE, not by script: a Cyrillic string in a file
    // that is not one of the four is still checked (the invalid block proves
    // the other half of this).
    { code: `const sr = { "x.y": "Ово је важно" };`, filename: LOCALE_SR },

    // A COMMENT IS NEVER USER-FACING. This is the load-bearing case: src/**
    // holds 4,708 em dashes and 4,705 of them look exactly like this. A rule
    // that flagged one of them would be switched off the same day.
    {
      code: `
        // Settings ${EM} Security card
        /* Restore from another BombVault repo ${EM} Recovery page (#61) */
        const de = { "settings.security": "Sicherheit" };
      `,
      filename: LOCALE_DE,
    },
    // …including a JSX comment sitting in rendered markup.
    `<p>{/* the label ${EM} which used to be inline ${EM} now comes from t() */}{t("x")}</p>`,

    // An i18n KEY is read by the lookup, never by a person. (No key in this
    // app contains one, but a key is not user-facing on principle, and the
    // rule must not depend on that staying true by luck.)
    { code: `const de = { "a.b${EM}c": "Sicherheit" };`, filename: I18N },

    // Outside the tables, an ordinary string literal is a class list, a path,
    // a storage key or a test fixture. Not checked, and deliberately so.
    `const sep = " ${EM} ";`,
    `expect(render()).toContain("2026 ${EM} ok");`,
    // …not even in a .ts file that merely sits near the tables.
    { code: `export const SEP = "${EM}";`, filename: "/repo/web/src/lib/format.ts" },

    // An attribute that is not one of the four text-bearing ones.
    `<div data-sep="${EM}" className="a ${EM} b" />`,

    // The escape hatch, with a real reason.
    `
      {/* bv-convention-exception: no-em-dash-in-user-text -- the empty-value
          glyph in a numeric cell, not prose; it is the typographic "no value"
          mark and reads wrong as a comma. */}
      <td>${EM}</td>
    `,
  ],
  invalid: [
    {
      // The plain case, in the file where 4,900 of them lived.
      code: `const de = { "files.hint": "Sichert alles ${EM} auch die Konfiguration." };`,
      filename: LOCALE_DE,
      errors: [{ messageId: "inTranslation" }],
    },
    {
      // WITH A PLACEHOLDER. The one thing the fix must never disturb: the
      // sweep's whole failure mode would have been a mangled {path} rendering
      // a broken sentence at runtime, so the rule reports the string that
      // carries one exactly like any other.
      code: `const de = { "files.restored": "{count} Dateien nach {path} ${EM} fertig in {when}." };`,
      filename: LOCALE_DE,
      errors: [{ messageId: "inTranslation" }],
    },
    {
      // i18n.ts holds en and de inline as the source of truth, so it is a
      // translation table too.
      code: `export const en = { "files.hint": "Backs everything up ${EM} configuration included." };`,
      filename: I18N,
      errors: [{ messageId: "inTranslation" }],
    },
    {
      // Written as an escape sequence rather than the character. A grep over
      // the source text misses this one; the rule reads the cooked value.
      code: `const de = { "files.hint": "Sichert alles \\u2014 auch die Konfiguration." };`,
      filename: LOCALE_DE,
      errors: [{ messageId: "inTranslation" }],
    },
    {
      // A template literal's literal chunks are as user-facing as a plain
      // string; the interpolation between them is not the rule's business.
      code: "const de = { \"files.hint\": `Sichert ${n} Dateien \\u2014 auch die Konfiguration.` };",
      filename: LOCALE_DE,
      errors: [{ messageId: "inTranslation" }],
    },
    {
      // A Cyrillic STRING in a locale that is not one of the four exempt ones.
      // The exemption is by locale, not by script.
      code: `const mk = { "x.y": "Тоа ${EM} е важно" };`,
      filename: "/repo/web/src/lib/locales/mk.ts",
      errors: [{ messageId: "inTranslation" }],
    },

    // REAL TREE, the three the locale sweep could not reach: separators
    // hardcoded in components, where no translator can see them.
    {
      // pages/Containers.tsx:667
      code: `<p className="text-xs">{t("containers.updateCheckLabel")}: {relativeTime(t, at)} ${EM} {resultText(t, r)}</p>`,
      errors: [{ messageId: "inJsxText" }],
    },
    {
      // pages/Recovery.tsx:547, inside an <option>.
      code: `<option key={s.id} value={s.id}>{new Date(s.time).toLocaleString()} ${EM} {s.id.slice(0, 8)}</option>`,
      errors: [{ messageId: "inJsxText" }],
    },
    {
      // pages/Recovery.tsx:1356, prefixing an error string.
      code: `<span dir="ltr" className="font-mono break-all"> ${EM} {r.error}</span>`,
      errors: [{ messageId: "inJsxText" }],
    },
    {
      // The entity spelling React decodes at render time. Not in the tree
      // today, and the obvious way around a rule that only knows the
      // character.
      code: `<p>Backs everything up &mdash; configuration included.</p>`,
      errors: [{ messageId: "inJsxText" }],
    },
    {
      // Spoken by a screen reader, so it is text a user reads.
      code: `<button aria-label="Delete ${EM} permanently" onClick={f}><svg /></button>`,
      errors: [{ messageId: "inAttribute" }],
    },
    {
      code: `<input placeholder={"host ${EM} optional"} />`,
      errors: [{ messageId: "inAttribute" }],
    },
    {
      // A marker whose reason is a shrug suppresses nothing.
      code: `
        {/* bv-convention-exception: no-em-dash-in-user-text -- nah */}
        <td>${EM}</td>
      `,
      errors: [{ messageId: "inJsxText" }],
    },
    {
      // A marker naming a DIFFERENT rule does not suppress this one.
      code: `
        {/* bv-convention-exception: user-message-is-translated -- the wrong
            rule name, so this exception does not apply here at all. */}
        <td>${EM}</td>
      `,
      errors: [{ messageId: "inJsxText" }],
    },
  ],
});

// ---------------------------------------------------------------------------
// 9. The router's routed pages are all files page-uses-page-shell can see.
//
// The RuleTester cases above run against synthetic `filename:` strings, so they
// prove what the rule DOES with a file it is handed. They cannot prove it is
// handed the right files, and that is the rule's one structural blind spot: it
// returns `{}` for anything outside src/pages/*.tsx, and inside such a file it
// only recognises the page component as the default export or an export named
// after the file (`Fleet` / `FleetPage` in Fleet.tsx). A routed page whose
// component is neither — `web/src/pages/Reports.tsx` exporting
// `function ReportsView()` — or one placed outside src/pages, ships with its
// own width and gap and the rule says nothing.
//
// page-uses-page-shell.js's header cites this test as the proof that no routed
// page falls through that gap. It is that proof: it reads the REAL router and
// resolves every routed element back to a file the rule would actually visit.
// ---------------------------------------------------------------------------
describe("page-uses-page-shell sees every routed page", () => {
  const HERE = dirname(fileURLToPath(import.meta.url));
  const ROUTER = readFileSync(join(HERE, "..", "app", "router.tsx"), "utf8");
  const PAGES_DIR = join(HERE, "..", "pages");

  /** `<Route path="/x" element={<Foo />} />` → the component names routed to.
   *  `<Navigate …>` is excluded: a redirect renders no page. */
  const routed = [
    ...new Set(
      [...ROUTER.matchAll(/element=\{<([A-Z][A-Za-z0-9_]*)\s*\/?>/g)]
        .map((m) => m[1])
        .filter((name) => name !== "Navigate" && name !== "Layout")
    ),
  ];

  /** Every name router.tsx imports, mapped to the module it came from, e.g.
   *  Recovery → "../pages/Recovery". Both `import X from` and `import { X } from`
   *  are read: the pages use both forms. */
  const IMPORTS = new Map<string, string>();
  for (const m of ROUTER.matchAll(/^import\s+([^;]+?)\s+from\s+"([^"]+)";/gm)) {
    const [, clause, from] = m;
    const braced = /\{([^}]*)\}/.exec(clause);
    const names = braced
      ? braced[1].split(",").map((n) => n.trim().split(/\s+as\s+/).pop() ?? "")
      : [clause.trim()];
    for (const n of names) if (n) IMPORTS.set(n, from);
  }

  it("finds the routes (the scan is not silently empty)", () => {
    expect(routed.length).toBeGreaterThan(8);
    expect(routed).toContain("SettingsPage");
  });

  it.each(routed)("%s lives in src/pages and exports a name the rule recognises", (name) => {
    const from = IMPORTS.get(name);
    expect(from, `${name} is routed but not imported in router.tsx`).toBeDefined();
    expect(
      from,
      `${name} is routed from "${from}" — page-uses-page-shell only visits src/pages/*.tsx, ` +
        `so a routed page outside it is unchecked`
    ).toMatch(/^\.\.\/pages\//);

    const stem = (from as string).slice("../pages/".length);
    const source = readFileSync(join(PAGES_DIR, stem + ".tsx"), "utf8");

    // The rule's own candidate test: the default export, or an export named
    // <stem> or <stem>Page. Keep this in step with page-uses-page-shell.js's
    // `wanted` set — if that widens, widen this.
    const named = new RegExp(
      String.raw`export\s+(?:(?:async\s+)?function|const)\s+(?:${stem}|${stem}Page)\b`
    );
    const hasDefault = /export\s+default\s/.test(source);
    expect(
      hasDefault || named.test(source),
      `src/pages/${stem}.tsx must export its page component as the default export or as ` +
        `"${stem}"/"${stem}Page" — page-uses-page-shell finds the component by that name and ` +
        `silently checks nothing otherwise`
    ).toBe(true);

    // …and the routed name must be one of those, not a third alias.
    expect(
      [stem, `${stem}Page`].includes(name) || hasDefault,
      `router.tsx routes <${name} />, which is neither ${stem} nor ${stem}Page nor a default export`
    ).toBe(true);
  });
});
