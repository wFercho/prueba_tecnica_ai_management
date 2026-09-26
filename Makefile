# The Docker way is the default way. Every target below that runs Go or Node runs it
# in a container, so a clean checkout needs only Docker.
#
#   make demo      levanta la pila e importa los datos, sin analizar
#   make up        database and API
#   make seed      import the delivered CSVs
#   make analyze   run the detector
#   make down      stop everything
#
# `make help` lists the rest.

COMPOSE := docker compose

.DEFAULT_GOAL := help
.PHONY: help demo up down build rebuild logs seed analyze test test-race test-sql vet fmt \
        psql shell fresh db-only test-db smoke clean nuke

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ---------- the stack ----------

demo: ## Levanta la pila e importa los datos, sin lanzar el análisis
	@$(MAKE) up
	@$(MAKE) seed

up: ## Build and start the database and the API, and wait for both to be healthy
	$(COMPOSE) up -d --build --wait
	@echo
	@echo "  dashboard  http://$$($(COMPOSE) port api 8080)"
	@echo "  next       make seed && make analyze"

down: ## Stop the stack, keeping the data
	$(COMPOSE) down

build: ## Build the image without starting anything
	$(COMPOSE) build

rebuild: ## Rebuild from scratch, ignoring the layer cache
	$(COMPOSE) build --no-cache

logs: ## Follow the API logs
	$(COMPOSE) logs -f api

fresh: down ## Stop the stack and delete the database volume
	$(COMPOSE) down --volumes

db-only: ## Start only the database, for running the API on the host
	$(COMPOSE) up -d --wait db

# ---------- the data ----------

seed: ## Import the delivered readings and events (idempotent)
	$(COMPOSE) run --rm seed

analyze: ## Run the detector over the imported data (uses the demo account)
	@echo "running the detector..."
	@jar=$$(mktemp); \
	base=http://$$( $(COMPOSE) port api 8080 | sed 's/0\.0\.0\.0/127.0.0.1/' ); \
	curl -sS -c $$jar -H 'content-type: application/json' \
		-d "{\"email\":\"admin@email.com\",\"password\":\"$${DEMO_PASSWORD:-admin}\"}" \
		$$base/auth/login >/dev/null; \
	curl -sS -b $$jar -X POST $$base/ai/analyze | sed 's/^/  /'; \
	status=$${PIPESTATUS[0]}; rm -f $$jar; exit $$status

# ---------- checks ----------

test: ## Run the whole test suite in a container, against a throwaway database
	@$(MAKE) test-db
	@$(COMPOSE) --profile test run --rm tests; status=$$?; \
		$(COMPOSE) --profile test rm -sf db-test >/dev/null; exit $$status

test-sql: ## Run only the SQL integration tests
	$(COMPOSE) --profile test run --rm tests sh -c 'go test -race -count=1 ./internal/store/postgres/'

test-race: ## Run the suite with the race detector, without the SQL tests
	$(COMPOSE) run --rm --no-deps tests sh -c 'go test -race -count=1 ./...'

vet: ## Run go vet
	$(COMPOSE) run --rm --no-deps tests go vet ./...

fmt: ## Format the Go sources
	$(COMPOSE) run --rm --no-deps tests gofmt -l -w .

test-db: ## Start only the throwaway test database, and leave it up
	$(COMPOSE) --profile test up -d --wait db-test

smoke: ## Walk the running API and check every field the dashboard reads
	$(COMPOSE) --profile smoke run --rm smoke

# ---------- looking around ----------

psql: ## Open a psql session on the API's database
	$(COMPOSE) exec db psql -U energy -d energy

shell: ## Open a shell in the API container
	$(COMPOSE) exec api /bin/sh

clean: ## Remove the images this project built
	$(COMPOSE) down --volumes --remove-orphans
	docker image prune -f --filter label=com.docker.compose.project=energy-anomaly-mvp

nuke: clean ## Remove the images and the build cache of this project
	docker builder prune -f --filter label=com.docker.compose.project=energy-anomaly-mvp
