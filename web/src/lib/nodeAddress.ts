// The receiving server takes the same address check before it stores one.
// Doing it here as well lets the field say what is wrong while it is typed.

/** What is wrong with an address nodes are to reach the receiver at: not an
 *  address at all, or plain http to a server on the internet. */
export type NodeAddressProblem = "form" | "plain";

function ipv4(host: string): number[] | null {
  const parts = host.split(".");
  if (parts.length !== 4 || !parts.every((p) => /^\d{1,3}$/.test(p) && Number(p) < 256)) return null;
  return parts.map(Number);
}

// The private ranges, loopback, and the carrier-grade NAT range where
// Tailscale and other overlay networks give out addresses.
function privateV4([a, b]: number[]): boolean {
  return a === 10 || a === 127 || (a === 172 && b >= 16 && b < 32) || (a === 192 && b === 168) || (a === 100 && b >= 64 && b < 128);
}

function privateV6(host: string): boolean {
  const bare = host.replace(/^\[|\]$/g, "").toLowerCase();
  return bare === "::1" || /^f[cd][0-9a-f]{0,2}:/.test(bare);
}

export function nodeAddressProblem(raw: string): NodeAddressProblem | null {
  const text = raw.trim();
  if (text === "") return null;
  let url: URL;
  try {
    url = new URL(text);
  } catch {
    return "form";
  }
  if (!url.host || url.username || url.password || url.search || url.hash) return "form";
  if (url.protocol === "https:") return null;
  if (url.protocol !== "http:") return "form";
  const v4 = ipv4(url.hostname);
  if (v4) return privateV4(v4) ? null : "plain";
  return url.hostname.startsWith("[") && privateV6(url.hostname) ? null : "plain";
}
