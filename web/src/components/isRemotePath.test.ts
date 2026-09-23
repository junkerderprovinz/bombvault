// isRemotePath mirrors restic's remoteRepoRe (internal/restic/restic.go) and
// decides whether PathModeSwitch opens a path in Local or Remote mode. It has to
// accept exactly the schemes resolveRepo treats as remote: a false negative
// shows the folder browser for a remote URL, and a false positive hides a local
// subpath that merely contains a colon behind the remote dialog.
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
    // An rclone remote name typed without the "rclone:" prefix, the mistake
    // restic.LooksLikeUnprefixedRemote looks for, is not remote yet and gets
    // the same local-mode fallback as a typo.
    expect(isRemotePath("BackBlaze:bucket/path")).toBe(false);
  });

  it("trims surrounding whitespace before matching", () => {
    expect(isRemotePath("  s3:bucket/path  ")).toBe(true);
  });

  it("is case-sensitive on the scheme, matching restic's own lowercase-only regex", () => {
    expect(isRemotePath("S3:bucket/path")).toBe(false);
  });
});
