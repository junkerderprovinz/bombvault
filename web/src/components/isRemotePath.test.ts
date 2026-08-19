// ---------------------------------------------------------------------------
// isRemotePath — the client-side mirror of restic's remoteRepoRe
// (internal/restic/restic.go), used by PathModeSwitch to decide whether a
// backup path field opens in Local or Remote mode. It must accept exactly the
// same schemes the backend's resolveRepo treats as a remote restic backend
// (issue #152) — a false negative here would silently show the folder
// browser for a path the backend actually hands straight to restic as a
// remote URL, and a false positive would hide the ordinary local-path field
// behind the remote dialog for a real subpath that merely contains a colon.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { isRemotePath } from "./PathModeSwitch";

describe("isRemotePath", () => {
  it("accepts every restic remote backend prefix restic.IsRemoteRepo does", () => {
    for (const url of [
      "rclone:backblaze:bucket/path",
      "sftp:user@host:/repo",
      "rest:http://host:8000/repo",
      "s3:bucket/path",
      "b2:bucket:path",
      "azure:container:path",
      "gs:bucket:path",
      "swift:container:path",
    ]) {
      expect(isRemotePath(url)).toBe(true);
    }
  });

  it("rejects a plain local subpath", () => {
    expect(isRemotePath("user/bombvault/containers")).toBe(false);
    expect(isRemotePath("")).toBe(false);
  });

  it("rejects an absolute local path even though it contains no scheme", () => {
    expect(isRemotePath("/mnt/user/bombvault/containers")).toBe(false);
  });

  it("rejects a scheme-like prefix that isn't a recognised remote backend", () => {
    // Mirrors restic.LooksLikeUnprefixedRemote's negative space: an rclone
    // remote NAME typed without the required "rclone:" prefix (the common
    // mistake) must NOT be treated as already-remote — it needs the same
    // "not a recognised URL" local-mode fallback a typo would get.
    expect(isRemotePath("BackBlaze:bucket/path")).toBe(false);
  });

  it("trims surrounding whitespace before matching", () => {
    expect(isRemotePath("  s3:bucket/path  ")).toBe(true);
  });

  it("is case-sensitive on the scheme, matching restic's own lowercase-only regex", () => {
    expect(isRemotePath("S3:bucket/path")).toBe(false);
  });
});
