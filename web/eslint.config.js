// ---------------------------------------------------------------------------
// ESLint flat config — the frontend lint gate (npm run lint → `eslint src`).
//
// @eslint/js recommended + typescript-eslint recommended (deliberately the
// NON-type-checked variant: type-aware linting would pull the whole DOM lib
// into the lint program) + react-hooks (rules-of-hooks = error, the
// correctness rule this gate exists for; exhaustive-deps = warn).
//
// TypeScript 6 side-by-side shim
// ------------------------------
// The project's `typescript` devDep is the native TS 7 compiler (tsgo), which
// no longer ships the JS compiler API. typescript-eslint hard-throws on
// ts.versionMajorMinor >= 7 and its error message prescribes the official
// "running side-by-side with TypeScript 6.0" pattern. npm cannot nest a peer
// dependency over a conflicting root direct dependency, so the TS 6.0.3 copy
// (the last JS-API TypeScript, inside typescript-eslint's ">=4.8.4 <6.1.0"
// peer range) lives in the lint-ts/ file: sub-package, and the resolve hook
// below redirects every `typescript` import made from inside the
// typescript-eslint module tree (incl. ts-api-utils) to it. Everything else —
// `tsc --noEmit` in the build, vite, vitest — keeps the real TS 7 package.
// ---------------------------------------------------------------------------

import { createRequire, registerHooks } from "node:module";
import { pathToFileURL } from "node:url";
import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";

// The anchor file does not need to exist — it only pins module resolution to
// the lint-ts/ directory, so `typescript` resolves to lint-ts/node_modules.
// (The sync resolve hook must return a final URL — rewriting context.parentURL
// for nextResolve is silently ignored for CJS requires.)
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

// Imported AFTER the hook is registered — a static import would hoist above
// the registerHooks call and load typescript-eslint against the API-less TS 7.
const tseslintModule = await import("typescript-eslint");
const tseslint = tseslintModule.default ?? tseslintModule;

const SRC = ["src/**/*.{ts,tsx}"];

export default [
  { linterOptions: { reportUnusedDisableDirectives: "error" } },

  // Base recommended sets, scoped to the app sources.
  ...[js.configs.recommended, ...tseslint.configs.recommended].map((config) => ({
    ...config,
    files: SRC,
  })),

  {
    files: SRC,
    plugins: { "react-hooks": reactHooks },
    rules: {
      // The correctness rules this gate exists for.
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",

      // TS itself checks undefined identifiers (and knows the DOM globals);
      // no-undef on TS files only produces false positives.
      "no-undef": "off",

      // Calibration against the existing tree (see the lint gate wave):
      // allow intentionally-unused values when prefixed with "_", and don't
      // fail on empty catch blocks used as deliberate "ignore" handlers.
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
];
