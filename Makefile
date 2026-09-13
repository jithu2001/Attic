# Attic — one Go server binary + one Flutter app.
#
# Run `make help` for the full target list.

SERVER_DIR := server
APP_DIR    := app
COMPOSE    := docker compose -f $(SERVER_DIR)/docker-compose.yml --env-file $(SERVER_DIR)/.env

.DEFAULT_GOAL := help
.PHONY: help up down logs ps server-build server-run server-test server-fmt server-lint \
        app-get app-test app-analyze app-run test fmt

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## ---- stack ---------------------------------------------------------------

up: $(SERVER_DIR)/.env ## Build and start app + postgres + caddy
	$(COMPOSE) up -d --build
	@echo "Attic is up. Health: http://127.0.0.1:8080/healthz"

down: ## Stop the stack (volumes are kept)
	$(COMPOSE) down

logs: ## Follow the app logs
	$(COMPOSE) logs -f app

ps: ## Show stack status
	$(COMPOSE) ps

$(SERVER_DIR)/.env:
	@echo "Missing $(SERVER_DIR)/.env — copy $(SERVER_DIR)/.env.example and set ATTIC_JWT_SECRET." >&2
	@exit 1

## ---- server --------------------------------------------------------------

server-build: ## Compile the server binary
	cd $(SERVER_DIR) && go build ./...

server-run: ## Run the server locally (needs a reachable Postgres)
	cd $(SERVER_DIR) && go run ./cmd/Attic

server-test: ## Run Go tests with the race detector
	cd $(SERVER_DIR) && go test -race ./...

server-fmt: ## Format Go code
	cd $(SERVER_DIR) && gofmt -w .

server-lint: ## Vet Go code and check formatting
	cd $(SERVER_DIR) && go vet ./...
	@cd $(SERVER_DIR) && test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }

## ---- app -----------------------------------------------------------------

app-get: ## Resolve Flutter dependencies
	cd $(APP_DIR) && flutter pub get

app-test: ## Run Flutter tests
	cd $(APP_DIR) && flutter test

app-analyze: ## Static analysis of the Flutter app
	cd $(APP_DIR) && flutter analyze

app-run: ## Run the app on the connected device
	cd $(APP_DIR) && flutter run

## ---- everything ----------------------------------------------------------

test: server-test app-test ## Run every test suite

fmt: server-fmt ## Format everything
	cd $(APP_DIR) && dart format lib test
