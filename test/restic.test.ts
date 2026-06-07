import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { version, initRepo, snapshots, parseSnapshotsJson } from "../lib/restic";

// Skip the whole suite gracefully when the restic binary is not installed. CI's
// test job installs it; the Dockerfile bundles it. This proves the adapter, not
// real backup destinations.
let skip: string | false = false;
try {
  execFileSync("restic", ["version"], { stdio: "ignore" });
} catch {
  skip = "restic binary not installed";
}

const PW = "test-repo-password";

// --- parseSnapshotsJson unit tests (no binary required) ---

test("parseSnapshotsJson: empty string returns []", () => {
  assert.deepEqual(parseSnapshotsJson(""), []);
});

test("parseSnapshotsJson: whitespace-only string returns []", () => {
  assert.deepEqual(parseSnapshotsJson("   \n\t  "), []);
});

test("parseSnapshotsJson: valid snapshots JSON is parsed correctly", () => {
  const snapshot = {
    id: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
    short_id: "abc123de",
    time: "2024-01-15T10:30:00.000000000Z",
    paths: ["/home/user/data"],
    hostname: "myhost",
    tags: ["daily"],
  };
  const result = parseSnapshotsJson(JSON.stringify([snapshot]));
  assert.equal(result.length, 1);
  assert.equal(result[0].short_id, "abc123de");
  assert.equal(result[0].hostname, "myhost");
  assert.deepEqual(result[0].paths, ["/home/user/data"]);
});

// --- restic binary integration tests (skipped when binary absent) ---

test("version() returns a parsed restic version string", { skip }, async () => {
  const v = await version();
  assert.match(v, /^\d+\.\d+/, `expected a version number, got "${v}"`);
});

test("initRepo() then snapshots() returns an empty array for a fresh repo", { skip }, async () => {
  const repo = join(mkdtempSync(join(tmpdir(), "bv-restic-")), "repo");
  await initRepo(repo, PW);
  const snaps = await snapshots(repo, PW);
  assert.ok(Array.isArray(snaps));
  assert.equal(snaps.length, 0);
});

test("snapshots() rejects with the wrong password", { skip }, async () => {
  const repo = join(mkdtempSync(join(tmpdir(), "bv-restic-")), "repo");
  await initRepo(repo, PW);
  await assert.rejects(() => snapshots(repo, "wrong-password"));
});
