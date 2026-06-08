import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRepo } from "../lib/backup-repo";
import { runMigrations } from "../server/schema";

// better-sqlite3 is a native addon; on a box without a compiler it may be
// missing. Skip (CI's test job builds it) rather than fail locally.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let Database: any;
let skip: string | false = false;
try {
  Database = (await import("better-sqlite3")).default;
  new Database(":memory:").close();
} catch {
  skip = "better-sqlite3 native addon not built";
}

// 64 lowercase hex chars = valid 32-byte APP_KEY
const APP_KEY = "a".repeat(64);

function freshDb() {
  const dir = mkdtempSync(join(tmpdir(), "bv-backup-repo-"));
  const db = new Database(join(dir, "test.sqlite"));
  db.pragma("journal_mode = WAL");
  db.pragma("foreign_keys = ON");
  runMigrations(db);
  return db;
}

// ── Destination tests ─────────────────────────────────────────────────────────

test(
  "createDestination stores password_ref as ciphertext (v1: prefix), never plaintext",
  { skip },
  () => {
    const db = freshDb();
    const repo = createRepo(db, APP_KEY);

    const dest = repo.createDestination({
      name: "local-repo",
      repoPath: "/mnt/backup/restic",
      password: "super-secret-restic-pw",
    });

    // The stored token must start with "v1:"
    assert.match(dest.password_ref, /^v1:/, "password_ref must be an encrypted v1 token");

    // It must NOT contain the plaintext anywhere in the token
    assert.ok(
      !dest.password_ref.includes("super-secret-restic-pw"),
      "plaintext password must not appear in password_ref",
    );

    db.close();
  },
);

test("getDestinationPassword decrypts back to the original plaintext", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  repo.createDestination({
    name: "local-repo",
    repoPath: "/mnt/backup/restic",
    password: "hunter2-restic-repo-password",
  });

  const dest = repo.listDestinations()[0];
  assert.ok(dest, "destination should exist after creation");

  const decrypted = repo.getDestinationPassword(dest.id);
  assert.equal(decrypted, "hunter2-restic-repo-password");

  db.close();
});

test("getDestinationRow returns the row by id", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const created = repo.createDestination({
    name: "test-dest",
    repoPath: "/srv/restic",
    password: "pw123",
  });

  const fetched = repo.getDestinationRow(created.id);
  assert.ok(fetched, "row should be found");
  assert.equal(fetched.id, created.id);
  assert.equal(fetched.name, "test-dest");
  assert.equal(fetched.repo_path, "/srv/restic");

  db.close();
});

test("getDestinationRow returns undefined for unknown id", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const row = repo.getDestinationRow("no-such-id");
  assert.equal(row, undefined);

  db.close();
});

test("listDestinations returns all destinations in created order", { skip }, () => {
  const db = freshDb();
  let t = 1000;
  const repo = createRepo(db, APP_KEY, () => t++);

  repo.createDestination({ name: "A", repoPath: "/a", password: "pa" });
  repo.createDestination({ name: "B", repoPath: "/b", password: "pb" });

  const list = repo.listDestinations();
  assert.equal(list.length, 2);
  assert.equal(list[0].name, "A");
  assert.equal(list[1].name, "B");

  db.close();
});

// ── Target tests ──────────────────────────────────────────────────────────────

test("createTarget JSON-encodes appdata_paths and options", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "plex",
    appdataPaths: ["/mnt/user/appdata/plex", "/mnt/user/appdata/plex-media"],
    destinationId: dest.id,
    options: { excludeCache: true },
  });

  assert.equal(target.container_name, "plex");
  assert.equal(target.destination_id, dest.id);

  const parsed = repo.parseTarget(target);
  assert.deepEqual(parsed.appdata_paths, ["/mnt/user/appdata/plex", "/mnt/user/appdata/plex-media"]);
  assert.deepEqual(parsed.options, { excludeCache: true });

  db.close();
});

