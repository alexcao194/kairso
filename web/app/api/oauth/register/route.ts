import { NextRequest } from "next/server";
import { corsPreflight, forwardToBackend } from "@/lib/server/oauth";

// RFC 7591 dynamic client registration, handled by the API server.
export const POST = (req: NextRequest) => forwardToBackend(req, "/oauth/register");
export const OPTIONS = corsPreflight;
