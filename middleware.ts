import { NextResponse, type NextRequest } from "next/server";
import { decideRoute } from "./lib/route-guard";
import { verifySession, SESSION_COOKIE } from "./lib/session";

// Edge middleware verifies the signed session cookie using HMAC-SHA-256 via
// WebCrypto (globalThis.crypto.subtle). WebCrypto is available in both the
// Next.js Edge runtime and Node 20+. No DB or argon2 calls — those stay in
// Node-runtime server components and server actions only.
//
// DISABLE_AUTH fast-path: the config module cannot be imported on Edge (it uses
// node:fs and node:path), so we read process.env.DISABLE_AUTH directly here,
// applying the same truthy-string parsing used by lib/config.ts (coerce
// "true"/"1" → true; anything else → false). When disabled, all requests pass
// through without any session check or redirect.
function isAuthDisabled(): boolean {
  const v = process.env.DISABLE_AUTH ?? "";
  return v === "true" || v === "1";
}

export async function middleware(req: NextRequest) {
  if (isAuthDisabled()) return NextResponse.next();

  const token = req.cookies.get(SESSION_COOKIE)?.value ?? "";
  const appKey = process.env.APP_KEY ?? "";
  const authed = appKey.length === 64 && (await verifySession(token, appKey)) !== null;
  const target = decideRoute({ path: req.nextUrl.pathname, authed });
  if (target) return NextResponse.redirect(new URL(target, req.url));
  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/spike/:path*", "/login", "/onboarding"],
};
