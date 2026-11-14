# Kairso — Web

The web front end for **Kairso**, a platform for managing workspaces, projects, KPIs, attendance and initiatives for teams. This is the `web/` folder of the [Kairso monorepo](../README.md): the Next.js app. It also proxies REST calls to the backend so the signing secret never reaches the browser.

- 🌐 Website & docs: **https://home.kairso.com** (see the [Install & self-host guide](https://home.kairso.com/install))
- 🚀 Hosted app: **https://app.kairso.com**
- 🧩 Backend: **[`server/`](../server)** — the other half of the stack
- 🐳 Image: **[`alexcao194/kairso`](https://hub.docker.com/r/alexcao194/kairso)** — all-in-one (web + API), multi-arch `amd64` + `arm64`

## Stack

| | |
|---|---|
| Framework | Next.js 16 (App Router, Turbopack) |
| UI | React 19 + TypeScript |
| Data | React Query + a generated, type-safe API client |
| Auth | Firebase |
| API access | Same-origin proxy at `/api/proxy` that signs `X-App-Token` (HMAC) |

The dev server runs on `:3000`.

---

## Run from source (development)

### Prerequisites
- [Node.js 20+](https://nodejs.org/) and npm
- A running **backend** — either a local [`server/`](../server) or the hosted one at `https://app.kairso.com`
- A **Firebase project** for authentication

### Steps

```bash
# 1. Install dependencies
npm install

# 2. Create a local env file
cp .env.example .env.local
#    point NEXT_PUBLIC_BASE_URL / NEXT_PUBLIC_WS_URL at your backend,
#    and set APP_SIGNATURE_SECRET to match the backend's APP_TOKEN_SECRET

# 3. Generate the API client from the backend's Swagger
#    (reads ../server/docs/swagger.json from the same repo)
npm run gen:api

# 4. Start the dev server
npm run dev
```

Open http://localhost:3000.

### Scripts

| Script | Does |
|---|---|
| `npm run dev` | Start the dev server (`:3000`) |
| `npm run build` | Production build |
| `npm run start` | Serve the production build |
| `npm run gen:api` | Regenerate `lib/api/*` from the backend Swagger |
| `npm run typecheck` | `tsc --noEmit` |

### Key environment variables

| Variable | Purpose |
|---|---|
| `NEXT_PUBLIC_BASE_URL` | Backend REST base the proxy forwards to |
| `NEXT_PUBLIC_WS_URL` | Realtime WebSocket URL |
| `APP_SIGNATURE_SECRET` | HMAC shared with the backend — must match its `APP_TOKEN_SECRET` |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` / `SERVER_CLIENT_ID` | Google sign-in |

---

## Self-host

The web app ships together with the API in the all-in-one image [`alexcao194/kairso`](https://hub.docker.com/r/alexcao194/kairso) on Docker Hub — follow the guide there or at https://home.kairso.com/install.

The image is configured **at runtime**: on startup it writes `/env-config.js` from environment variables, so Firebase config and the WebSocket origin are injected when the container starts, not baked into the bundle, and the same image runs on any domain.
