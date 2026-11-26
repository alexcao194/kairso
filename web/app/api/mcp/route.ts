import { NextRequest, NextResponse } from "next/server";
import { BACKEND_ORIGIN, CORS_HEADERS, corsPreflight, resourceMetadataUrl } from "@/lib/server/oauth";

// Public MCP endpoint for AI assistants (Claude Code, Claude Desktop, Cursor…): forwards to the
// API server's /mcp. Callers authenticate there with a personal API key or an OAuth access
// token, so no app signature is added. A 401 points clients at the OAuth discovery document.

const FORWARDED_REQUEST = ["authorization", "content-type", "accept", "mcp-protocol-version", "mcp-session-id"];
const FORWARDED_RESPONSE = ["content-type", "allow", "mcp-session-id"];

async function forward(req: NextRequest) {
  const headers: Record<string, string> = {};
  for (const name of FORWARDED_REQUEST) {
    const value = req.headers.get(name);
    if (value) headers[name] = value;
  }
  try {
    const upstream = await fetch(`${BACKEND_ORIGIN}/mcp`, {
      method: req.method,
      headers,
      body: req.method === "POST" ? await req.arrayBuffer() : undefined,
      cache: "no-store",
    });
    const out = new Headers(CORS_HEADERS);
    for (const name of FORWARDED_RESPONSE) {
      const value = upstream.headers.get(name);
      if (value) out.set(name, value);
    }
    if (upstream.status === 401) {
      const invalid = req.headers.has("authorization") ? `, error="invalid_token"` : "";
      out.set("WWW-Authenticate", `Bearer resource_metadata="${resourceMetadataUrl(req)}"${invalid}`);
    }
    const empty = upstream.status === 202 || upstream.status === 204;
    return new NextResponse(empty ? null : await upstream.arrayBuffer(), { status: upstream.status, headers: out });
  } catch (err) {
    const message = err instanceof Error ? err.message : "Could not reach the backend";
    return NextResponse.json({ error: message }, { status: 502, headers: CORS_HEADERS });
  }
}

export const POST = forward;
export const GET = forward;
export const DELETE = forward;
export const OPTIONS = corsPreflight;
