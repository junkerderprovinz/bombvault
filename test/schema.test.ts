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

test("runMigrations creates the setting and user tables", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const tables = (
    db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all() as { name: string }[]
  ).map((t) => t.name);
  assert.ok(tables.includes("setting"), "missing setting table");
  assert.ok(tables.includes("user"), "missing user table");
  assert.ok(tables.includes("schema_migrations"), "missing migrations bookkeeping table");
  db.close();
});

test("user table has the expected columns", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const cols = (db.prepare("PRAGMA table_info(user)").all() as { name: string }[]).map(
    (c) => c.name,
  );
  for (const c of ["id", "username", "password_hash", "created_at"]) {
    assert.ok(cols.includes(c), `missing user column ${c}`);
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

test("migration seeds session_version = 0 (SEC-002 revocation epoch)", { skip }, () => {
  const db = freshDb();
  runMigrations(db);
  const row = db.prepare("SELECT value FROM setting WHERE key = ?").get("session_version") as
    | { value: string }
    | undefined;
  assert.ok(row, "session_version setting row should be seeded");
  assert.equal(row!.value, "0");
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
