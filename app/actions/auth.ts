"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import {
  isOnboarded,
  setAdminPassword,
  authenticate,
  signSession,
  SESSION_COOKIE,
} from "../../lib/auth";

// The `secure` flag must be false when HTTP_ONLY=true (plain-HTTP mode, e.g.
// behind a TLS-terminating proxy). A secure cookie over plain HTTP is silently
// dropped by the browser, which makes login appear to succeed but immediately
// redirects back to /login. When the server runs HTTPS (the default), set secure.
const secureCookie = (process.env.HTTP_ONLY ?? "false").toLowerCase() !== "true";

export async function completeOnboarding(formData: FormData): Promise<void> {
  const password = String(formData.get("password") ?? "");
  if (password.length < 8) throw new Error("password must be at least 8 characters");
  const db = getDb();
  if (isOnboarded(db)) redirect("/login");
  await setAdminPassword(db, "admin", password);
  cookies().set(SESSION_COOKIE, await signSession("admin", getConfig().APP_KEY), {
    httpOnly: true,
    sameSite: "lax",
    secure: secureCookie,
    path: "/",
  });
  redirect("/dashboard");
}

export async function login(formData: FormData): Promise<void> {
  const password = String(formData.get("password") ?? "");
  const db = getDb();
  if (!(await authenticate(db, "admin", password))) redirect("/login?error=1");
  cookies().set(SESSION_COOKIE, await signSession("admin", getConfig().APP_KEY), {
    httpOnly: true,
    sameSite: "lax",
    secure: secureCookie,
    path: "/",
  });
  redirect("/dashboard");
}

export async function logout(): Promise<void> {
  cookies().delete(SESSION_COOKIE);
  redirect("/login");
}
