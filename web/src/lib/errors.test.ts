// #129 — regression: the file-listing panels must surface the server's own
// error text (restic's real, scrubbed failure reason) instead of always
// falling back to the generic "Failed to load files" message.
import { describe, expect, it } from "vitest";
import { loadErrorMessage } from "./errors";

describe("loadErrorMessage", () => {
  it("prefers the server's own error text when present", () => {
    expect(loadErrorMessage({ error: "unable to create lock: already locked" }, "fallback")).toBe(
      "unable to create lock: already locked"
    );
  });

  it("falls back to the generic message when error is absent", () => {
    expect(loadErrorMessage({}, "fallback")).toBe("fallback");
  });

  it("falls back to the generic message when error is empty/whitespace", () => {
    expect(loadErrorMessage({ error: "" }, "fallback")).toBe("fallback");
    expect(loadErrorMessage({ error: "   " }, "fallback")).toBe("fallback");
  });
});
