// ---------------------------------------------------------------------------
// Vitest config — pure-logic tests only (node environment, no DOM library).
// Test files live next to their subject as src/**/*.test.ts; they are excluded
// from the tsc program (see tsconfig.json "exclude") — esbuild transpiles them
// for the test run.
// ---------------------------------------------------------------------------
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
