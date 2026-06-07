import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";

// AES-256-GCM helper for credentials + restic repo passwords at rest. The key is
// APP_KEY (64 hex chars = 32 bytes). Token format: "v1:<ivHex>:<ctHex>:<tagHex>".
// GCM's auth tag gives tamper-detection: any change to iv/ct/tag (or a wrong key)
// makes decrypt throw, never returns wrong plaintext.
const VERSION = "v1";
const IV_BYTES = 12;

function keyBuffer(appKey: string): Buffer {
  if (!/^[0-9a-f]{64}$/.test(appKey)) {
    throw new Error("APP_KEY must be 64 lowercase hex characters (32 bytes)");
  }
  return Buffer.from(appKey, "hex");
}

export function encryptSecret(plaintext: string, appKey: string): string {
  const iv = randomBytes(IV_BYTES);
  const cipher = createCipheriv("aes-256-gcm", keyBuffer(appKey), iv);
  const ct = Buffer.concat([cipher.update(plaintext, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return [VERSION, iv.toString("hex"), ct.toString("hex"), tag.toString("hex")].join(":");
}

export function decryptSecret(token: string, appKey: string): string {
  const parts = token.split(":");
  if (parts.length !== 4 || parts[0] !== VERSION) {
    throw new Error("malformed secret token");
  }
  const [, ivHex, ctHex, tagHex] = parts;
  const decipher = createDecipheriv("aes-256-gcm", keyBuffer(appKey), Buffer.from(ivHex, "hex"));
  decipher.setAuthTag(Buffer.from(tagHex, "hex"));
  return Buffer.concat([decipher.update(Buffer.from(ctHex, "hex")), decipher.final()]).toString(
    "utf8",
  );
}
