# MTG Data Pipeline Makefile
# Automates building, running, and managing the development environment

.PHONY: help
help: ## Show this help message
	@echo "MTG Data Pipeline - Development Commands"
	@echo "========================================"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Environment variables
DOCKER_COMPOSE = docker-compose
GO = go
MAVEN = docker run --rm -v $(PWD)/data-ingestion/mtg-ingestor/flink:/app -w /app maven:3.8-openjdk-11 mvn

# Directories
INGESTOR_DIR = data-ingestion/mtg-ingestor
FLINK_DIR = $(INGESTOR_DIR)/flink
DECKS_DIR = decks

# === MAIN COMMANDS ===

.PHONY: setup
setup: up create-topics build deploy-flink ## Complete setup: start services, create topics, build, and deploy
	@echo "✅ Setup complete! Services are running."
	@echo "📊 Access points:"
	@echo "  - Kafka UI: http://localhost:8080"
	@echo "  - Flink UI: http://localhost:8081"
	@echo "  - Query UI: http://localhost:8090/query-ui.html"

.PHONY: up
up: ## Start all Docker services
	$(DOCKER_COMPOSE) up -d
	@echo "⏳ Waiting for services to initialize..."
	@sleep 10

.PHONY: down
down: ## Stop all Docker services
	$(DOCKER_COMPOSE) down

.PHONY: restart
restart: down up ## Restart all services

.PHONY: status
status: ## Check service status
	@echo "🔍 Checking service status..."
	@docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

.PHONY: logs
logs: ## Show logs (use service=<name> to filter)
	$(DOCKER_COMPOSE) logs -f $(service)

# === BUILD COMMANDS ===

.PHONY: build
build: build-go build-flink build-docker ## Build all components

.PHONY: build-go
build-go: ## Build Go services
	@echo "🔨 Building Go services..."
	cd $(INGESTOR_DIR) && $(GO) build -o mtg-ingestor cmd/main.go
	cd $(INGESTOR_DIR) && $(GO) build -o deck-ingester cmd/deck-ingester/main.go
	@echo "✅ Go services built"

.PHONY: build-flink
build-flink: ## Build Flink JAR
	@echo "🔨 Building Flink processor..."
	cd $(FLINK_DIR) && $(MAVEN) clean package
	@echo "✅ Flink JAR built"

.PHONY: build-docker
build-docker: ## Build Docker images
	@echo "🐳 Building Docker images..."
	$(DOCKER_COMPOSE) build
	@echo "✅ Docker images built"

# === KAFKA COMMANDS ===

.PHONY: create-topics
create-topics: ## Create Kafka topics
	@echo "📝 Creating Kafka topics..."
	@$(INGESTOR_DIR)/scripts/create-deck-topics.sh || true
	@echo "✅ Kafka topics created"

.PHONY: kafka-topics
kafka-topics: ## List Kafka topics
	docker exec kafka kafka-topics --list --bootstrap-server localhost:9092

.PHONY: kafka-consume
kafka-consume: ## Consume from Kafka topic (use topic=<name>)
	docker exec kafka kafka-console-consumer \
		--bootstrap-server localhost:9092 \
		--topic $(topic) \
		--from-beginning \
		--max-messages 10

# === INGESTION COMMANDS ===

.PHONY: ingest-all
ingest-all: ingest-mtg ingest-decks ## Run all ingestion jobs

.PHONY: ingest-mtg
ingest-mtg: ## Ingest MTGJSON data
	@echo "📥 Ingesting MTG data from MTGJSON..."
	$(DOCKER_COMPOSE) --profile ingest up mtg-ingestor

.PHONY: ingest-decks
ingest-decks: ## Ingest deck files
	@echo "📥 Ingesting deck files..."
	$(DOCKER_COMPOSE) run --rm --entrypoint /app/deck-ingester deck-ingestor -dir /decks

.PHONY: test-decks
test-decks: ## Test deck ingestion (dry-run)
	@echo "🧪 Testing deck ingestion..."
	$(DOCKER_COMPOSE) run --rm --entrypoint /app/deck-ingester deck-ingestor -dir /decks -dry-run

# === FLINK COMMANDS ===

.PHONY: deploy-flink
deploy-flink: ## Deploy Flink jobs
	@echo "🚀 Deploying Flink jobs..."
	@if [ ! -f $(FLINK_DIR)/target/mtg-flink-processor-1.0.0.jar ]; then \
		echo "❌ JAR not found. Running build first..."; \
		$(MAKE) build-flink-docker; \
	fi
	docker cp $(FLINK_DIR)/target/mtg-flink-processor-1.0.0.jar flink-jobmanager:/tmp/
	docker exec flink-jobmanager flink run -c com.mtg.flink.DeckValueProcessor /tmp/mtg-flink-processor-1.0.0.jar || true
	@echo "✅ Flink jobs deployed"

.PHONY: flink-jobs
flink-jobs: ## List running Flink jobs
	docker exec flink-jobmanager flink list

.PHONY: flink-cancel
flink-cancel: ## Cancel Flink job (use job=<id>)
	docker exec flink-jobmanager flink cancel $(job)

