# kairso

**Kairso** is an open-source platform for teams: workspaces, projects, tasks, KPIs, attendance, workflows, meeting notes and quotes, with realtime updates and vi/en notifications.

This is the **all-in-one image**: the web app, the API and realtime WebSocket in one container behind a single port. It applies database migrations on start and generates its own secrets.

- **Multi-arch:** `linux/amd64`, `linux/arm64`
- **Tags:** `latest`, and one per release (e.g. `v0.2.0`)
- **Source:** https://github.com/alexcao194/kairso · **Docs:** https://home.kairso.com/install

## Quick start (with bundled PostgreSQL, Redis and MinIO)

```bash
git clone https://github.com/alexcao194/kairso && cd kairso/deploy/selfhost
cp .env.example .env            # set passwords + Firebase web config
cp <service-account>.json firebase-credentials.json
docker compose up -d
```

Open `http://localhost:8080`.

## Run the container directly

```bash
docker run -d -p 8080:80 -v kairso-data:/data \
  -e PUBLIC_URL=http://localhost:8080 \
  -e DB_HOST=db -e DB_PASSWORD=... \
  -e REDIS_HOST=redis \
  -e MINIO_ENDPOINT=minio:9000 -e MINIO_ACCESS_KEY=... -e MINIO_SECRET_KEY=... \
  -e FIREBASE_API_KEY=... -e FIREBASE_AUTH_DOMAIN=... -e FIREBASE_PROJECT_ID=... -e FIREBASE_APP_ID=... \
  -e FIREBASE_CREDENTIALS_JSON="$(cat service-account.json)" \
  alexcao194/kairso
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
| `FIREBASE_CREDENTIALS_JSON` / `FIREBASE_CREDENTIALS_PATH` | `/etc/kairso/firebase-credentials.json`, then `/data/firebase-credentials.json` | Firebase service account (content or path) |
| `JWT_SECRET` `APP_TOKEN_SECRET` | generated into `/data/secrets.env` | Session and internal signing secrets |
| `MAX_UPLOAD_SIZE` | `200m` | Upload size limit |

Keep `/data` on a volume so the generated secrets survive restarts.
