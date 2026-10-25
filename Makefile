# HomeHub monorepo tasks. Services live in services/<name>.
# Usage: make run SERVICE=portal

SERVICE ?= portal
HTMX_VERSION ?= 2.0.4
# Every module listed in go.work, as package patterns (new services are picked up automatically).
MODS = $(shell awk '/^\t\.\//{print $$1"/..."}' go.work)

.PHONY: help build run test vet fmt vendor image lint-chart deploy

help:
	@echo "make build  SERVICE=portal   build bin/<service>"
	@echo "make run    SERVICE=portal   run a service locally"
	@echo "make test                    run all tests (with -race)"
	@echo "make vet                     go vet everything"
	@echo "make fmt                     gofmt everything"
	@echo "make vendor                  download HTMX into the portal's static assets"
	@echo "make image  SERVICE=portal   build the container image locally (homehub/<service>:dev)"
	@echo "make lint-chart SERVICE=...  helm lint a service's chart"
	@echo "make deploy SERVICE=portal   helm upgrade --install into the cluster (run on the Pi)"

build:
	go build -o bin/$(SERVICE) ./services/$(SERVICE)/cmd/$(SERVICE)

# The portal seeds placeholder cards locally so the dashboard is not empty.
run:
	SEED_DEMO=true POLL_INTERVAL=5s go run ./services/$(SERVICE)/cmd/$(SERVICE)

test:
	go test -race $(MODS)

vet:
	go vet $(MODS)

fmt:
	gofmt -w shared services

# One-off: fetch the pinned HTMX build and commit the result.
vendor:
	curl -fsSL -o services/portal/web/static/htmx.min.js \
		https://cdn.jsdelivr.net/npm/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js
	@wc -c services/portal/web/static/htmx.min.js

# Local image for the host architecture. CI publishes the real multi-arch images.
image:
	docker build -f services/$(SERVICE)/Dockerfile --build-arg SERVICE=$(SERVICE) -t homehub/$(SERVICE):dev .

lint-chart:
	helm lint services/$(SERVICE)/charts/$(SERVICE)

# Installs or upgrades the release using the image published to GHCR.
deploy:
	helm upgrade --install $(SERVICE) services/$(SERVICE)/charts/$(SERVICE) --wait --timeout 3m
