import { NextResponse, type NextRequest } from "next/server";
import { decideRoute } from "./lib/route-guard";
import { verifySession, SESSION_COOKIE } from "./lib/session";

// Edge middleware verifies the signed session cookie using HMAC-SHA256 (node:crypto,
// available in Next.js 14 Edge runtime). No DB or argon2 calls — those stay in
// Node-runtime server components and server actions only.
export function middleware(req: NextRequest) {
  const token = req.cookies.get(SESSION_COOKIE)?.value ?? "";
  const appKey = process.env.APP_KEY ?? "";
  const authed = appKey.length === 64 && verifySession(token, appKey) !== null;
  const target = decideRoute({ path: req.nextUrl.pathname, authed });
  if (target) return NextResponse.redirect(new URL(target, req.url));
  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/spike/:path*", "/login", "/onboarding"],
};
