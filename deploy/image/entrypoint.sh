#!/bin/sh
# Entrypoint of the all-in-one Kairso image: fills in defaults, runs migrations, then
# supervises the API (:8080), the web app (:3000) and nginx (:80). If any of them
# exits, the container stops so Docker's restart policy can bring it back.
set -eu

log() { echo "[kairso] $*"; }
die() { echo "[kairso] ERROR: $*" >&2; exit 1; }

# ---- defaults ---------------------------------------------------------------
: "${PUBLIC_URL:=http://localhost:8080}"
PUBLIC_URL="${PUBLIC_URL%/}"
: "${DB_HOST:=postgres}"
: "${DB_PORT:=5432}"
: "${DB_USER:=kairso}"
: "${DB_NAME:=kairso}"
: "${DB_SSLMODE:=disable}"
: "${REDIS_HOST:=redis}"
: "${REDIS_PORT:=6379}"
: "${MINIO_ENDPOINT:=minio:9000}"
: "${MINIO_BUCKET:=kairso}"
: "${MINIO_USE_SSL:=false}"
: "${JWT_ACCESS_EXPIRATION:=15m}"
: "${JWT_REFRESH_EXPIRATION:=168h}"
: "${MAX_UPLOAD_SIZE:=200m}"
# PDFs: Times New Roman .ttf files put in /data/fonts (or PDF_FONT_DIR); else Liberation Serif.
: "${PDF_FONT_DIR:=/data/fonts}"
[ -n "${DB_PASSWORD:-}" ] || die "DB_PASSWORD is required"
[ -n "${MINIO_ACCESS_KEY:-}" ] && [ -n "${MINIO_SECRET_KEY:-}" ] || die "MINIO_ACCESS_KEY and MINIO_SECRET_KEY are required"

# Bundled MinIO is served on the same origin under /cdn; external S3 sets its own URL.
CDN_PROXY=0
if [ -z "${MINIO_PUBLIC_URL:-}" ]; then
  MINIO_PUBLIC_URL="$PUBLIC_URL/cdn"
  CDN_PROXY=1
fi

# ---- secrets: generated once and kept in the /data volume -----------------------
SECRETS=/data/secrets.env
if [ -z "${JWT_SECRET:-}" ] || [ -z "${APP_TOKEN_SECRET:-}" ]; then
  if [ ! -f "$SECRETS" ]; then
    log "generating JWT_SECRET and APP_TOKEN_SECRET into $SECRETS"
    umask 077
    printf 'GEN_JWT_SECRET=%s\nGEN_APP_TOKEN_SECRET=%s\n' "$(openssl rand -hex 32)" "$(openssl rand -hex 32)" > "$SECRETS"
  fi
  . "$SECRETS"
  : "${JWT_SECRET:=$GEN_JWT_SECRET}"
  : "${APP_TOKEN_SECRET:=$GEN_APP_TOKEN_SECRET}"
fi

# ---- Firebase -------------------------------------------------------------------
# Service account: a mounted file, or its JSON content in FIREBASE_CREDENTIALS_JSON.
if [ -n "${FIREBASE_CREDENTIALS_JSON:-}" ]; then
  FIREBASE_CREDENTIALS_PATH=/tmp/firebase-credentials.json
  printf '%s' "$FIREBASE_CREDENTIALS_JSON" > "$FIREBASE_CREDENTIALS_PATH"
  chmod 600 "$FIREBASE_CREDENTIALS_PATH"
fi
if [ -z "${FIREBASE_CREDENTIALS_PATH:-}" ]; then
  FIREBASE_CREDENTIALS_PATH=/etc/kairso/firebase-credentials.json
  [ -f "$FIREBASE_CREDENTIALS_PATH" ] || FIREBASE_CREDENTIALS_PATH=/data/firebase-credentials.json
fi
[ -f "$FIREBASE_CREDENTIALS_PATH" ] || die "Firebase service account not found: mount it at $FIREBASE_CREDENTIALS_PATH or set FIREBASE_CREDENTIALS_JSON"

export DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME DB_SSLMODE REDIS_HOST REDIS_PORT \
  MINIO_ENDPOINT MINIO_ACCESS_KEY MINIO_SECRET_KEY MINIO_BUCKET MINIO_USE_SSL MINIO_PUBLIC_URL \
  JWT_SECRET JWT_ACCESS_EXPIRATION JWT_REFRESH_EXPIRATION APP_TOKEN_SECRET FIREBASE_CREDENTIALS_PATH \
  PDF_FONT_DIR

# ---- migrations -------------------------------------------------------------------
DSN="host=$DB_HOST port=$DB_PORT user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME sslmode=$DB_SSLMODE"
i=0
until goose -dir /opt/kairso/server/db/migrations postgres "$DSN" up; do
  i=$((i + 1))
  [ "$i" -lt 30 ] || die "database migrations failed"
  log "database not ready, retrying in 2s ($i/30)"
  sleep 2
done

# ---- web runtime config (read by the browser from /env-config.js) -------------------
# FIREBASE_* is accepted as a shorter alias of NEXT_PUBLIC_FIREBASE_*.
cat > /opt/kairso/web/public/env-config.js <<EOF
window.__KAIRSO_ENV__ = {
  firebaseApiKey: "${NEXT_PUBLIC_FIREBASE_API_KEY:-${FIREBASE_API_KEY:-}}",
  firebaseAuthDomain: "${NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN:-${FIREBASE_AUTH_DOMAIN:-}}",
  firebaseProjectId: "${NEXT_PUBLIC_FIREBASE_PROJECT_ID:-${FIREBASE_PROJECT_ID:-}}",
  firebaseStorageBucket: "${NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET:-${FIREBASE_STORAGE_BUCKET:-}}",
  firebaseMessagingSenderId: "${NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID:-${FIREBASE_MESSAGING_SENDER_ID:-}}",
  firebaseAppId: "${NEXT_PUBLIC_FIREBASE_APP_ID:-${FIREBASE_APP_ID:-}}",
  firebaseVapidKey: "${NEXT_PUBLIC_FIREBASE_VAPID_KEY:-${FIREBASE_VAPID_KEY:-}}",
  wsUrl: "${NEXT_PUBLIC_WS_URL:-}"
};
EOF

# ---- nginx ----------------------------------------------------------------------------
CDN_BLOCK=""
if [ "$CDN_PROXY" = 1 ]; then
  CDN_BLOCK="        location /cdn/ {
            resolver 127.0.0.11 valid=30s;
            set \$minio_up \"$MINIO_ENDPOINT\";
            rewrite ^/cdn/(.*)\$ /\$1 break;
            proxy_pass http://\$minio_up;
            proxy_set_header Host \$host;
        }"
fi
mkdir -p /run/nginx
awk -v body="$MAX_UPLOAD_SIZE" -v cdn="$CDN_BLOCK" \
  '{ gsub(/__MAX_BODY__/, body); if ($0 ~ /__CDN_BLOCK__/) print cdn; else print }' \
  /opt/kairso/nginx.conf.template > /etc/nginx/nginx.conf
nginx -t -q

# ---- start and supervise ------------------------------------------------------------------
(cd /opt/kairso/server && exec ./server) &
API_PID=$!
(cd /opt/kairso/web && PORT=3000 HOSTNAME=127.0.0.1 NEXT_PUBLIC_BASE_URL=http://127.0.0.1:8080 \
  APP_SIGNATURE_SECRET="$APP_TOKEN_SECRET" exec node server.js) &
WEB_PID=$!
nginx -g 'daemon off;' &
NGINX_PID=$!

stop() { kill "$API_PID" "$WEB_PID" "$NGINX_PID" 2>/dev/null || true; wait; exit "${1:-0}"; }
trap 'stop 0' TERM INT

log "listening on :80 (public URL $PUBLIC_URL)"
while kill -0 "$API_PID" 2>/dev/null && kill -0 "$WEB_PID" 2>/dev/null && kill -0 "$NGINX_PID" 2>/dev/null; do
  sleep 2
done
log "a process exited, stopping"
stop 1
