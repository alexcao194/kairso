# Kairso

**Kairso** is an open-source platform for teams to manage workspaces, projects, tasks, KPIs, attendance, workflows, meeting notes and quotes, with realtime updates and multilingual (vi/en) notifications.

- 🌐 Website & docs: **https://home.kairso.com** ([Install & self-host guide](https://home.kairso.com/install))
- 🚀 Hosted app: **https://app.kairso.com**
- 🐳 Images: [`alexcao194/kairso-server`](https://hub.docker.com/r/alexcao194/kairso-server) · [`alexcao194/kairso-web`](https://hub.docker.com/r/alexcao194/kairso-web) (multi-arch `amd64` + `arm64`)

## Repository layout

| Path | What it is |
|---|---|
| [`server/`](server) | Go API server: REST API, realtime WebSocket hub, migrations ([README](server/README.md)) |
| [`web/`](web) | Next.js web app, with a signing proxy to the API ([README](web/README.md)) |
| [`deploy/selfhost/`](deploy/selfhost) | Docker Compose bundle to self-host the whole stack from the published images |
| [`deploy/dockerhub/`](deploy/dockerhub) | Docker Hub overview pages for both images |

Both apps are versioned together: a `v*` tag builds, deploys and publishes `kairso-server` and `kairso-web` with the same version.

## Self-host with Docker

```bash
cd deploy/selfhost
cp .env.example .env         # fill in secrets + Firebase
docker compose up -d         # migrations run automatically
```

Open `http://localhost:8080` (or your `PUBLIC_URL`). See [`deploy/selfhost/README.md`](deploy/selfhost/README.md) for bringing your own PostgreSQL / S3 storage and HTTPS.

## Develop from source

Requires Go 1.25+, Node.js 20+, Docker and a Firebase project.

```bash
make server-dev      # backend stack in Docker (PostgreSQL, Redis, MinIO, API with hot reload)
make web-install     # npm install in web/
make web-dev         # Next.js dev server on :3000
make api             # after changing API handlers: regenerate Swagger, then the web API client
```

See each app's README for environment variables and details.

## Contributing

See [server/CONTRIBUTING.md](server/CONTRIBUTING.md).
