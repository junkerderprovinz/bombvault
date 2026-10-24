import { describe, expect, it } from "vitest";
import { excludesEverything } from "./excludeLines";

// The same lines as internal/backup/excludes_test.go, so both sides agree.
describe("excludesEverything", () => {
  it("catches a line that matches every path", () => {
    for (const line of ["/", " // ", "/.", "/config/..", "*", "**", "*/", "/*", "/**", "**/*", "/**/*"]) {
      expect(excludesEverything(line), line).toBe(true);
    }
  });

  it("leaves every other line alone", () => {
    for (const line of ["", ".", "a/..", "*.log", ".git", "/config", "/config/*", "/*/cache", "**/cache", "../*"]) {
      expect(excludesEverything(line), line).toBe(false);
    }
  });
});
