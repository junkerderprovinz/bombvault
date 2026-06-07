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

test("session sign + verify roundtrip; tampered/forged tokens rejected", async () => {
  const token = await signSession("admin", KEY);
  assert.equal(await verifySession(token, KEY), "admin");
  assert.equal(await verifySession(token + "x", KEY), null);
  assert.equal(await verifySession("garbage", KEY), null);
  assert.equal(await verifySession(token, "e".repeat(64)), null);
});

test("SESSION_COOKIE has the app-namespaced name", () => {
  assert.equal(SESSION_COOKIE, "bombvault_session");
});

// --- new negative tests (security hardening) ---------------------------------

test("signSession throws on empty username", async () => {
  await assert.rejects(() => signSession("", KEY), /empty/i);
});

test("signSession throws on whitespace-only username", async () => {
  await assert.rejects(() => signSession("   ", KEY), /empty/i);
});

test("signSession throws on malformed appKey (too short)", async () => {
  await assert.rejects(() => signSession("admin", "deadbeef"), /64 lowercase-hex/i);
});

test("signSession throws on appKey with uppercase hex", async () => {
  await assert.rejects(() => signSession("admin", "D".repeat(64)), /64 lowercase-hex/i);
});

test("verifySession throws on malformed appKey (too short)", async () => {
  const token = await signSession("admin", KEY);
  await assert.rejects(() => verifySession(token, "short"), /64 lowercase-hex/i);
});

test("verifySession rejects token signed with a different valid key", async () => {
  const token = await signSession("admin", KEY);
  assert.equal(await verifySession(token, "e".repeat(64)), null);
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

// --- WebCrypto round-trip regression (guards against "builds but throws on Edge") ---
// These tests run in Node (which exposes globalThis.crypto since Node 19+) and
// verify the full sign→verify cycle using the same WebCrypto path that Edge uses.

test("WebCrypto round-trip: signSession → verifySession returns username", async () => {
  const token = await signSession("alice", KEY);
  assert.equal(await verifySession(token, KEY), "alice");
});

test("WebCrypto round-trip: tampered payload → null", async () => {
  const token = await signSession("alice", KEY);
  // Flip one char in the payload (before the dot)
  const dot = token.lastIndexOf(".");
  const tampered = token.slice(0, dot - 1) + "X" + token.slice(dot);
  assert.equal(await verifySession(tampered, KEY), null);
});

test("WebCrypto round-trip: tampered signature → null", async () => {
  const token = await signSession("alice", KEY);
  const dot = token.lastIndexOf(".");
  // Flip one char inside the base64url signature
  const badSig = token.slice(dot + 1, -1) + "X";
  assert.equal(await verifySession(token.slice(0, dot + 1) + badSig, KEY), null);
});

test("WebCrypto round-trip: wrong appKey (valid hex) → null", async () => {
  const token = await signSession("alice", KEY);
  assert.equal(await verifySession(token, "f".repeat(64)), null);
});

test("WebCrypto round-trip: short appKey → throws before any crypto call", async () => {
  await assert.rejects(() => signSession("alice", "abc123"), /64 lowercase-hex/i);
  await assert.rejects(() => verifySession("anything.atall", "abc123"), /64 lowercase-hex/i);
});
