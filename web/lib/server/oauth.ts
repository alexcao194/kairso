import { NextRequest, NextResponse } from "next/server";

// Server-only helpers for the OAuth / MCP routes.

export const BACKEND_ORIGIN = new URL(process.env.NEXT_PUBLIC_BASE_URL || "https://api.kairso.com/api").origin;

/** The origin users reach the web app at, also behind a reverse proxy. */
export function publicOrigin(req: NextRequest): string {
  const configured = process.env.KAIRSO_PUBLIC_URL || process.env.PUBLIC_URL;
  if (configured) return configured.replace(/\/+$/, "");
  const host = req.headers.get("x-forwarded-host")?.split(",")[0].trim() || req.headers.get("host") || req.nextUrl.host;
  const proto = req.headers.get("x-forwarded-proto")?.split(",")[0].trim() || req.nextUrl.protocol.replace(":", "");
  return `${proto}://${host}`;
}

export const mcpUrl = (req: NextRequest) => `${publicOrigin(req)}/api/mcp`;
export const resourceMetadataUrl = (req: NextRequest) => `${publicOrigin(req)}/.well-known/oauth-protected-resource/api/mcp`;

/** OAuth endpoints are called by other apps (some from a browser), so they allow any origin. */
export const CORS_HEADERS: Record<string, string> = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  "Access-Control-Allow-Headers": "Authorization, Content-Type, Mcp-Protocol-Version, Mcp-Session-Id",
  "Access-Control-Expose-Headers": "WWW-Authenticate, Mcp-Session-Id",
};

export function corsPreflight() {
  return new NextResponse(null, { status: 204, headers: CORS_HEADERS });
}

export function jsonWithCors(body: unknown, status = 200) {
  return NextResponse.json(body, { status, headers: { ...CORS_HEADERS, "Cache-Control": "no-store" } });
}

/** Forwards a request body to the API server and relays the answer. */
export async function forwardToBackend(req: NextRequest, path: string) {
  try {
    const upstream = await fetch(`${BACKEND_ORIGIN}${path}`, {
      method: "POST",
      headers: { "content-type": req.headers.get("content-type") || "application/json", accept: "application/json" },
      body: await req.arrayBuffer(),
      cache: "no-store",
    });
    return new NextResponse(await upstream.arrayBuffer(), {
      status: upstream.status,
      headers: { ...CORS_HEADERS, "Content-Type": upstream.headers.get("content-type") || "application/json", "Cache-Control": "no-store" },
    });
  } catch {
    return jsonWithCors({ error: "temporarily_unavailable", error_description: "Could not reach the API server" }, 503);
  }
}
