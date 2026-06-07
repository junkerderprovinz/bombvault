"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import {
  isOnboarded,
  setAdminPassword,
  authenticate,
  getSessionVersion,
  bumpSessionVersion,
  signSession,
  SESSION_COOKIE,
} from "../../lib/auth";
import { SESSION_LIFETIME_SECONDS, verifySessionClaims } from "../../lib/session";

// The `secure` flag must be false when HTTP_ONLY=true (plain-HTTP mode, e.g.
// behind a TLS-terminating proxy). A secure cookie over plain HTTP is silently
// dropped by the browser, which makes login appear to succeed but immediately
// redirects back to /login. When the server runs HTTPS (the default), set secure.
const secureCookie = (process.env.HTTP_ONLY ?? "false").toLowerCase() !== "true";

// Cookie options shared by onboarding + login. maxAge matches the token's exp so
// the browser drops the cookie exactly when the signed token would be rejected.
function sessionCookieOptions() {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: secureCookie,
    path: "/",
    maxAge: SESSION_LIFETIME_SECONDS,
  };
}

export async function completeOnboarding(formData: FormData): Promise<void> {
  const password = String(formData.get("password") ?? "");
  if (password.length < 8) throw new Error("password must be at least 8 characters");
  const db = getDb();
  if (isOnboarded(db)) redirect("/login");
  await setAdminPassword(db, "admin", password);
  const token = await signSession("admin", getConfig().APP_KEY, getSessionVersion(db));
  (await cookies()).set(SESSION_COOKIE, token, sessionCookieOptions());
  redirect("/dashboard");
}

export async function login(formData: FormData): Promise<void> {
  const password = String(formData.get("password") ?? "");
  const db = getDb();
  if (!(await authenticate(db, "admin", password))) redirect("/login?error=1");
  const token = await signSession("admin", getConfig().APP_KEY, getSessionVersion(db));
  (await cookies()).set(SESSION_COOKIE, token, sessionCookieOptions());
  redirect("/dashboard");
}

export async function logout(): Promise<void> {
  // SEC-006: Only bump the global revocation epoch when the caller holds a
  // currently-valid session. An unauthenticated (or forged/replayed) logout
  // invocation must NOT be able to invalidate the admin's tokens — that would
  // be a denial-of-service vector. We still clear the (absent) cookie and
  // redirect so the caller's UX is identical either way.
  const token = (await cookies()).get(SESSION_COOKIE)?.value ?? "";
  const appKey = getConfig().APP_KEY;
  const claims = await verifySessionClaims(token, appKey);
  if (claims) {
    // Token is cryptographically valid AND not expired; also verify the
    // revocation epoch so a stale replayed token can't bump the version.
    const current = getSessionVersion(getDb());
    if (claims.sv === current) {
      bumpSessionVersion(getDb());
    }
  }
  (await cookies()).delete(SESSION_COOKIE);
  redirect("/login");
}
