// Pure language-resolution logic, shared by the server picker and tests.
// Given an ordered list of candidate locale tags (e.g. Accept-Language entries
// or a stored cookie), return the first one we actually support, matching either
// the full tag or its base language. Region variants fall back to their base
// (de-AT -> de); unknown candidates are skipped; nothing matches -> fallback.

export const COOKIE = "bv_lang";

export function resolveLanguage(
  candidates: readonly string[],
  supported: readonly string[],
  fallback: string,
): string {
  const supportedSet = new Set(supported.map((s) => s.toLowerCase()));

  for (const raw of candidates) {
    const tag = raw?.trim().toLowerCase();
    if (!tag) continue;
    if (supportedSet.has(tag)) return tag;
    const base = tag.split("-")[0];
    if (base && supportedSet.has(base)) return base;
  }

  return fallback;
}

/**
 * Parse an Accept-Language header into an ordered list of locale tags, most
 * preferred first. Q-values sort the list (default q=1); the wildcard and
 * malformed entries are dropped. A stable sort keeps the original order among
 * equal q-values. Safe to call on the server.
 */
export function parseAcceptLanguage(
  header: string | null | undefined,
): string[] {
  if (!header) return [];
  return header
    .split(",")
    .map((part, index) => {
      const [tag, ...params] = part.trim().split(";");
      const qParam = params.find((p) => p.trim().startsWith("q="));
      const qRaw = qParam ? Number(qParam.trim().slice(2)) : 1;
      const q = Number.isFinite(qRaw) ? qRaw : 1;
      return { tag: tag.trim(), q, index };
    })
    .filter((e) => e.tag && e.tag !== "*")
    .sort((a, b) => b.q - a.q || a.index - b.index)
    .map((e) => e.tag);
}

/**
 * Server-side language resolution from a cookie value and an Accept-Language
 * header: an explicit cookie choice wins, then the browser's header
 * preferences, then `fallback`.
 */
export function pickLanguage(
  cookie: string | null | undefined,
  acceptLanguage: string | null | undefined,
  supported: readonly string[],
  fallback: string,
): string {
  const candidates = [
    ...(cookie ? [cookie] : []),
    ...parseAcceptLanguage(acceptLanguage),
  ];
  return resolveLanguage(candidates, supported, fallback);
}

/** Persist the chosen language for a year so it survives reloads (client only). */
export function writeLanguageCookie(code: string): void {
  if (typeof document === "undefined") return;
  const maxAge = 60 * 60 * 24 * 365;
  document.cookie = `${COOKIE}=${encodeURIComponent(code)}; path=/; max-age=${maxAge}; samesite=lax`;
}
