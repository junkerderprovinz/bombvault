// Node-side authentication helper for protected server components (SEC-005).
// Defense-in-depth: middleware is the first gate; requireSession() is the second,
// run inside the React Server Component on the Node runtime where it CAN read the
// DB. It verifies the signed/expiring token AND that the embedded session_version
// still matches the stored revocation epoch — so logout / password change
// instantly invalidate old tokens even if the cookie is still presented.
//
// This file is Node-only (imports better-sqlite3 transitively via server/db).
// Never import it from middleware.ts or any Edge code.
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getDb } from "../server/db";
import { getConfig } from "./config";
import { getSessionVersion } from "./auth";
import { verifySessionClaims, SESSION_COOKIE } from "./session";

/**
 * Require a valid, non-revoked session. Returns the authenticated username, or
 * redirects to /login (which throws, so the caller never falls through).
 */
export async function requireSession(): Promise<string> {
  const token = (await cookies()).get(SESSION_COOKIE)?.value ?? "";
  const appKey = getConfig().APP_KEY;

  const claims = await verifySessionClaims(token, appKey);
  if (!claims) redirect("/login");

  // Revocation check: the embedded session_version must match the stored epoch.
  const current = getSessionVersion(getDb());
  if (claims.sv !== current) redirect("/login");

  return claims.u;
}
