// Tests run in the node environment by default. A test that needs a real DOM
// opts into jsdom with a `// @vitest-environment jsdom` docblock at the top of
// its file. Test files are excluded from the tsc program (tsconfig.json) and
// transpiled by esbuild for the run.
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    // jsdom has no ResizeObserver, and Selector observes its row; see the
    // stub's own header.
    setupFiles: ["src/lib/testSetup/resizeObserver.ts"],
  },
});
