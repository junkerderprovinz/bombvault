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
  getSessionVersion,
  bumpSessionVersion,
  signSession,
  verifySession,
  SESSION_COOKIE,
} from "../lib/auth";
import {
  verifySessionClaims,
  SESSION_LIFETIME_SECONDS,
} from "../lib/session";
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

// --- SEC-002: session expiry + integrity-protected claims --------------------

test("token within lifetime is accepted; claims carry u/iat/exp/sv", async () => {
  const now = 1_000_000;
  const token = await signSession("admin", KEY, 0, now);
  const claims = await verifySessionClaims(token, KEY, now + 60);
  assert.ok(claims, "expected valid claims");
  assert.equal(claims!.u, "admin");
  assert.equal(claims!.iat, now);
  assert.equal(claims!.exp, now + SESSION_LIFETIME_SECONDS);
  assert.equal(claims!.sv, 0);
});

test("expired token is rejected (exp < now)", async () => {
  const now = 1_000_000;
  const token = await signSession("admin", KEY, 0, now, 100); // 100s lifetime
  // one second past expiry
  assert.equal(await verifySession(token, KEY, now + 101), null);
  assert.equal(await verifySessionClaims(token, KEY, now + 101), null);
});

test("token exactly at expiry boundary is still valid", async () => {
  const now = 1_000_000;
  const token = await signSession("admin", KEY, 0, now, 100);
  // exp == now+100; reject only when exp < now, so exactly at exp is valid
  assert.equal(await verifySession(token, KEY, now + 100), "admin");
});

test("tampered payload (re-encoded longer expiry) is rejected", async () => {
  const now = 1_000_000;
  const token = await signSession("admin", KEY, 0, now, 100);
  const dot = token.lastIndexOf(".");
  const sig = token.slice(dot); // keep original signature
  // Forge a payload claiming a far-future expiry, keep the old signature.
  const forgedPayload = Buffer.from(
    JSON.stringify({ u: "admin", iat: now, exp: now + 999999, sv: 0 }),
    "utf8",
  ).toString("base64url");
  assert.equal(await verifySession(forgedPayload + sig, KEY, now + 200), null);
});

test("malformed payload (not JSON) with valid signature is rejected", async () => {
  // Sign an arbitrary base64url string that is NOT valid claims JSON, with the
  // real key, then verify — signature is valid but parseClaims must reject it.
  const payload = Buffer.from("not-json-at-all", "utf8").toString("base64url");
  // Re-create the exact signing the lib does so the signature is valid.
  const keyBuf = Buffer.from(KEY, "hex");
  const cryptoKey = await globalThis.crypto.subtle.importKey(
    "raw",
    keyBuf.buffer.slice(keyBuf.byteOffset, keyBuf.byteOffset + keyBuf.byteLength),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const pbuf = Buffer.from(payload, "utf8");
  const sigBuf = await globalThis.crypto.subtle.sign(
    "HMAC",
    cryptoKey,
    pbuf.buffer.slice(pbuf.byteOffset, pbuf.byteOffset + pbuf.byteLength),
  );
  const sig = Buffer.from(sigBuf).toString("base64url");
  assert.equal(await verifySession(`${payload}.${sig}`, KEY), null);
});

test("sv from sign is round-tripped into the verified claims", async () => {
  const now = 1_000_000;
  const token = await signSession("admin", KEY, 7, now);
  const claims = await verifySessionClaims(token, KEY, now);
  assert.equal(claims!.sv, 7);
});

// --- SEC-002: session_version revocation epoch (DB helpers) -------------------

test("getSessionVersion is 0 on a fresh migrated DB", { skip }, () => {
  const db = freshDb();
  assert.equal(getSessionVersion(db), 0);
  db.close();
});

test("bumpSessionVersion increments and getSessionVersion reflects it", { skip }, () => {
  const db = freshDb();
  assert.equal(bumpSessionVersion(db), 1);
  assert.equal(getSessionVersion(db), 1);
  assert.equal(bumpSessionVersion(db), 2);
  assert.equal(getSessionVersion(db), 2);
  db.close();
});

// --- SEC-006: logout must not bump session_version for unauthenticated callers ---
// These tests exercise the guard logic directly (no Next.js cookies/redirect).
// The logout() server action uses verifySessionClaims + sv comparison before
// calling bumpSessionVersion — we verify each branch of that logic here.

test(
  "valid session token with matching sv → bumpSessionVersion fires (logout guard, happy path)",
  { skip },
  async () => {
    const db = freshDb();
    const sv = getSessionVersion(db); // 0
    const token = await signSession("admin", KEY, sv);
    const claims = await verifySessionClaims(token, KEY);
    assert.ok(claims, "token should verify");
    // Simulate the guard: bump only if claims exist AND sv matches stored epoch.
    const current = getSessionVersion(db);
    if (claims && claims.sv === current) bumpSessionVersion(db);
    assert.equal(getSessionVersion(db), 1, "version should have been bumped for valid session");
    db.close();
  },
);

test(
  "absent/empty token → verifySessionClaims returns null, version must NOT be bumped",
  { skip },
  async () => {
    const db = freshDb();
    const before = getSessionVersion(db); // 0
    const claims = await verifySessionClaims("", KEY);
    assert.equal(claims, null, "empty token must not produce valid claims");
    // Guard: claims is null, so we skip bumpSessionVersion.
    assert.equal(getSessionVersion(db), before, "version must be unchanged for unauthenticated logout");
    db.close();
  },
);

test(
  "expired token → verifySessionClaims returns null, version must NOT be bumped",
  { skip },
  async () => {
    const db = freshDb();
    const before = getSessionVersion(db);
    const pastNow = 1_000_000;
    // Lifetime of 1 second; verify at pastNow + 2 → already expired.
    const token = await signSession("admin", KEY, 0, pastNow, 1);
    const claims = await verifySessionClaims(token, KEY, pastNow + 2);
    assert.equal(claims, null, "expired token must not produce valid claims");
    assert.equal(getSessionVersion(db), before, "version must be unchanged for expired token");
    db.close();
  },
);

test(
  "stale sv (revoked token) → claims.sv !== current epoch, version must NOT be bumped",
  { skip },
  async () => {
    const db = freshDb();
    // Token signed with sv=0; then version bumped to 1 (e.g. prior password change).
    const token = await signSession("admin", KEY, 0);
    bumpSessionVersion(db); // advance epoch to 1
    const claims = await verifySessionClaims(token, KEY);
    assert.ok(claims, "token is still cryptographically valid (not expired)");
    const current = getSessionVersion(db); // 1
    // Guard: sv mismatch → skip bump.
    const versionBefore = current;
    if (claims && claims.sv === current) bumpSessionVersion(db);
    assert.equal(getSessionVersion(db), versionBefore, "stale sv must not trigger another bump");
    db.close();
  },
);
