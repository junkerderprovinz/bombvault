import { createHmac, timingSafeEqual } from "node:crypto";
import argon2 from "argon2";
import type Database from "better-sqlite3";

export const SESSION_COOKIE = "bombvault_session";

// Precomputed argon2id hash of the string "dummy" — used to equalise timing
// when a username lookup misses, preventing user-enumeration via response time.
// Regenerate with: argon2.hash("dummy", { type: argon2.argon2id })
const DUMMY_HASH =
  "$argon2id$v=19$m=65536,t=3,p=4$aaaaaaaaaaaaaaaaaaaaaa$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

// --- key validation ----------------------------------------------------------
// APP_KEY must be exactly 64 lowercase-hex chars (32 bytes). Buffer.from(key,"hex")
// silently truncates on malformed input, which would make tokens cross-forgeable.
function assertValidKey(appKey: string): void {
  if (!/^[0-9a-f]{64}$/.test(appKey)) {
    throw new Error("appKey must be exactly 64 lowercase-hex characters (32 bytes)");
  }
}

// --- password hashing (argon2id) --------------------------------------------
export function hashPassword(password: string): Promise<string> {
  return argon2.hash(password, { type: argon2.argon2id });
}

// Real library errors (bad hash format, OOM, ABI mismatch) are allowed to
// propagate — only a boolean false is returned on a valid-but-wrong password.
export function verifyPassword(password: string, hash: string): Promise<boolean> {
  return argon2.verify(hash, password);
}

// --- onboarding state --------------------------------------------------------
// "Onboarded" === an admin user row exists. The single-admin model means the
// presence of any user row is the gate.
export function isOnboarded(db: Database.Database): boolean {
  const row = db.prepare("SELECT COUNT(*) AS n FROM user").get() as { n: number };
  return row.n > 0;
}

export async function setAdminPassword(
  db: Database.Database,
  username: string,
  password: string,
): Promise<void> {
  // Hash outside the transaction (async; SQLite transactions must be sync).
  const hash = await hashPassword(password);

  // Atomic: re-check onboarded state and insert within a single transaction to
  // close the TOCTOU window. UNIQUE(username) alone cannot prevent a second row
  // with a different username — we enforce the single-admin invariant explicitly.
  const insert = db.transaction(() => {
    const count = (db.prepare("SELECT COUNT(*) AS n FROM user").get() as { n: number }).n;
    if (count > 0) throw new Error("already onboarded: an admin already exists");
    db.prepare("INSERT INTO user (username, password_hash, created_at) VALUES (?, ?, ?)").run(
      username,
      hash,
      Date.now(),
    );
  });
  insert();
}

export async function authenticate(
  db: Database.Database,
  username: string,
  password: string,
): Promise<boolean> {
  const row = db.prepare("SELECT password_hash FROM user WHERE username = ?").get(username) as
    | { password_hash: string }
    | undefined;

  if (!row) {
    // Always run a full argon2 verify to equalise timing and prevent
    // user-enumeration via response-time differences.
    await argon2.verify(DUMMY_HASH, password).catch(() => false);
    return false;
  }

  return verifyPassword(password, row.password_hash);
}

// --- stateless signed session token -----------------------------------------
// "<username-b64>.<hmacHex>" signed with APP_KEY. Stateless: no session table in
// P0; rotating APP_KEY invalidates all sessions.
function sign(value: string, appKey: string): string {
  return createHmac("sha256", Buffer.from(appKey, "hex")).update(value).digest("hex");
}

export function signSession(username: string, appKey: string): string {
  if (!username || !username.trim()) {
    throw new Error("username must not be empty or whitespace");
  }
  assertValidKey(appKey);
  const payload = Buffer.from(username, "utf8").toString("base64url");
  return `${payload}.${sign(payload, appKey)}`;
}

export function verifySession(token: string, appKey: string): string | null {
  assertValidKey(appKey);
  const dot = token.lastIndexOf(".");
  if (dot <= 0) return null;
  const payload = token.slice(0, dot);
  const mac = token.slice(dot + 1);
  // SHA-256 HMAC is always 32 bytes = 64 lowercase hex chars. Reject anything
  // that doesn't match exactly — Buffer.from silently truncates on invalid hex.
  if (!/^[0-9a-f]{64}$/.test(mac)) return null;
  const expected = sign(payload, appKey);
  const a = Buffer.from(mac, "hex");
  const b = Buffer.from(expected, "hex");
  if (!timingSafeEqual(a, b)) return null;
  return Buffer.from(payload, "base64url").toString("utf8");
}
