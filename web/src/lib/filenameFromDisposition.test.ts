/**
 * The download filename comes from the server.
 *
 * The recovery kit is the reason: it is "bombvault-recovery-kit.md" in the
 * clear and "bombvault-recovery-kit.md.age" once export encryption seals it.
 * A hard-coded name on the client would save ciphertext under a .md extension,
 * which an editor then opens as a wall of base64 with no hint why.
 */
import { describe, expect, it } from "vitest";

import { filenameFromDisposition } from "./api";

describe("filenameFromDisposition", () => {
  it("reads a quoted filename", () => {
    expect(filenameFromDisposition(`attachment; filename="bombvault-recovery-kit.md"`)).toBe(
      "bombvault-recovery-kit.md"
    );
  });

  it("reads the sealed name, extension and all", () => {
    expect(filenameFromDisposition(`attachment; filename="bombvault-recovery-kit.md.age"`)).toBe(
      "bombvault-recovery-kit.md.age"
    );
  });

  it("reads an unquoted filename", () => {
    expect(filenameFromDisposition("attachment; filename=bundle.zip")).toBe("bundle.zip");
  });

  it("falls back to null when there is nothing to read", () => {
    expect(filenameFromDisposition(null)).toBeNull();
    expect(filenameFromDisposition("attachment")).toBeNull();
    expect(filenameFromDisposition("")).toBeNull();
  });
});
