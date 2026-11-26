import { NextRequest } from "next/server";
import { corsPreflight, jsonWithCors, mcpUrl, publicOrigin } from "@/lib/server/oauth";

// RFC 9728 protected resource metadata for /api/mcp (served at /.well-known/oauth-protected-resource).
export function GET(req: NextRequest) {
  return jsonWithCors({
    resource: mcpUrl(req),
    authorization_servers: [publicOrigin(req)],
    bearer_methods_supported: ["header"],
    scopes_supported: ["mcp"],
    resource_name: "Kairso",
  });
}

export const OPTIONS = corsPreflight;
