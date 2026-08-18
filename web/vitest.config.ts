// ---------------------------------------------------------------------------
// Vitest config — pure-logic tests by default (node environment, no DOM
// library). Test files live next to their subject as src/**/*.test.ts; they
// are excluded from the tsc program (see tsconfig.json "exclude") — esbuild
// transpiles them for the test run.
//
// src/**/*.test.tsx is the deliberate exception: a component that needs a
// real DOM (e.g. clicking a button and asserting which API call it made) opts
// into jsdom via a per-file `// @vitest-environment jsdom` docblock at the top
// of that test file — the default stays "node" for every plain .test.ts file.
// ---------------------------------------------------------------------------
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
  },
});
