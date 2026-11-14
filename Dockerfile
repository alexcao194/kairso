# All-in-one Kairso image (published as <namespace>/kairso on Docker Hub).
# One container runs the Go API, the Next.js web app and an nginx front door on :80,
# applies database migrations on start and generates its own secrets when none are set.
# Build from the repo root:  docker buildx build --platform linux/amd64,linux/arm64 -t kairso .

# ---- server (cross-compiled natively on the build platform)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildvcs=false -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go install github.com/pressly/goose/v3/cmd/goose@v3.26.0 \
 && GOOSE_BIN="$(go env GOPATH)/bin/${TARGETOS}_${TARGETARCH}/goose" \
 && if [ ! -f "$GOOSE_BIN" ]; then GOOSE_BIN="$(go env GOPATH)/bin/goose"; fi \
 && cp "$GOOSE_BIN" /out/goose

# ---- web
FROM node:20-alpine AS web-deps
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci

FROM node:20-alpine AS web
WORKDIR /app
COPY --from=web-deps /app/node_modules ./node_modules
COPY web/ .
ENV NEXT_TELEMETRY_DISABLED=1 NODE_ENV=production
RUN rm -f .env .env.production .env.local && npm run build

# ---- runtime
FROM node:20-alpine
RUN apk add --no-cache nginx tzdata openssl
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1
WORKDIR /opt/kairso

COPY --from=server /out/server /opt/kairso/server/server
COPY --from=server /out/goose /usr/local/bin/goose
COPY server/db/migrations /opt/kairso/server/db/migrations

COPY --from=web /app/.next/standalone /opt/kairso/web/
COPY --from=web /app/.next/static /opt/kairso/web/.next/static
COPY --from=web /app/public /opt/kairso/web/public

COPY deploy/image/nginx.conf /opt/kairso/nginx.conf.template
COPY deploy/image/entrypoint.sh /usr/local/bin/kairso-entrypoint
RUN chmod +x /usr/local/bin/kairso-entrypoint && mkdir -p /data

VOLUME /data
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s CMD wget -qO- http://127.0.0.1/healthz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/kairso-entrypoint"]
