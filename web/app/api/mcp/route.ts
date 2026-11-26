import { NextRequest, NextResponse } from "next/server";

// Public MCP endpoint for AI assistants (Claude Code, Cursor…): forwards to the API server's
// /mcp. Callers authenticate there with a personal API key, so no app signature is added.
const BACKEND_BASE = process.env.NEXT_PUBLIC_BASE_URL || "https://api.kairso.com/api";

const FORWARDED_REQUEST = ["authorization", "content-type", "accept", "mcp-protocol-version", "mcp-session-id"];
const FORWARDED_RESPONSE = ["content-type", "www-authenticate", "allow", "mcp-session-id"];

async function forward(req: NextRequest) {
  const headers: Record<string, string> = {};
  for (const name of FORWARDED_REQUEST) {
    const value = req.headers.get(name);
    if (value) headers[name] = value;
  }
  try {
    const upstream = await fetch(`${new URL(BACKEND_BASE).origin}/mcp`, {
      method: req.method,
      headers,
      body: req.method === "POST" ? await req.arrayBuffer() : undefined,
      cache: "no-store",
    });
    const out = new Headers();
    for (const name of FORWARDED_RESPONSE) {
      const value = upstream.headers.get(name);
      if (value) out.set(name, value);
    }
    const empty = upstream.status === 202 || upstream.status === 204;
    return new NextResponse(empty ? null : await upstream.arrayBuffer(), { status: upstream.status, headers: out });
  } catch (err) {
    const message = err instanceof Error ? err.message : "Could not reach the backend";
    return NextResponse.json({ error: message }, { status: 502 });
  }
}

export const POST = forward;
export const GET = forward;
export const DELETE = forward;
