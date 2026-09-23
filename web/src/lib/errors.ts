// loadErrorMessage returns the server's own reason from res.error, such as
// "backup path is not mounted", and the generic message only when the server
// sent none.
export function loadErrorMessage(res: { error?: string }, fallback: string): string {
  const msg = res.error?.trim();
  return msg ? msg : fallback;
}
