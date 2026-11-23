# Kairso

**Kairso** is an open-source platform for teams to manage workspaces, projects, tasks, KPIs, attendance, workflows, meeting notes and quotes, with realtime updates and multilingual (vi/en) notifications.

- 💻 Source: **https://github.com/alexcao194/kairso**
- 🌐 Website & docs: **https://kairso.com** ([Install & self-host guide](https://kairso.com/install))
- 🚀 Hosted app: **https://app.kairso.com**
- 🐳 Image: **[`alexcao194/kairso`](https://hub.docker.com/r/alexcao194/kairso)** — all-in-one (web + API), multi-arch `amd64` + `arm64`

## Repository layout

| Path | What it is |
|---|---|
| [`server/`](server) | Go API server: REST API, realtime WebSocket hub, migrations ([README](server/README.md)) |
| [`web/`](web) | Next.js web app, with a signing proxy to the API ([README](web/README.md)) |
| [`deploy/selfhost/`](deploy/selfhost) | The `docker-compose.yml` and `.env.example` self-hosters copy |
| [`deploy/image/`](deploy/image) | Entrypoint and nginx config of the all-in-one image (built from the root `Dockerfile`) |
| [`deploy/dockerhub/`](deploy/dockerhub) | Docker Hub overview page |

Both apps are versioned together: a `v*` tag builds and deploys both, and publishes one image, `kairso`, with that version.

## Self-host

Self-hosting uses the prebuilt image from Docker Hub; no need to clone this repo. Follow the guide on [Docker Hub](https://hub.docker.com/r/alexcao194/kairso) or at https://kairso.com/install: one `docker-compose.yml`, one `.env`, then `docker compose up -d`.

This repository is for developing Kairso.

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

Bug reports and feature ideas go to [GitHub Issues](https://github.com/alexcao194/kairso/issues); pull requests are welcome. See [server/CONTRIBUTING.md](server/CONTRIBUTING.md) for conventions.
