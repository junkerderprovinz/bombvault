import argon2 from "argon2";
import type Database from "better-sqlite3";

// Re-export Edge-safe session helpers from lib/session.ts so callers use a
// single import. middleware.ts imports directly from lib/session.ts to avoid
// pulling argon2 (a Node-only native module) into the Edge runtime.
export { SESSION_COOKIE, signSession, verifySession } from "./session";

// Precomputed argon2id hash of the string "dummy" — used to equalise timing
// when a username lookup misses, preventing user-enumeration via response time.
// Regenerate with: argon2.hash("dummy", { type: argon2.argon2id })
const DUMMY_HASH =
  "$argon2id$v=19$m=65536,t=3,p=4$aaaaaaaaaaaaaaaaaaaaaa$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

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

// --- session revocation (session_version) ------------------------------------
// The session_version is a monotonically-increasing revocation epoch stored in
// the `setting` table (seeded to 0 by migration 2). Tokens embed the value at
// sign time; requireSession() rejects tokens whose embedded sv != the stored one.
// Bumping it (logout, future password change) invalidates every existing token.

/** Read the current session_version. Lazily seeds the row to 0 if absent. */
export function getSessionVersion(db: Database.Database): number {
  const row = db.prepare("SELECT value FROM setting WHERE key = 'session_version'").get() as
    | { value: string }
    | undefined;
  if (!row) {
    db.prepare(
      "INSERT OR IGNORE INTO setting (key, value) VALUES ('session_version', '0')",
    ).run();
    return 0;
  }
  const n = Number.parseInt(row.value, 10);
  return Number.isFinite(n) ? n : 0;
}

/** Atomically increment session_version, returning the new value. */
export function bumpSessionVersion(db: Database.Database): number {
  const tx = db.transaction(() => {
    const current = getSessionVersion(db);
    const next = current + 1;
    db.prepare(
      "INSERT INTO setting (key, value) VALUES ('session_version', ?) " +
        "ON CONFLICT(key) DO UPDATE SET value = excluded.value",
    ).run(String(next));
    return next;
  });
  return tx();
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
