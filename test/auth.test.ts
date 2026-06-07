import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  hashPassword,
  verifyPassword,
  isOnboarded,
  setAdminPassword,
  signSession,
  verifySession,
  SESSION_COOKIE,
} from "../lib/auth";
import { runMigrations } from "../server/schema";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let Database: any;
let skip: string | false = false;
try {
  Database = (await import("better-sqlite3")).default;
  new Database(":memory:").close();
} catch {
  skip = "better-sqlite3 native addon not built";
}

const KEY = "d".repeat(64);

function freshDb() {
  const dir = mkdtempSync(join(tmpdir(), "bv-auth-"));
  const db = new Database(join(dir, "bombvault.sqlite"));
  runMigrations(db);
  return db;
}

// --- existing tests ----------------------------------------------------------

test("hashPassword + verifyPassword roundtrip", async () => {
  const hash = await hashPassword("correct horse battery staple");
  assert.ok(hash.startsWith("$argon2"));
  assert.equal(await verifyPassword("correct horse battery staple", hash), true);
});

test("verifyPassword rejects the wrong password", async () => {
  const hash = await hashPassword("right");
  assert.equal(await verifyPassword("wrong", hash), false);
});

test("isOnboarded is false on a fresh DB, true after setAdminPassword", { skip }, async () => {
  const db = freshDb();
  assert.equal(isOnboarded(db), false);
  await setAdminPassword(db, "admin", "s3cret-pw");
  assert.equal(isOnboarded(db), true);
});

test("setAdminPassword refuses a second admin", { skip }, async () => {
  const db = freshDb();
  await setAdminPassword(db, "admin", "first");
  await assert.rejects(() => setAdminPassword(db, "admin2", "second"), /already onboarded/i);
});

test("session sign + verify roundtrip; tampered/forged tokens rejected", () => {
  const token = signSession("admin", KEY);
  assert.equal(verifySession(token, KEY), "admin");
  assert.equal(verifySession(token + "x", KEY), null);
  assert.equal(verifySession("garbage", KEY), null);
  assert.equal(verifySession(token, "e".repeat(64)), null);
});

test("SESSION_COOKIE has the app-namespaced name", () => {
  assert.equal(SESSION_COOKIE, "bombvault_session");
});

// --- new negative tests (security hardening) ---------------------------------

test("signSession throws on empty username", () => {
  assert.throws(() => signSession("", KEY), /empty/i);
});

test("signSession throws on whitespace-only username", () => {
  assert.throws(() => signSession("   ", KEY), /empty/i);
});

test("signSession throws on malformed appKey (too short)", () => {
  assert.throws(() => signSession("admin", "deadbeef"), /64 lowercase-hex/i);
});

test("signSession throws on appKey with uppercase hex", () => {
  assert.throws(() => signSession("admin", "D".repeat(64)), /64 lowercase-hex/i);
});

test("verifySession throws on malformed appKey (too short)", () => {
  const token = signSession("admin", KEY);
  assert.throws(() => verifySession(token, "short"), /64 lowercase-hex/i);
});

test("verifySession rejects token signed with a different valid key", () => {
  const token = signSession("admin", KEY);
  assert.equal(verifySession(token, "e".repeat(64)), null);
});

test("setAdminPassword called twice throws on same username", { skip }, async () => {
  const db = freshDb();
  await setAdminPassword(db, "admin", "first");
  await assert.rejects(() => setAdminPassword(db, "admin", "second"), /already onboarded/i);
});

test("setAdminPassword called twice throws on different username", { skip }, async () => {
  const db = freshDb();
  await setAdminPassword(db, "admin", "first");
  // Single-admin invariant: even a different username must be rejected.
  await assert.rejects(
    () => setAdminPassword(db, "superuser", "second"),
    /already onboarded/i,
  );
});
