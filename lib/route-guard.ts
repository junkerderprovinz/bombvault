// Pure routing decision shared by middleware.ts (unit-testable in isolation).
// Returns the path to redirect to, or null to allow the request through.
const PUBLIC = new Set(["/login", "/onboarding"]);

export function decideRoute(input: { path: string; authed: boolean }): string | null {
  const { path, authed } = input;
  if (PUBLIC.has(path)) {
    return authed && path === "/login" ? "/dashboard" : null;
  }
  return authed ? null : "/login";
}
