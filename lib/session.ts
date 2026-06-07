// Edge-safe stateless session helpers — no argon2, no better-sqlite3.
// Uses the "crypto" module (bare specifier, no node: prefix) which is resolved
// by Next.js in both the Edge runtime and Node.js server components.
// lib/auth.ts re-exports these so the rest of the codebase uses a single import.
// middleware.ts imports directly from here to avoid pulling argon2 into Edge.

import { createHmac, timingSafeEqual } from "crypto";

export const SESSION_COOKIE = "bombvault_session";

// APP_KEY must be exactly 64 lowercase-hex chars (32 bytes). Buffer.from(key,"hex")
// silently truncates on malformed input, which would make tokens cross-forgeable.
function assertValidKey(appKey: string): void {
  if (!/^[0-9a-f]{64}$/.test(appKey)) {
    throw new Error("appKey must be exactly 64 lowercase-hex characters (32 bytes)");
  }
}

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
