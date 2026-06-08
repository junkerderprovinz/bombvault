import { test } from "node:test";
import assert from "node:assert/strict";
import { resolve, sep } from "node:path";
import {
  isValidSnapshotId,
  assertValidSnapshotId,
  resolveSnapshotTemplatePath,
} from "../app/containers/snapshots/validate";

// --- SEC-103/104: snapshotId hex validation (layer 1) ---

test("isValidSnapshotId: accepts a plain restic hex id", () => {
  assert.ok(isValidSnapshotId("deadbeef12345678"));
  assert.ok(isValidSnapshotId("0a1b2c3d")); // 8 chars (min)
  assert.ok(isValidSnapshotId("a".repeat(64))); // 64 chars (max)
});

test("isValidSnapshotId: rejects too-short, too-long and non-hex ids", () => {
  assert.ok(!isValidSnapshotId("abc")); // < 8
  assert.ok(!isValidSnapshotId("a".repeat(65))); // > 64
  assert.ok(!isValidSnapshotId("DEADBEEF12345678")); // uppercase
  assert.ok(!isValidSnapshotId("g".repeat(8))); // non-hex
});

test("assertValidSnapshotId: rejects an arg-injection-y snapshotId (SEC-103)", () => {
  assert.throws(() => assertValidSnapshotId("--target=/etc"), /invalid snapshot id/);
});

test("assertValidSnapshotId: rejects a traversal-y snapshotId (SEC-104, layer 1)", () => {
  assert.throws(
    () => assertValidSnapshotId("../../../etc/passwd"),
    /invalid snapshot id/,
    "hex validation already blocks traversal at the source",
  );
});

// --- SEC-104: path-containment assertion (layer 2, defence-in-depth) ---

test("resolveSnapshotTemplatePath: returns an in-dir path for a valid id", () => {
  const dir = "/data/templates";
  const p = resolveSnapshotTemplatePath(dir, "deadbeef12345678", "plex");
  assert.ok(
    resolve(p).startsWith(resolve(dir) + sep),
    "the resolved path must live inside the templates dir",
  );
  assert.ok(p.endsWith("deadbeef12345678-my-plex.xml"));
});

test("resolveSnapshotTemplatePath: rejects a traversal that escapes the templates dir (SEC-104, layer 2)", () => {
  // Even though the hex validation (layer 1) would reject this, the path-
  // containment check independently rejects an escaping snapshotId.
  assert.throws(
    () => resolveSnapshotTemplatePath("/data/templates", "../../etc/cron.d/evil", "x"),
    /invalid snapshot template path/,
  );
});

test("resolveSnapshotTemplatePath: rejects traversal injected via the container name (SEC-104)", () => {
  // container_name is also interpolated into the filename — layer 2 must catch it.
  assert.throws(
    () => resolveSnapshotTemplatePath("/data/templates", "deadbeef12345678", "../../../evil"),
    /invalid snapshot template path/,
  );
});
