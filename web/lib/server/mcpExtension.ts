// Builds the Claude Desktop extension (.mcpb): a manifest plus a tiny Node bridge that speaks
// MCP over stdio to Claude Desktop and forwards each message to Kairso's HTTP MCP endpoint with
// the user's API key. Claude Desktop ships its own Node runtime and stores the key (marked
// sensitive) in the OS keychain.
import { zip, type ZipEntry } from "./zip";

export const EXTENSION_VERSION = "1.1.0";

const BRIDGE = String.raw`#!/usr/bin/env node
// Kairso MCP bridge: stdio (Claude Desktop) <-> Streamable HTTP (Kairso /api/mcp).
"use strict";
const readline = require("readline");

const ENDPOINT = process.env.KAIRSO_MCP_URL;
const KEY = (process.env.KAIRSO_API_KEY || "").trim();
const VERSION = process.env.KAIRSO_BRIDGE_VERSION || "1";

let pending = 0;
let closing = false;

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + "\n");
}

function requestsIn(msg) {
  return (Array.isArray(msg) ? msg : [msg]).filter((m) => m && typeof m === "object" && m.id !== undefined && m.id !== null && m.method);
}

function fail(msg, message) {
  for (const m of requestsIn(msg)) send({ jsonrpc: "2.0", id: m.id, error: { code: -32000, message } });
}

function errorText(status, body) {
  try {
    const j = JSON.parse(body);
    if (j && j.error) return typeof j.error === "string" ? j.error : JSON.stringify(j.error);
  } catch (_) {}
  return body.slice(0, 300) || "HTTP " + status;
}

async function forward(line, msg) {
  if (!ENDPOINT) return fail(msg, "Kairso extension is missing its server URL. Download it again from Kairso.");
  if (!KEY) return fail(msg, "Kairso API key is not set. Open Claude Desktop > Settings > Extensions > Kairso and paste a key from Kairso Settings > AI & API keys.");
  let res;
  try {
    res = await fetch(ENDPOINT, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        Authorization: "Bearer " + KEY,
        "User-Agent": "kairso-mcpb/" + VERSION,
      },
      body: line,
    });
  } catch (e) {
    return fail(msg, "Could not reach Kairso at " + ENDPOINT + ": " + (e && e.message ? e.message : e));
  }
  if (res.status === 202 || res.status === 204) return;
  const body = await res.text();
  if (!res.ok) {
    const hint = res.status === 401 ? " Check the API key in Claude Desktop > Settings > Extensions > Kairso." : "";
    return fail(msg, "Kairso returned HTTP " + res.status + ": " + errorText(res.status, body) + hint);
  }
  try {
    send(JSON.parse(body));
  } catch (_) {
    fail(msg, "Kairso sent an invalid response");
  }
}

const rl = readline.createInterface({ input: process.stdin });
rl.on("line", async (raw) => {
  const line = raw.trim();
  if (!line) return;
  let msg;
  try {
    msg = JSON.parse(line);
  } catch (_) {
    return send({ jsonrpc: "2.0", id: null, error: { code: -32700, message: "Parse error" } });
  }
  pending++;
  try {
    await forward(line, msg);
  } finally {
    pending--;
    if (closing && pending === 0) process.exit(0);
  }
});
rl.on("close", () => {
  closing = true;
  if (pending === 0) process.exit(0);
});
`;

export function buildExtension(opts: { mcpUrl: string; homepage: string; icon?: Uint8Array }): Uint8Array {
  const manifest = {
    manifest_version: "0.2",
    name: "kairso",
    display_name: "Kairso",
    version: EXTENSION_VERSION,
    description: "Manage tasks, meeting minutes, quotes, customer feedback and daily reports in Kairso.",
    long_description:
      "Connects Claude to your Kairso workspace. Claude can browse workspaces, projects, members and modules; search, create, edit, label and assign tasks; manage modules and sprints; write meeting minutes; prepare, send and export quotes; triage and answer customer feedback; write end-of-day reports. It acts with your API key, so it can only do what you are allowed to do.",
    author: { name: "Kairso", url: opts.homepage },
    homepage: opts.homepage,
    ...(opts.icon ? { icon: "icon.png" } : {}),
    server: {
      type: "node",
      entry_point: "server/index.js",
      mcp_config: {
        command: "node",
        args: ["${__dirname}/server/index.js"],
        env: {
          KAIRSO_MCP_URL: opts.mcpUrl,
          KAIRSO_API_KEY: "${user_config.api_key}",
          KAIRSO_BRIDGE_VERSION: EXTENSION_VERSION,
        },
      },
    },
    tools_generated: true,
    user_config: {
      api_key: {
        type: "string",
        title: "Kairso API key",
        description: "Create one in Kairso: Settings > AI & API keys (starts with ksk_).",
        sensitive: true,
        required: true,
      },
    },
    compatibility: {
      platforms: ["darwin", "win32", "linux"],
      runtimes: { node: ">=18.0.0" },
    },
  };
  const entries: ZipEntry[] = [
    { name: "manifest.json", data: JSON.stringify(manifest, null, 2) },
    { name: "server/index.js", data: BRIDGE },
  ];
  if (opts.icon) entries.push({ name: "icon.png", data: opts.icon });
  return zip(entries);
}
