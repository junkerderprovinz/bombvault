// #129 — a failed load/action often carries the server's own scrubbed reason
// (e.g. "unable to create lock: already locked exclusively", "backup path is
// not mounted") in res.error, but a couple of panels ignored it and always
// showed a generic translated message — turning an actionable error into
// "Weird, it just failed" for both the reporter and whoever triages it next.
// loadErrorMessage prefers that real reason, falling back to the generic
// message only when the server didn't send one (network failure, etc).
export function loadErrorMessage(res: { error?: string }, fallback: string): string {
  const msg = res.error?.trim();
  return msg ? msg : fallback;
}
