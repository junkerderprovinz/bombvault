// ---------------------------------------------------------------------------
// randomId — a v4-UUID-shaped identifier that also works on plain HTTP.
//
// WHY THIS EXISTS (do not "simplify" it back to crypto.randomUUID()):
// crypto.randomUUID() is a SECURE-CONTEXT-ONLY API. It exists on https:// and
// on localhost/127.0.0.1, and is `undefined` everywhere else — so calling it
// throws `TypeError: crypto.randomUUID is not a function`.
//
// BombVault is routinely reached over exactly such an insecure origin: the
// container ships a documented plain-HTTP mode (HTTP_ONLY=true / the Unraid
// template's "WebUI Port (HTTP)", whose own description tells the user to open
// http://<ip>:3000/ instead of the HTTPS link), and that is a LAN IP, not
// localhost, so the browser marks the page insecure. Any crypto.randomUUID()
// on such a page throws — and when the call sits in a load path, the throw
// takes the whole page down with it.
//
// crypto.getRandomValues() is NOT secure-context gated, so the same 122 bits
// of randomness are available there; only the convenience wrapper is missing.
// This helper prefers randomUUID() when present and otherwise assembles the
// identical RFC 9562 v4 layout from getRandomValues(), with a last-resort
// Math.random() path for any exotic environment that has neither (these ids
// are opaque handles, never secrets or tokens — nothing here relies on them
// being unguessable).
// ---------------------------------------------------------------------------

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