# === QUERY COMMANDS ===

.PHONY: query-decks
query-decks: ## Show deck values
	@echo "💰 Deck Values:"
	@timeout 3 docker exec kafka kafka-console-consumer \
		--bootstrap-server localhost:9092 \
		--topic mtg.deck-values \
		--from-beginning 2>/dev/null | \
		jq -r '.data | "\(.deck_name): $$\(.total_value) (\(.total_cards) cards)"' | sort -u || true

.PHONY: ksql-cli
ksql-cli: ## Open KSQL CLI
	docker exec -it ksqldb-cli ksql http://ksqldb-server:8088

.PHONY: web-ui
web-ui: ## Open web UI in browser
	@echo "🌐 Opening web UI..."
	@command -v xdg-open >/dev/null 2>&1 && xdg-open http://localhost:8090/query-ui.html || \
		command -v open >/dev/null 2>&1 && open http://localhost:8090/query-ui.html || \
		echo "Please open http://localhost:8090/query-ui.html in your browser"

# === DATABASE COMMANDS ===

.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	docker exec -it postgres psql -U mtg_user -d mtg

.PHONY: db-backup
db-backup: ## Backup database
	@mkdir -p backups
	docker exec postgres pg_dump -U mtg_user mtg > backups/mtg_backup_$$(date +%Y%m%d_%H%M%S).sql
	@echo "✅ Database backed up to backups/"

# === CLEANUP COMMANDS ===

.PHONY: clean
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(INGESTOR_DIR)/mtg-ingestor $(INGESTOR_DIR)/deck-ingester
	rm -rf $(FLINK_DIR)/target
	@echo "✅ Clean complete"

.PHONY: clean-data
clean-data: ## Clean data volumes (WARNING: deletes all data)
	@echo "⚠️  WARNING: This will delete all data!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		$(DOCKER_COMPOSE) down -v; \
		echo "✅ Data volumes cleaned"; \
	fi

.PHONY: clean-all
clean-all: clean clean-data ## Clean everything (artifacts and data)

.PHONY: reset
reset: clean-all setup ## Reset everything and start fresh

# === DEVELOPMENT COMMANDS ===

.PHONY: test
test: ## Run tests
	@echo "🧪 Running tests..."
	cd $(INGESTOR_DIR) && $(GO) test ./...

.PHONY: fmt
fmt: ## Format Go code
	@echo "📝 Formatting Go code..."
	cd $(INGESTOR_DIR) && $(GO) fmt ./...

.PHONY: lint
lint: ## Lint Go code
	@echo "🔍 Linting Go code..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	cd $(INGESTOR_DIR) && golangci-lint run

.PHONY: mod-tidy
mod-tidy: ## Tidy Go modules
	@echo "📦 Tidying Go modules..."
	cd $(INGESTOR_DIR) && $(GO) mod tidy

# === GIT COMMANDS ===

.PHONY: git-clean
git-clean: ## Remove large files from git
	@echo "🗑️  Removing large build files from git..."
	git rm -r --cached $(FLINK_DIR)/target/ 2>/dev/null || true
	git rm --cached $(INGESTOR_DIR)/mtg-ingestor 2>/dev/null || true
	git rm --cached $(INGESTOR_DIR)/deck-ingester 2>/dev/null || true
	@echo "✅ Large files removed from git tracking"
	@echo "📝 Don't forget to commit these changes!"

# === SPECIAL BUILD COMMAND FOR FLINK WITHOUT MAVEN ===

.PHONY: build-flink-docker
build-flink-docker: ## Build Flink JAR using Docker (no Maven required)
	@echo "🐳 Building Flink JAR with Docker..."
	cd $(FLINK_DIR) && \
	docker build -f Dockerfile.build -t flink-job-builder . && \
	docker create --name temp-flink flink-job-builder && \
	docker cp temp-flink:/output/. ./target/ && \
	docker rm temp-flink
	@echo "✅ Flink JAR built with Docker"

# === INFO COMMANDS ===

.PHONY: info
info: ## Show project information
	@echo "📊 MTG Data Pipeline Information"
	@echo "================================"
	@echo "Decks tracked: $$(ls $(DECKS_DIR)/*.deck 2>/dev/null | wc -l)"
	@echo "Services running: $$(docker ps --format '{{.Names}}' | wc -l)"
	@echo ""
	@echo "📈 Quick Stats:"
	@$(MAKE) -s query-decks | head -5
	@echo ""
	@echo "🔗 Access URLs:"
	@echo "  Kafka UI: http://localhost:8080"
	@echo "  Flink UI: http://localhost:8081"
	@echo "  Query UI: http://localhost:8090/query-ui.html"

.PHONY: version
version: ## Show component versions
	@echo "Component Versions:"
	@echo "  Go: $$(go version 2>/dev/null || echo 'not installed')"
	@echo "  Docker: $$(docker --version)"
	@echo "  Docker Compose: $$(docker-compose --version)"

# Default target
.DEFAULT_GOAL := help