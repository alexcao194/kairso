# Kairso — self-host

Runs the whole of Kairso with one `docker compose up`. The app is a single
all-in-one image, [`alexcao194/kairso`](https://hub.docker.com/r/alexcao194/kairso)
(web + API + realtime). PostgreSQL, Redis and MinIO are bundled next to it.

## Requirements
- Docker with the Compose plugin
- A **Firebase project of your own** for sign-in (see below)

## Quick start
```bash
cp .env.example .env
nano .env                      # set the passwords and the 6 FIREBASE_* values
cp ~/Downloads/<service-account>.json firebase-credentials.json
mkdir -p data/minio && sudo chown -R 1001:1001 data/minio   # MinIO runs as uid 1001
docker compose up -d
```
Open **http://localhost:8080** (or your `PUBLIC_URL`).

On start the container applies database migrations and, unless you set them,
generates `JWT_SECRET` and `APP_TOKEN_SECRET` into `data/kairso/secrets.env`.
All data lives in `./data/` and survives upgrades.

## Firebase (required for sign-in)
1. Create a project at <https://console.firebase.google.com> and enable
   **Authentication → Google** (and/or Email).
2. **Web config** (Project settings → Your apps → SDK config) → the `FIREBASE_*` values in `.env`.
3. **Service account** (Project settings → Service accounts → Generate new private key)
   → save as `./firebase-credentials.json`. Alternatively put the JSON content in
   `FIREBASE_CREDENTIALS_JSON`.
4. Add your domain under **Authentication → Settings → Authorized domains**.

## Bring your own PostgreSQL / S3 / Redis
Set `COMPOSE_FILE=docker-compose.yml` in `.env` (only the Kairso container starts), then:

```
DB_HOST=your-db-host            # PostgreSQL 15+ (RDS, Supabase, Neon…)
DB_PORT=5432
DB_USER=kairso
DB_NAME=kairso
DB_SSLMODE=require
MINIO_ENDPOINT=s3.amazonaws.com # any S3-compatible: S3, R2, B2, Wasabi, MinIO…
MINIO_USE_SSL=true
MINIO_BUCKET=your-bucket
MINIO_PUBLIC_URL=https://your-bucket.s3.amazonaws.com
REDIS_HOST=your-redis
```

## How it fits together
The browser only talks to the container's port 80:
- `/` → web app, which calls the API internally through `/api/proxy` (HMAC-signed)
- `/ws` → API realtime WebSocket (same origin)
- `/cdn/` → bundled MinIO (files and images), when `MINIO_PUBLIC_URL` is not set

Firebase config is injected at runtime via `/env-config.js`, so the same image runs
on any domain without rebuilding.

## Upgrade
```bash
docker compose pull && docker compose up -d
```

## HTTPS / reverse proxy
Put Caddy, Traefik or nginx in front, point it at `PUBLIC_PORT`, and set
`PUBLIC_URL=https://your-domain`. Nothing else changes (everything is same-origin).
