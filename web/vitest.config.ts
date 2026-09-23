// Tests run in the node environment by default. A test that needs a real DOM
// opts into jsdom with a `// @vitest-environment jsdom` docblock at the top of
// its file. Test files are excluded from the tsc program (tsconfig.json) and
// transpiled by esbuild for the run.
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    // Two guarded stubs for what jsdom does not implement, each explained in
    // its own header: matchMedia, which lib/useMediaQuery.ts subscribes to
    // during the Layout render and which had to land with useMediaQuery, and
    // ResizeObserver, which Selector uses to measure the row it sits in.
    setupFiles: [
      "src/lib/testSetup/matchMedia.ts",
      "src/lib/testSetup/resizeObserver.ts",
    ],
  },
});
