import { NextRequest } from "next/server";
import { corsPreflight, jsonWithCors, publicOrigin } from "@/lib/server/oauth";

// RFC 8414 authorization server metadata (served at /.well-known/oauth-authorization-server).
export function GET(req: NextRequest) {
  const origin = publicOrigin(req);
  return jsonWithCors({
    issuer: origin,
    authorization_endpoint: `${origin}/oauth/authorize`,
    token_endpoint: `${origin}/api/oauth/token`,
    registration_endpoint: `${origin}/api/oauth/register`,
    response_types_supported: ["code"],
    grant_types_supported: ["authorization_code", "refresh_token"],
    code_challenge_methods_supported: ["S256"],
    token_endpoint_auth_methods_supported: ["none"],
    scopes_supported: ["mcp"],
    service_documentation: `${origin}/settings/ai`,
  });
}

export const OPTIONS = corsPreflight;
