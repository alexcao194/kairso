# Kairso — self-host

The files here are what self-hosters use: `docker-compose.yml` and `.env.example`.
They run the published [`alexcao194/kairso`](https://hub.docker.com/r/alexcao194/kairso)
image from Docker Hub plus PostgreSQL, Redis and MinIO — nothing is built from source.

The step-by-step guide lives on the [Docker Hub page](https://hub.docker.com/r/alexcao194/kairso)
(source: [`../dockerhub/kairso.md`](../dockerhub/kairso.md)) and at
https://kairso.com/install. Keep all three in sync when these files change.

To try a local build of the image instead, run `docker build -t alexcao194/kairso:dev .`
from the repo root and set `KAIRSO_TAG=dev` in `.env`.
