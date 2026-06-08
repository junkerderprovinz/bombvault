import { createHmac } from "node:crypto";

// Derive the restic repository password from APP_KEY so the user never has to
// pick, type, or remember one. The backup is still fully encrypted by restic;
// it stays recoverable as long as APP_KEY is preserved (the same key that
// protects every other secret at rest). HMAC domain-separation ("restic-repo")
// keeps this value distinct from any other APP_KEY-derived secret.
//
// APP_KEY is validated upstream as 64 lowercase hex chars (32 bytes); we use the
// raw key bytes as the HMAC key so the full entropy is carried through.
export function deriveResticPassword(appKey: string): string {
  return createHmac("sha256", Buffer.from(appKey, "hex"))
    .update("bombvault:restic-repo")
    .digest("hex");
}
