/**
 * snapshots-view.test.ts — unit tests for app/containers/snapshots/view.ts
 *
 * Pure function: no restic, no DB, no network — always runs in CI.
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import type { ResticSnapshot } from "../lib/restic";
import { toSnapshotRows } from "../app/containers/snapshots/view";

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

function makeSnapshot(overrides: Partial<ResticSnapshot> & { id: string; short_id: string; time: string }): ResticSnapshot {
  return {
    id: overrides.id,
    short_id: overrides.short_id,
    time: overrides.time,
    paths: overrides.paths ?? ["/mnt/user/appdata/plex"],
    hostname: overrides.hostname ?? "unraid",
    tags: overrides.tags,
  };
}

// ---------------------------------------------------------------------------
// toSnapshotRows
// ---------------------------------------------------------------------------

test("toSnapshotRows: maps id from ResticSnapshot", () => {
  const snap = makeSnapshot({ id: "abcdef1234567890abcdef1234567890abcdef12", short_id: "abcdef12", time: "2026-06-01T10:00:00.000Z" });
  const rows = toSnapshotRows([snap]);
  assert.equal(rows[0]?.id, "abcdef1234567890abcdef1234567890abcdef12");
});

test("toSnapshotRows: maps shortId from short_id field", () => {
  const snap = makeSnapshot({ id: "abcdef1234567890abcdef1234567890abcdef12", short_id: "abcdef12", time: "2026-06-01T10:00:00.000Z" });
  const rows = toSnapshotRows([snap]);
  assert.equal(rows[0]?.shortId, "abcdef12");
});

test("toSnapshotRows: maps time as ISO string", () => {
  const time = "2026-06-03T14:30:00.000Z";
  const snap = makeSnapshot({ id: "deadbeef00000000deadbeef00000000deadbeef", short_id: "deadbeef", time });
  const rows = toSnapshotRows([snap]);
  assert.equal(rows[0]?.time, time);
});

test("toSnapshotRows: maps tags when present", () => {
  const snap = makeSnapshot({
    id: "1111111100000000111111110000000011111111",
    short_id: "11111111",
    time: "2026-06-04T08:00:00.000Z",
    tags: ["container:plex", "p1"],
  });
  const rows = toSnapshotRows([snap]);
  assert.deepEqual(rows[0]?.tags, ["container:plex", "p1"]);
});

test("toSnapshotRows: tags is empty array when undefined", () => {
  const snap = makeSnapshot({ id: "2222222200000000222222220000000022222222", short_id: "22222222", time: "2026-06-04T09:00:00.000Z" });
  // tags not set → undefined in fixture
  const rows = toSnapshotRows([snap]);
  assert.deepEqual(rows[0]?.tags, []);
});

test("toSnapshotRows: empty input returns empty array", () => {
  const rows = toSnapshotRows([]);
  assert.deepEqual(rows, []);
});

test("toSnapshotRows: preserves order of snapshots", () => {
  const snaps = [
    makeSnapshot({ id: "aaaa000000000000aaaa000000000000aaaa0000", short_id: "aaaa0000", time: "2026-06-01T00:00:00.000Z" }),
    makeSnapshot({ id: "bbbb000000000000bbbb000000000000bbbb0000", short_id: "bbbb0000", time: "2026-06-02T00:00:00.000Z" }),
    makeSnapshot({ id: "cccc000000000000cccc000000000000cccc0000", short_id: "cccc0000", time: "2026-06-03T00:00:00.000Z" }),
  ];
  const rows = toSnapshotRows(snaps);
  assert.equal(rows.length, 3);
  assert.equal(rows[0]?.shortId, "aaaa0000");
  assert.equal(rows[1]?.shortId, "bbbb0000");
  assert.equal(rows[2]?.shortId, "cccc0000");
});