test("getTarget returns the raw row by id", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "whoami",
    appdataPaths: [],
    destinationId: dest.id,
  });

  const fetched = repo.getTarget(target.id);
  assert.ok(fetched);
  assert.equal(fetched.id, target.id);
  assert.equal(fetched.container_name, "whoami");

  db.close();
});

// ── Run tests ─────────────────────────────────────────────────────────────────

test("createRun records status=running, finishRun records final status and snapshotId", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "plex",
    appdataPaths: ["/mnt/user/appdata/plex"],
    destinationId: dest.id,
  });

  const run = repo.createRun({ targetId: target.id, kind: "backup" });
  assert.equal(run.status, "running");
  assert.equal(run.kind, "backup");
  assert.equal(run.target_id, target.id);
  assert.equal(run.snapshot_id, null);
  assert.equal(run.finished_at, null);

  const finished = repo.finishRun(run.id, {
    status: "success",
    snapshotId: "abc1234def",
    bytes: 1024,
  });
  assert.equal(finished.status, "success");
  assert.equal(finished.snapshot_id, "abc1234def");
  assert.equal(finished.bytes, 1024);
  assert.ok(finished.finished_at !== null, "finished_at should be set");

  db.close();
});

test("finishRun with failed status records the error message", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "whoami",
    appdataPaths: [],
    destinationId: dest.id,
  });

  const run = repo.createRun({ targetId: target.id, kind: "backup" });
  const finished = repo.finishRun(run.id, {
    status: "failed",
    error: "restic exited with code 1",
  });

  assert.equal(finished.status, "failed");
  assert.equal(finished.error, "restic exited with code 1");
  assert.equal(finished.snapshot_id, null);

  db.close();
});

test("lastBackupRun returns the most recent successful backup run", { skip }, () => {
  const db = freshDb();
  let t = 1_000_000;
  const repo = createRepo(db, APP_KEY, () => t++);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "plex",
    appdataPaths: ["/mnt/user/appdata/plex"],
    destinationId: dest.id,
  });

  // First successful backup
  const r1 = repo.createRun({ targetId: target.id, kind: "backup" });
  repo.finishRun(r1.id, { status: "success", snapshotId: "snap001", bytes: 512 });

  // Second successful backup (newer)
  const r2 = repo.createRun({ targetId: target.id, kind: "backup" });
  repo.finishRun(r2.id, { status: "success", snapshotId: "snap002", bytes: 768 });

  // A failed backup (should NOT be returned)
  const r3 = repo.createRun({ targetId: target.id, kind: "backup" });
  repo.finishRun(r3.id, { status: "failed", error: "oops" });

  const last = repo.lastBackupRun(target.id);
  assert.ok(last, "should find a last backup run");
  assert.equal(last.snapshot_id, "snap002", "must return the latest successful backup");
  assert.equal(last.status, "success");

  db.close();
});

test("lastBackupRun returns undefined when no successful backup exists", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "empty",
    appdataPaths: [],
    destinationId: dest.id,
  });

  const result = repo.lastBackupRun(target.id);
  assert.equal(result, undefined);

  db.close();
});

test("lastBackupRun ignores restore runs even if successful", { skip }, () => {
  const db = freshDb();
  const repo = createRepo(db, APP_KEY);

  const dest = repo.createDestination({ name: "d", repoPath: "/r", password: "pw" });
  const target = repo.createTarget({
    containerRef: "plex",
    appdataPaths: ["/mnt/user/appdata/plex"],
    destinationId: dest.id,
  });

  // Successful restore (must not count as a backup run)
  const rr = repo.createRun({ targetId: target.id, kind: "restore" });
  repo.finishRun(rr.id, { status: "success", snapshotId: "snap-restore" });

  const last = repo.lastBackupRun(target.id);
  assert.equal(last, undefined, "restore runs must not appear in lastBackupRun");

  db.close();
});
