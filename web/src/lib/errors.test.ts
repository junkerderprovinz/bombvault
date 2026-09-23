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

  it("falls back to the generic message when error is empty or whitespace", () => {
    expect(loadErrorMessage({ error: "" }, "fallback")).toBe("fallback");
    expect(loadErrorMessage({ error: "   " }, "fallback")).toBe("fallback");
  });
});
