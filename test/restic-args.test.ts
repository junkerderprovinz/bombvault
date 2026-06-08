import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  buildBackupArgs,
  buildRestoreArgs,
  parseBackupSummary,
} from "../lib/restic";

// Pure unit tests — no restic binary required.

// --- buildBackupArgs ---

test("buildBackupArgs: produces correct argv for repo, paths and tags", () => {
  const args = buildBackupArgs("/repo", ["/data/a", "/data/b"], ["container:plex", "p1"]);
  assert.deepEqual(args, [
    "-r", "/repo",
    "backup",
    "--json",
    "--tag", "container:plex",
    "--tag", "p1",
    "--",
    "/data/a",
    "/data/b",
  ]);
});

test("buildBackupArgs: works with a single path and no tags", () => {
  const args = buildBackupArgs("/myrepo", ["/single/path"], []);
  assert.deepEqual(args, [
    "-r", "/myrepo",
    "backup",
    "--json",
    "--",
    "/single/path",
  ]);
});

test("buildBackupArgs: places -- before any path that begins with a dash (SEC-103)", () => {
  const args = buildBackupArgs("/repo", ["--malicious-flag"], []);
  const dashIdx = args.indexOf("--");
  const pathIdx = args.indexOf("--malicious-flag");
  assert.ok(dashIdx >= 0, "end-of-options -- must be present");
  assert.ok(pathIdx > dashIdx, "the dash-leading path must come after --");
});

// --- buildRestoreArgs ---

test("buildRestoreArgs: produces correct argv for repo, snapshotId and targetDir", () => {
  const args = buildRestoreArgs("/repo", "abc123", "/restore/here");
  assert.deepEqual(args, [
    "-r", "/repo",
    "restore",
    "--target", "/restore/here",
    "--", "abc123",
  ]);
});

test("buildRestoreArgs: places -- before the snapshotId so it cannot be read as a flag (SEC-103)", () => {
  const args = buildRestoreArgs("/repo", "deadbeef", "/");
  const dashIdx = args.indexOf("--");
  const snapIdx = args.indexOf("deadbeef");
  assert.ok(dashIdx >= 0, "end-of-options -- must be present");
  assert.ok(snapIdx > dashIdx, "snapshotId must come after --");
});

// --- parseBackupSummary ---

test("parseBackupSummary: extracts summary from realistic multi-line --json stream", () => {
  const fixtureDir = join(import.meta.dirname ?? new URL(".", import.meta.url).pathname, "fixtures");
  const raw = readFileSync(join(fixtureDir, "restic-backup-summary.json"), "utf-8");
  const summary = parseBackupSummary(raw);
  assert.match(summary.snapshotId, /^[0-9a-f]{8,}$/);
  assert.strictEqual(typeof summary.bytesAdded, "number");
  assert.ok(summary.bytesAdded >= 0);
  assert.strictEqual(typeof summary.totalBytesProcessed, "number");
  assert.ok(summary.totalBytesProcessed >= 0);
});

test("parseBackupSummary: throws when no summary line is present", () => {
  const onlyStatus = `{"message_type":"status","percent_done":0.5}\n{"message_type":"status","percent_done":1.0}\n`;
  assert.throws(
    () => parseBackupSummary(onlyStatus),
    /no summary/i,
  );
});

test("parseBackupSummary: tolerates malformed JSON lines before a valid summary", () => {
  const mixed = `not-json\n{"message_type":"summary","snapshot_id":"deadbeefdeadbeef","data_added":999,"total_bytes_processed":512}\n`;
  const summary = parseBackupSummary(mixed);
  assert.equal(summary.snapshotId, "deadbeefdeadbeef");
  assert.equal(summary.bytesAdded, 999);
  assert.equal(summary.totalBytesProcessed, 512);
});
