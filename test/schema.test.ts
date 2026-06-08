import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
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

function freshDb() {
  const dir = mkdtempSync(join(tmpdir(), "bv-db-"));
  return new Database(join(dir, "bombvault.sqlite"));
}

test("runMigrations creates the setting table", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const tables = (
    db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all() as { name: string }[]
  ).map((t) => t.name);
  assert.ok(tables.includes("setting"), "missing setting table");
  assert.ok(tables.includes("schema_migrations"), "missing migrations bookkeeping table");
  assert.ok(!tables.includes("user"), "user table should not exist");
  db.close();
});

test("setting table has the expected columns", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const cols = (db.prepare("PRAGMA table_info(setting)").all() as { name: string }[]).map(
    (c) => c.name,
  );
  for (const c of ["key", "value"]) {
    assert.ok(cols.includes(c), `missing setting column ${c}`);
  }
  db.close();
});

test("runMigrations is idempotent and records applied versions", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const before = db.prepare("SELECT COUNT(*) AS n FROM schema_migrations").get() as { n: number };
  assert.doesNotThrow(() => runMigrations(db));
  const after = db.prepare("SELECT COUNT(*) AS n FROM schema_migrations").get() as { n: number };
  assert.equal(after.n, before.n, "re-running must not re-apply migrations");
  db.close();
});

test("a setting row survives a second migration run", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  db.prepare("INSERT INTO setting (key, value) VALUES (?, ?)").run("onboarded", "false");
  runMigrations(db);
  const row = db.prepare("SELECT value FROM setting WHERE key = ?").get("onboarded") as {
    value: string;
  };
  assert.equal(row.value, "false");
  db.close();
});

// ── v2 tests ────────────────────────────────────────────────────────────────

test("v2 creates destination, backup_target and run tables", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const tables = (
    db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all() as { name: string }[]
  ).map((t) => t.name);
  for (const tbl of ["destination", "backup_target", "run"]) {
    assert.ok(tables.includes(tbl), `missing table ${tbl}`);
  }
  db.close();
});

test("run table has the expected columns", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const cols = (db.prepare("PRAGMA table_info(run)").all() as { name: string }[]).map(
    (c) => c.name,
  );
  for (const c of [
    "id",
    "target_id",
    "kind",
    "status",
    "started_at",
    "finished_at",
    "snapshot_id",
    "bytes",
    "error",
    "log_ref",
  ]) {
    assert.ok(cols.includes(c), `missing run column ${c}`);
  }
  db.close();
});

test("FK from backup_target.destination_id to destination is enforced", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  db.pragma("foreign_keys = ON");
  assert.throws(
    () => {
      db
        .prepare(
          "INSERT INTO backup_target (id, destination_id, container_name, appdata_paths, options, created_at) VALUES (?,?,?,?,?,?)",
        )
        .run("bt1", "nonexistent-dest-id", "mycontainer", "[]", "{}", Date.now());
    },
    /FOREIGN KEY constraint failed/i,
    "inserting a backup_target with a bad destination_id must throw",
  );
  db.close();
});

test("runMigrations is idempotent at version 2", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const before = db.prepare("SELECT COUNT(*) AS n FROM schema_migrations").get() as { n: number };
  assert.doesNotThrow(() => runMigrations(db));
  const after = db.prepare("SELECT COUNT(*) AS n FROM schema_migrations").get() as { n: number };
  assert.equal(after.n, before.n, "re-running must not re-apply migrations");
  assert.equal(after.n, 2, "exactly two migrations should be recorded");
  db.close();
});
