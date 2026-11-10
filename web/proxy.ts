import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Public routes accessible without authentication
const PUBLIC_PREFIXES = [
  "/login",
  "/forgot-password",
  "/f", // Public feedback form: /f/[token]
];

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // 1. Bypass Next.js internal files, static assets, and api proxy
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

  // 2. Unauthenticated user trying to access protected route -> Redirect to /login
  if (!token && !isPublicRoute) {
    const loginUrl = new URL("/login", request.url);
    if (pathname !== "/") {
      loginUrl.searchParams.set("from", pathname);
    }
    return NextResponse.redirect(loginUrl);
  }

  // 3. Authenticated user trying to access /login or /forgot-password -> Redirect to home
  if (token && (pathname === "/login" || pathname === "/forgot-password")) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon)
     * - api routes
     */
    "/((?!api|_next/static|_next/image|favicon.ico).*)",
  ],
};
