.DEFAULT_GOAL := help

BIN := bin/tabletop

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n", $$1, $$2}'

.PHONY: web
web: ## Build the browser UI into web/dist (embedded in the binary)
	cd web && npm ci --no-audit --no-fund && npm run build

.PHONY: build
build: ## Build the binary into bin/ (run `make web` first for the UI)
	go build -o $(BIN) ./cmd/tabletop

.PHONY: run
run: build ## Build and run it for yourself (-local: auto sign-in, keys from env)
	./$(BIN) -local

.PHONY: debug
debug: build ## Run with debug logging to the terminal and data/tabletop.log
	./$(BIN) -local -debug -log-file data/tabletop.log

.PHONY: serve
serve: build ## Run as a multi-user server (needs TABLETOP_ADMIN_USER/_PASSWORD on first start)
	./$(BIN)

.PHONY: demo
demo: build ## Run offline with a scripted DM, on its own save file data/demo.db
	./$(BIN) -demo

.PHONY: backup
backup: ## Back up data/tabletop.db into data/backups now (safe while the server runs)
	go run ./cmd/tabletop -backup-now -backup-dir data/backups

.PHONY: docker
docker: ## Build the Docker image hearthbound:latest
	docker build -t hearthbound:latest .

.PHONY: up
up: ## Start Hearthbound behind Caddy (HTTPS) with docker compose; needs .env (see deploy/.env.example)
	docker compose up -d --build

.PHONY: down
down: ## Stop the docker compose services (data volumes are kept)
	docker compose down

.PHONY: dev-web
.PHONY: stock-list
PACK ?= candlelit
stock-list: ## List the stock images (scenes, monsters, portraits) and which are painted
	go run ./cmd/tabletop -stock-list -art-pack $(PACK)

.PHONY: stock-paint
KINDS ?= all
LIMIT ?= 0
stock-paint: ## Paint missing stock images with OPENAI_API_KEY (PACK=candlelit KINDS=scenes,monsters,portraits LIMIT=n), then make web
	go run ./cmd/tabletop -paint-stock $(KINDS) -art-pack $(PACK) -stock-limit $(LIMIT) -image-quality medium

dev-web: ## Run the Vite dev server (proxies /api to :8080)
	cd web && npm run dev

.PHONY: fmt
fmt: ## Format code (gofumpt via golangci-lint)
	golangci-lint fmt ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: test
test: ## Run tests with the race detector
	go test -race ./...

.PHONY: cover
cover: ## Fail unless internal/usecase/... is 100% covered
	go test -coverprofile=usecase.cover ./internal/usecase/...
	@go tool cover -func=usecase.cover | awk '/^total:/ { if ($$3 != "100.0%") { print "usecase coverage " $$3 " < 100%"; exit 1 } }'
	@rm -f usecase.cover

.PHONY: check
check: lint test cover ## Definition of done: lint + test + cover

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin usecase.cover data
