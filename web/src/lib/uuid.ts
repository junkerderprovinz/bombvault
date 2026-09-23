// crypto.randomUUID exists only in secure contexts, and BombVault is often
// opened over plain HTTP on a LAN address (HTTP_ONLY=true), where calling it
// throws. getRandomValues is not gated, so randomId builds the same v4 layout
// from it. The Math.random fallback is acceptable because these ids are opaque
// handles, not secrets.

/** A random v4-UUID string, safe to call on insecure (plain-HTTP) origins. */
export function randomId(): string {
  const c: Crypto | undefined = globalThis.crypto;
  if (typeof c?.randomUUID === "function") return c.randomUUID();

  const bytes = new Uint8Array(16);
  if (typeof c?.getRandomValues === "function") {
    c.getRandomValues(bytes);
  } else {
    for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant 10xx

  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
