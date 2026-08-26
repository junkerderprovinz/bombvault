// ---------------------------------------------------------------------------
// The settled UI conventions — rule tests.
//
// web/lint-rules/ turns six house conventions into lint errors. This file is
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
// 7. The router's routed pages are all files page-uses-page-shell can see.
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
