import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";

// AES-256-GCM helper for credentials + restic repo passwords at rest. The key is
// APP_KEY (64 hex chars = 32 bytes). Token format: "v1:<ivHex>:<ctHex>:<tagHex>".
// GCM's auth tag gives tamper-detection: any change to iv/ct/tag (or a wrong key)
// makes decrypt throw, never returns wrong plaintext.
//
// Auth tag is pinned to 128 bits (16 bytes) at the cipher level on both encrypt
// and decrypt so OpenSSL cannot silently accept truncated tags (Node DEP0182).
const VERSION = "v1";
const IV_BYTES = 12;
const TAG_BYTES = 16;

const HEX_RE = /^[0-9a-f]*$/;

function malformed(): never {
  throw new Error("malformed secret token");
}

function hexToBuffer(hex: string, expectedBytes?: number): Buffer {
  if (hex.length % 2 !== 0 || !HEX_RE.test(hex)) malformed();
  const buf = Buffer.from(hex, "hex");
  if (expectedBytes !== undefined && buf.length !== expectedBytes) malformed();
  return buf;
}

function keyBuffer(appKey: string): Buffer {
  if (!/^[0-9a-f]{64}$/.test(appKey)) {
    throw new Error("APP_KEY must be 64 lowercase hex characters (32 bytes)");
  }
  return Buffer.from(appKey, "hex");
}

export function encryptSecret(plaintext: string, appKey: string): string {
  const iv = randomBytes(IV_BYTES);
  const cipher = createCipheriv("aes-256-gcm", keyBuffer(appKey), iv, { authTagLength: TAG_BYTES });
  const ct = Buffer.concat([cipher.update(plaintext, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return [VERSION, iv.toString("hex"), ct.toString("hex"), tag.toString("hex")].join(":");
}

export function decryptSecret(token: string, appKey: string): string {
  const parts = token.split(":");
  if (parts.length !== 4 || parts[0] !== VERSION) malformed();
  const [, ivHex, ctHex, tagHex] = parts;

  // Require non-empty iv/tag; ct may be empty (zero-length plaintext).
  if (!ivHex || !tagHex) malformed();

  const iv = hexToBuffer(ivHex, IV_BYTES);
  const ct = hexToBuffer(ctHex); // length unconstrained
  const tag = hexToBuffer(tagHex, TAG_BYTES);

  const decipher = createDecipheriv("aes-256-gcm", keyBuffer(appKey), iv, {
    authTagLength: TAG_BYTES,
  });
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(ct), decipher.final()]).toString("utf8");
}
