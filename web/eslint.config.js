// ESLint flat config for `npm run lint` (src, the e2e specs and the
// Playwright config): @eslint/js and
// typescript-eslint recommended, without type information (type-aware linting
// would pull the whole DOM lib into the lint program), plus react-hooks.
//
// The project's `typescript` devDependency is the native TS 7 compiler, which
// ships no JS compiler API, and typescript-eslint refuses to run on TS 7. Its
// error message prescribes running TypeScript 6 side by side. npm cannot nest a
// peer dependency under a conflicting direct dependency, so TS 6.0.3 lives in
// the lint-ts/ sub-package, and the resolve hook below points every
// `typescript` import made from inside typescript-eslint or ts-api-utils at it.
// tsc, vite and vitest keep TS 7.

import { createRequire, registerHooks } from "node:module";
import { pathToFileURL } from "node:url";
import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import bombvault from "./lint-rules/index.js";

// The anchor file need not exist; it only makes `typescript` resolve from
// lint-ts/node_modules. The hook returns a final URL because rewriting
// context.parentURL for nextResolve is ignored for CJS requires.
const lintTsRequire = createRequire(new URL("./lint-ts/_anchor.js", import.meta.url));

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (
      (specifier === "typescript" || specifier.startsWith("typescript/")) &&
      typeof context.parentURL === "string" &&
      (context.parentURL.includes("typescript-eslint") || context.parentURL.includes("ts-api-utils"))
    ) {
      return {
        url: pathToFileURL(lintTsRequire.resolve(specifier)).href,
        shortCircuit: true,
      };
    }
    return nextResolve(specifier, context);
  },
});

// A static import would be hoisted above registerHooks and load
// typescript-eslint against TS 7.
const tseslintModule = await import("typescript-eslint");
const tseslint = tseslintModule.default ?? tseslintModule;

const SRC = ["src/**/*.{ts,tsx}"];
// The Playwright harness: files that run in node outside the app bundle, but
// are first-party TypeScript all the same, so the base gate reads them (the
// e2e/wipe-e2e-data.mjs pre-command is .mjs and stays out).
const HARNESS = ["e2e/**/*.ts", "playwright.config.ts"];

export default [
  { linterOptions: { reportUnusedDisableDirectives: "error" } },

  // Base recommended sets, scoped to the app sources plus the harness.
  ...[js.configs.recommended, ...tseslint.configs.recommended].map((config) => ({
    ...config,
    files: [...SRC, ...HARNESS],
  })),

  {
    files: SRC,
    plugins: { "react-hooks": reactHooks },
    rules: {
      // The correctness rules this gate exists for.
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },

  {
    files: [...SRC, ...HARNESS],
    rules: {
      // The compiler already reports undefined names; no-undef only adds
      // false positives on TypeScript files.
      "no-undef": "off",

      // A leading underscore marks a value that is meant to be unused.
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
        },
      ],
    },
  },

  // The house UI conventions; lint-rules/README.md has the reasoning. Errors
  // rather than warnings, so a regression fails the lint job.
  {
    files: SRC,
    plugins: { bombvault },
    rules: {
      "bombvault/icon-badge-needs-tooltip": "error",
      "bombvault/one-icon-badge-size": "error",
      "bombvault/no-status-color-on-control": "error",
      "bombvault/control-reads-engine-tokens": "error",
      "bombvault/user-message-is-translated": "error",
      // No options: the ru/uk/bg/sr exemption is a fact about those languages,
      // not a project setting, so it lives in the rule.
      "bombvault/no-em-dash-in-user-text": "error",
      "bombvault/page-uses-page-shell": [
        "error",
        {
          // Files that use a different shell, or none (null). See
          // src/lib/pageShell.ts.
          exceptions: {
            // The seven-tab Selector strip is 1424px wide in German, and
            // PAGE_SHELL's 1152px cap would wrap it onto two rows.
            "Settings.tsx": "PAGE_SHELL_TABBED",
            // Below 48rem the Card rhythm steps down to 24px (md:gap-10 is
            // gap-10 at/above it, so desktop is unchanged by construction).
            // Same 1152px cap. See PAGE_SHELL_RESPONSIVE in pageShell.ts.
            "Dashboard.tsx": "PAGE_SHELL_RESPONSIVE",
            "Containers.tsx": "PAGE_SHELL_RESPONSIVE",
            "VMs.tsx": "PAGE_SHELL_RESPONSIVE",
            // Not a routed page: Layout renders it in place of the app shell
            // while auth is blocked.
            "Login.tsx": null,
          },
        },
      ],
    },
  },
];
