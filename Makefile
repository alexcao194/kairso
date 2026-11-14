.PHONY: server-dev server-down server-log web-install web-dev web-build typecheck api

server-dev:
	$(MAKE) -C server dev

server-down:
	$(MAKE) -C server dev-down

server-log:
	$(MAKE) -C server log

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

typecheck:
	cd server && go vet ./...
	cd web && npm run typecheck

# Regenerate server/docs/swagger.json, then the typed client in web/lib/api.
api:
	$(MAKE) -C server swagger
	cd web && npm run gen:api
