import { describe, expect, it } from "vitest";

import { ApiError } from "../../lib/api";
import { listError } from "./useInstances";

describe("listError", () => {
  it("shows a paired phone no red line for a list it is refused", () => {
    expect(listError(new ApiError(403, "sign in at this instance", "own-login-needed"), "fallback")).toBeNull();
  });

  it("shows every other failure", () => {
    expect(listError(new ApiError(500, "the server broke"), "fallback")).toBe("the server broke");
    expect(listError("no error object", "fallback")).toBe("fallback");
  });
});
