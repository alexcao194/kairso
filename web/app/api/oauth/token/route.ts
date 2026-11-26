import { NextRequest } from "next/server";
import { corsPreflight, forwardToBackend } from "@/lib/server/oauth";

// OAuth token endpoint (authorization_code + refresh_token), handled by the API server.
export const POST = (req: NextRequest) => forwardToBackend(req, "/oauth/token");
export const OPTIONS = corsPreflight;
