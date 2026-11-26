import { readFile } from "fs/promises";
import path from "path";
import { NextRequest, NextResponse } from "next/server";
import { buildExtension } from "@/lib/server/mcpExtension";
import { mcpUrl, publicOrigin } from "@/lib/server/oauth";

// Claude Desktop extension download: a .mcpb bundle pre-filled with this server's MCP URL.
// Opening the file in Claude Desktop shows its install dialog, which asks for the API key.
export async function GET(req: NextRequest) {
  let icon: Uint8Array | undefined;
  try {
    icon = new Uint8Array(await readFile(path.join(process.cwd(), "public", "icon-512.png")));
  } catch {
    icon = undefined;
  }
  const data = buildExtension({ mcpUrl: mcpUrl(req), homepage: publicOrigin(req), icon });
  return new NextResponse(Buffer.from(data), {
    headers: {
      "Content-Type": "application/zip",
      "Content-Disposition": `attachment; filename="kairso.mcpb"`,
      "Cache-Control": "no-store",
    },
  });
}
