// Edge-safe stateless session helpers — no argon2, no better-sqlite3.
// Uses globalThis.crypto.subtle (WebCrypto) — available in Next.js Edge runtime
// AND Node 20+. Do NOT import from "crypto" or "node:crypto" here; those modules
// are not available in the Edge runtime and will throw at request time even though
// `next build` succeeds.
// lib/auth.ts re-exports these so the rest of the codebase uses a single import.
// middleware.ts imports directly from here to avoid pulling argon2 into Edge.

export const SESSION_COOKIE = "bombvault_session";

// Session lifetime: tokens carry an absolute expiry (exp) and are rejected once
// it passes. Cookie maxAge in app/actions/auth.ts is set to match this.
export const SESSION_LIFETIME_SECONDS = 7 * 24 * 60 * 60; // 7 days

// Integrity-protected session claims. Carried in the token payload (base64url
// JSON) and HMAC-signed as a whole, so none of these fields can be tampered with.
//   u  — username
//   iat — issued-at (unix seconds)
//   exp — absolute expiry (unix seconds)
//   sv  — session_version at sign time; revalidated server-side (requireSession)
//         against the DB to support revocation (logout / password change bumps it).
export interface SessionClaims {
  u: string;
  iat: number;
  exp: number;
  sv: number;
}

// APP_KEY must be exactly 64 lowercase-hex chars (32 bytes). Buffer.from(key,"hex")
// silently truncates on malformed input, which would make tokens cross-forgeable.
function assertValidKey(appKey: string): void {
  if (!/^[0-9a-f]{64}$/.test(appKey)) {
    throw new Error("appKey must be exactly 64 lowercase-hex characters (32 bytes)");
  }
}

// Produce a plain ArrayBuffer from a Node Buffer so that TypeScript's strict
// BufferSource typing for WebCrypto APIs (which forbids SharedArrayBuffer) is
// satisfied. Buffer.prototype.buffer is typed as ArrayBufferLike; .slice() on
// the underlying ArrayBuffer always returns a fresh, non-shared ArrayBuffer.
function toArrayBuffer(buf: Buffer): ArrayBuffer {
  return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer;
}

// Import a raw 32-byte key for HMAC-SHA-256 signing/verification.
// Returns a CryptoKey usable only for the requested usages.
async function importKey(appKey: string, usages: KeyUsage[]): Promise<CryptoKey> {
  return globalThis.crypto.subtle.importKey(
    "raw",
    toArrayBuffer(Buffer.from(appKey, "hex")),
    { name: "HMAC", hash: "SHA-256" },
    false,
    usages,
  );
}

// Token format: <base64url-payload>.<base64url-signature>
// The payload is base64url-encoded JSON SessionClaims; the signature is the raw
// HMAC-SHA-256 output (32 bytes) over the payload, encoded as base64url.
// Buffer is available in Edge (Next.js bundles the `buffer` builtin); it is used
// only for text/base64url encoding — no node:crypto import anywhere in this file.

/**
 * Sign a session token for `username`.
 * @param sessionVersion current session_version (revocation epoch); revalidated
 *   server-side. @param now/lifetime overridable for tests.
 */
export async function signSession(
  username: string,
  appKey: string,
  sessionVersion = 0,
  now: number = Math.floor(Date.now() / 1000),
  lifetimeSeconds: number = SESSION_LIFETIME_SECONDS,
): Promise<string> {
  if (!username || !username.trim()) {
    throw new Error("username must not be empty or whitespace");
  }
  assertValidKey(appKey);
  const claims: SessionClaims = {
    u: username,
    iat: now,
    exp: now + lifetimeSeconds,
    sv: sessionVersion,
  };
  const payload = Buffer.from(JSON.stringify(claims), "utf8").toString("base64url");
  const key = await importKey(appKey, ["sign"]);
  const sigBuf = await globalThis.crypto.subtle.sign(
    "HMAC",
    key,
    toArrayBuffer(Buffer.from(payload, "utf8")),
  );
  const sig = Buffer.from(sigBuf).toString("base64url");
  return `${payload}.${sig}`;
}

function parseClaims(payload: string): SessionClaims | null {
  let json: unknown;
  try {
    json = JSON.parse(Buffer.from(payload, "base64url").toString("utf8"));
  } catch {
    return null;
  }
  if (typeof json !== "object" || json === null) return null;
  const c = json as Record<string, unknown>;
  if (
    typeof c.u !== "string" ||
    !c.u.trim() ||
    typeof c.iat !== "number" ||
    typeof c.exp !== "number" ||
    typeof c.sv !== "number" ||
    !Number.isFinite(c.exp)
  ) {
    return null;
  }
  return { u: c.u, iat: c.iat, exp: c.exp, sv: c.sv };
}

/**
 * Verify a token's signature, structure and expiry. Returns the full claims on
 * success, or null if forged, malformed, or expired. Does NOT check session_version
 * against the DB — that is a Node-side concern (see lib/auth-server.ts requireSession).
 */
export async function verifySessionClaims(
  token: string,
  appKey: string,
  now: number = Math.floor(Date.now() / 1000),
): Promise<SessionClaims | null> {
  assertValidKey(appKey);
  const dot = token.lastIndexOf(".");
  if (dot <= 0) return null;
  const payload = token.slice(0, dot);
  const sigB64 = token.slice(dot + 1);

  // Decode the signature; reject if it isn't exactly 32 bytes (SHA-256 output).
  let sigBytes: Buffer;
  try {
    sigBytes = Buffer.from(sigB64, "base64url");
  } catch {
    return null;
  }
  if (sigBytes.length !== 32) return null;

  const key = await importKey(appKey, ["verify"]);
  // crypto.subtle.verify is constant-time — replaces timingSafeEqual.
  const valid = await globalThis.crypto.subtle.verify(
    "HMAC",
    key,
    toArrayBuffer(sigBytes),
    toArrayBuffer(Buffer.from(payload, "utf8")),
  );
  if (!valid) return null;

  const claims = parseClaims(payload);
  if (!claims) return null;
  if (claims.exp < now) return null; // expired
  return claims;
}

/**
 * Verify a token and return the username, or null. Thin convenience wrapper over
 * verifySessionClaims for callers (middleware) that only need authentication, not
 * the claims. Edge-safe; no DB check (revocation is enforced server-side).
 */
export async function verifySession(
  token: string,
  appKey: string,
  now: number = Math.floor(Date.now() / 1000),
): Promise<string | null> {
  const claims = await verifySessionClaims(token, appKey, now);
  return claims ? claims.u : null;
}
