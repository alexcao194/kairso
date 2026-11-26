# kairso

**Kairso** is an open-source platform for teams: workspaces, projects, tasks, KPIs, attendance, workflows, meeting notes and quotes, with realtime updates and vi/en notifications.

This is the **all-in-one image**: the web app, the API and realtime WebSocket in one container behind a single port. It applies database migrations on start and generates its own secrets.

- **Multi-arch:** `linux/amd64`, `linux/arm64`
- **Tags:** `latest`, and one per release (e.g. `v0.2.0`)
- **Docs:** https://kairso.com/install

## Quick start

You need Docker with the Compose plugin and a Firebase project for sign-in.

1. Create a folder (e.g. `kairso/`) with three files:

**`docker-compose.yml`**
```yaml
# Kairso self-host: the all-in-one Kairso image plus PostgreSQL, Redis and MinIO.
# Put this file next to .env and firebase-credentials.json, then: docker compose up -d
# Using your own PostgreSQL / S3 / Redis? Delete those services (and depends_on) and
# set DB_* / MINIO_* / REDIS_* in .env.
name: kairso

services:
  kairso:
    image: alexcao194/kairso:${KAIRSO_TAG:-latest}
    restart: unless-stopped
    ports:
      - "${PUBLIC_PORT:-8080}:80"
    env_file: .env
    volumes:
      - kairso-data:/data
      - ./firebase-credentials.json:/etc/kairso/firebase-credentials.json:ro
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
      minio: { condition: service_started }

  postgres:
    image: postgres:15-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: kairso
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: kairso
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U kairso"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:alpine
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  minio:
    image: bitnamilegacy/minio:latest
    restart: unless-stopped
    environment:
      MINIO_ROOT_USER: ${MINIO_ACCESS_KEY}
      MINIO_ROOT_PASSWORD: ${MINIO_SECRET_KEY}
      MINIO_DEFAULT_BUCKETS: kairso:download
      MINIO_API_CORS_ALLOW_ORIGIN: "*"
    volumes:
      - minio-data:/bitnami/minio/data

volumes:
  kairso-data:
  postgres-data:
  minio-data:
```

**`.env`** — fill in the passwords and your Firebase web config (Project settings → Your apps → SDK config)
```env
# Kairso self-host settings. Fill in the passwords and the Firebase web config.
PUBLIC_URL=http://localhost:8080
PUBLIC_PORT=8080

DB_PASSWORD=change-me
MINIO_ACCESS_KEY=kairso
MINIO_SECRET_KEY=change-me-please

# Firebase > Project settings > Your apps > SDK config
FIREBASE_API_KEY=
FIREBASE_AUTH_DOMAIN=
FIREBASE_PROJECT_ID=
FIREBASE_STORAGE_BUCKET=
FIREBASE_MESSAGING_SENDER_ID=
FIREBASE_APP_ID=

# Optional
# KAIRSO_TAG=v0.2.0
# MAX_UPLOAD_SIZE=200m
# FIREBASE_VAPID_KEY=
```

**`firebase-credentials.json`** — your Firebase service account (Project settings → Service accounts → Generate new private key)

2. Start it:
```bash
docker compose up -d
```

Open `http://localhost:8080` (or your `PUBLIC_URL`), and add that domain under Firebase **Authentication → Settings → Authorized domains**.

Upgrade with `docker compose pull && docker compose up -d`. Data lives in Docker volumes and survives upgrades.

## Bring your own PostgreSQL / S3 / Redis

Delete the `postgres`, `redis` and/or `minio` services (and their `depends_on` entries), then point Kairso at yours in `.env`:

```env
DB_HOST=your-db-host        # PostgreSQL 15+
DB_USER=kairso
DB_NAME=kairso
DB_SSLMODE=require
MINIO_ENDPOINT=s3.amazonaws.com   # any S3-compatible storage, host[:port]
MINIO_USE_SSL=true
MINIO_BUCKET=your-bucket
MINIO_PUBLIC_URL=https://your-bucket.s3.amazonaws.com
REDIS_HOST=your-redis
```

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `PUBLIC_URL` | `http://localhost:8080` | URL users open |
| `DB_HOST` `DB_PORT` `DB_USER` `DB_PASSWORD` `DB_NAME` `DB_SSLMODE` | `postgres` `5432` `kairso` — `kairso` `disable` | PostgreSQL 15+ (`DB_PASSWORD` required) |
| `REDIS_HOST` `REDIS_PORT` | `redis` `6379` | Redis |
| `MINIO_ENDPOINT` `MINIO_ACCESS_KEY` `MINIO_SECRET_KEY` `MINIO_BUCKET` `MINIO_USE_SSL` | `minio:9000` — — `kairso` `false` | S3-compatible storage (keys required) |
| `MINIO_PUBLIC_URL` | `<PUBLIC_URL>/cdn` | Public URL of the bucket; unset = served through the container |
| `FIREBASE_API_KEY` `FIREBASE_AUTH_DOMAIN` `FIREBASE_PROJECT_ID` `FIREBASE_STORAGE_BUCKET` `FIREBASE_MESSAGING_SENDER_ID` `FIREBASE_APP_ID` `FIREBASE_VAPID_KEY` | — | Firebase web config |
| `FIREBASE_CREDENTIALS_JSON` / `FIREBASE_CREDENTIALS_PATH` | `/etc/kairso/firebase-credentials.json` | Firebase service account (content or path) |
| `JWT_SECRET` `APP_TOKEN_SECRET` | generated into `/data/secrets.env` | Session and internal signing secrets |
| `MAX_UPLOAD_SIZE` | `200m` | Upload size limit |
| `PDF_WEB_URL` | `http://127.0.0.1:3000` | Web app opened by the bundled Chromium for PDF export; empty disables PDF export |

Behind HTTPS: put Caddy, Traefik or nginx in front of `PUBLIC_PORT` and set `PUBLIC_URL=https://your-domain`.

## Source code

Kairso is open source: https://github.com/alexcao194/kairso
