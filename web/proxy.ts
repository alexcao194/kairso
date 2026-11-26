import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PUBLIC_PREFIXES = [
  "/login",
  "/forgot-password",
  "/f", // Public feedback form: /f/[token]
];

/** A same-site path to return to after login, or "/". */
function safeNext(from: string | null): string {
  return from && from.startsWith("/") && !from.startsWith("//") && !from.startsWith("/\\") ? from : "/";
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/api") ||
    pathname.includes("favicon.ico") ||
    pathname.includes(".")
  ) {
    return NextResponse.next();
  }

  const token = request.cookies.get("kairso_access_token")?.value;

  const isPublicRoute = PUBLIC_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)
  );

  if (!token && !isPublicRoute) {
    const loginUrl = new URL("/login", request.url);
    if (pathname !== "/") {
      // Keep the query: the OAuth consent page needs its request parameters after login.
      loginUrl.searchParams.set("from", pathname + request.nextUrl.search);
    }
    return NextResponse.redirect(loginUrl);
  }

  if (token && (pathname === "/login" || pathname === "/forgot-password")) {
    return NextResponse.redirect(new URL(safeNext(request.nextUrl.searchParams.get("from")), request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    "/((?!api|_next/static|_next/image|favicon.ico).*)",
  ],
};
