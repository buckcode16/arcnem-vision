DOCKER ?= docker
DOCKER_COMPOSE ?= docker compose
PROJECT_NAME := arcnem-vision
SMOKE_WAIT_SECONDS ?= 5
HOST_GATEWAY_ARG := --add-host host.docker.internal:host-gateway
LOCAL_DOCKER_ARGS := $(HOST_GATEWAY_ARG)

SERVICES := api dashboard agents mcp
INFRA_SERVICES := postgres redis minio minio-init
INNGEST_LOG_FILE := .make/inngest.log
INNGEST_TMUX_SESSION := $(PROJECT_NAME)-inngest
INNGEST_DEV_URL := http://localhost:3020/api/inngest
INNGEST_HEALTH_URL := http://localhost:8288/health

API_IMAGE := $(PROJECT_NAME)-api
API_CONTAINER := $(PROJECT_NAME)-api
API_TEST_CONTAINER := $(PROJECT_NAME)-api-smoke
API_DOCKERFILE := server/packages/api/Dockerfile
API_CONTEXT := server
API_ENV_FILE := server/packages/api/.env.docker
API_ENV_EXAMPLE_FILE := server/packages/api/.env.docker.example
API_TEST_ENV_FILE := $(API_ENV_FILE)
API_PORT := 3000
API_CONTAINER_PORT := 3000
API_TEST_PORT := 13000
API_TEST_URL := http://localhost:13000/health

DASHBOARD_IMAGE := $(PROJECT_NAME)-dashboard
DASHBOARD_CONTAINER := $(PROJECT_NAME)-dashboard
DASHBOARD_TEST_CONTAINER := $(PROJECT_NAME)-dashboard-smoke
DASHBOARD_DOCKERFILE := server/packages/dashboard/Dockerfile
DASHBOARD_CONTEXT := server
DASHBOARD_ENV_FILE := server/packages/dashboard/.env.docker
DASHBOARD_ENV_EXAMPLE_FILE := server/packages/dashboard/.env.docker.example
DASHBOARD_TEST_ENV_FILE := $(DASHBOARD_ENV_FILE)
DASHBOARD_BUILD_ENV_FILE := $(DASHBOARD_ENV_FILE)
DASHBOARD_BUILD_ARG_VARS := VITE_API_URL
DASHBOARD_PORT := 3001
DASHBOARD_CONTAINER_PORT := 3001
DASHBOARD_TEST_PORT := 13001
DASHBOARD_TEST_URL := http://localhost:13001/

AGENTS_IMAGE := $(PROJECT_NAME)-agents
AGENTS_CONTAINER := $(PROJECT_NAME)-agents
AGENTS_TEST_CONTAINER := $(PROJECT_NAME)-agents-smoke
AGENTS_DOCKERFILE := models/agents/Dockerfile
AGENTS_CONTEXT := models
AGENTS_ENV_FILE := models/agents/.env.docker
AGENTS_ENV_EXAMPLE_FILE := models/agents/.env.docker.example
AGENTS_TEST_ENV_FILE := $(AGENTS_ENV_FILE)
AGENTS_PORT := 3020
AGENTS_CONTAINER_PORT := 3020
AGENTS_TEST_PORT := 13020
AGENTS_TEST_URL := http://localhost:13020/health

MCP_IMAGE := $(PROJECT_NAME)-mcp
MCP_CONTAINER := $(PROJECT_NAME)-mcp
MCP_TEST_CONTAINER := $(PROJECT_NAME)-mcp-smoke
MCP_DOCKERFILE := models/mcp/Dockerfile
MCP_CONTEXT := models
MCP_ENV_FILE := models/mcp/.env.docker
MCP_ENV_EXAMPLE_FILE := models/mcp/.env.docker.example
MCP_TEST_ENV_FILE := $(MCP_ENV_FILE)
MCP_PORT := 3021
MCP_CONTAINER_PORT := 3021
MCP_TEST_PORT := 13021
MCP_TEST_URL := http://localhost:13021/health

DB_IMAGE := $(PROJECT_NAME)-db
DB_CONTAINER := $(PROJECT_NAME)-db
DB_DOCKERFILE := server/packages/db/Dockerfile
DB_CONTEXT := server
DB_ENV_FILE := server/packages/db/.env.docker
DB_ENV_EXAMPLE_FILE := server/packages/db/.env.docker.example
DB_MIGRATIONS_DIR := $(CURDIR)/server/packages/db/src/migrations

.PHONY: help infra-up infra-down run-inngest stop-inngest logs-inngest verify-stack build-all test-all run-all stop-all clean-all build-db migrate-db generate-db clean-db $(SERVICES:%=build-%) $(SERVICES:%=test-%) $(SERVICES:%=run-%) $(SERVICES:%=stop-%) $(SERVICES:%=logs-%) $(SERVICES:%=clean-%)

help:
	@echo "Docker targets"
	@echo ""
	@echo "Local docker runs and smoke tests read each service's .env.docker file."
	@echo "Seed them from the committed .env.docker.example files."
	@echo ""
	@echo "Per service:"
	@echo "  make build-{api|dashboard|agents|mcp}"
	@echo "  make test-{api|dashboard|agents|mcp}"
	@echo "  make run-{api|dashboard|agents|mcp}"
	@echo "  make stop-{api|dashboard|agents|mcp}"
	@echo "  make logs-{api|dashboard|agents|mcp}"
	@echo "  make clean-{api|dashboard|agents|mcp}"
	@echo ""
	@echo "Database:"
	@echo "  make build-db"
	@echo "  make migrate-db"
	@echo "  make generate-db"
	@echo "  make clean-db"
	@echo ""
	@echo "Infrastructure:"
	@echo "  make infra-up"
	@echo "  make infra-down"
	@echo "  make run-inngest"
	@echo "  make stop-inngest"
	@echo "  make logs-inngest"
	@echo "  make verify-stack"
	@echo ""
	@echo "Bulk:"
	@echo "  make build-all"
	@echo "  make test-all"
	@echo "  make run-all"
	@echo "  make stop-all"
	@echo "  make clean-all"

define DOCKER_SERVICE_TARGETS
build-$(1):
	@set -a; \
	if [ -n "$($(2)_BUILD_ENV_FILE)" ]; then \
		if [ ! -f "$($(2)_BUILD_ENV_FILE)" ]; then \
			echo "Missing env file: $($(2)_BUILD_ENV_FILE) (copy $($(2)_ENV_EXAMPLE_FILE))"; \
			exit 1; \
		fi; \
		. "$($(2)_BUILD_ENV_FILE)"; \
	fi; \
	$(DOCKER) build \
		$(foreach var,$($(2)_BUILD_ARG_VARS),--build-arg $(var)) \
		-t $($(2)_IMAGE):latest \
		-f $($(2)_DOCKERFILE) \
		$($(2)_CONTEXT)

test-$(1): build-$(1)
	@if [ ! -f "$($(2)_TEST_ENV_FILE)" ]; then \
		echo "Missing env file: $($(2)_TEST_ENV_FILE) (copy $($(2)_ENV_EXAMPLE_FILE))"; \
		exit 1; \
	fi
	@set -e; \
	trap '$(DOCKER) rm -f $($(2)_TEST_CONTAINER) >/dev/null 2>&1 || true' EXIT; \
	$(DOCKER) rm -f $($(2)_TEST_CONTAINER) >/dev/null 2>&1 || true; \
	$(DOCKER) run -d \
		--name $($(2)_TEST_CONTAINER) \
		-p $($(2)_TEST_PORT):$($(2)_CONTAINER_PORT) \
		--env-file $($(2)_TEST_ENV_FILE) \
		$(LOCAL_DOCKER_ARGS) \
		$($(2)_TEST_DOCKER_ARGS) \
		$($(2)_IMAGE):latest >/dev/null; \
	sleep $(SMOKE_WAIT_SECONDS); \
	if [ "$$$$($(DOCKER) inspect -f '{{.State.Running}}' $($(2)_TEST_CONTAINER) 2>/dev/null)" != "true" ]; then \
		$(DOCKER) logs $($(2)_TEST_CONTAINER); \
		exit 1; \
	fi; \
	if [ -n "$($(2)_TEST_URL)" ]; then \
		curl --fail --silent --show-error "$($(2)_TEST_URL)" >/dev/null; \
	fi; \
	echo "Smoke test passed: $(1)"

run-$(1): build-$(1)
	@if [ ! -f "$($(2)_ENV_FILE)" ]; then \
		echo "Missing env file: $($(2)_ENV_FILE) (copy $($(2)_ENV_EXAMPLE_FILE))"; \
		exit 1; \
	fi
	@$(DOCKER) rm -f $($(2)_CONTAINER) >/dev/null 2>&1 || true
	$(DOCKER) run -d \
		--name $($(2)_CONTAINER) \
		-p $($(2)_PORT):$($(2)_CONTAINER_PORT) \
		--env-file $($(2)_ENV_FILE) \
		$(LOCAL_DOCKER_ARGS) \
		$($(2)_RUN_DOCKER_ARGS) \
		$($(2)_IMAGE):latest

stop-$(1):
	@$(DOCKER) rm -f $($(2)_CONTAINER) >/dev/null 2>&1 || true

logs-$(1):
	$(DOCKER) logs -f $($(2)_CONTAINER)

clean-$(1): stop-$(1)
	@$(DOCKER) rm -f $($(2)_TEST_CONTAINER) >/dev/null 2>&1 || true
	@$(DOCKER) rmi $($(2)_IMAGE):latest >/dev/null 2>&1 || true
endef

$(eval $(call DOCKER_SERVICE_TARGETS,api,API))
$(eval $(call DOCKER_SERVICE_TARGETS,dashboard,DASHBOARD))
$(eval $(call DOCKER_SERVICE_TARGETS,agents,AGENTS))
$(eval $(call DOCKER_SERVICE_TARGETS,mcp,MCP))

build-db:
	$(DOCKER) build -t $(DB_IMAGE):latest -f $(DB_DOCKERFILE) $(DB_CONTEXT)

migrate-db: build-db
	@if [ ! -f "$(DB_ENV_FILE)" ]; then \
		echo "Missing env file: $(DB_ENV_FILE) (copy $(DB_ENV_EXAMPLE_FILE))"; \
		exit 1; \
	fi
	$(DOCKER) run --rm \
		--name $(DB_CONTAINER) \
		--env-file $(DB_ENV_FILE) \
		$(LOCAL_DOCKER_ARGS) \
		$(DB_IMAGE):latest

generate-db: build-db
	@if [ ! -f "$(DB_ENV_FILE)" ]; then \
		echo "Missing env file: $(DB_ENV_FILE) (copy $(DB_ENV_EXAMPLE_FILE))"; \
		exit 1; \
	fi
	$(DOCKER) run --rm \
		--name $(DB_CONTAINER) \
		-v "$(DB_MIGRATIONS_DIR):/app/packages/db/src/migrations" \
		--env-file $(DB_ENV_FILE) \
		$(LOCAL_DOCKER_ARGS) \
		$(DB_IMAGE):latest \
		./packages/db/generate.sh

clean-db:
	@$(DOCKER) rm -f $(DB_CONTAINER) >/dev/null 2>&1 || true
	@$(DOCKER) rmi $(DB_IMAGE):latest >/dev/null 2>&1 || true

infra-up:
	$(DOCKER_COMPOSE) up -d $(INFRA_SERVICES)

infra-down:
	@$(DOCKER_COMPOSE) rm -sf $(INFRA_SERVICES) >/dev/null 2>&1 || true

run-inngest:
	@mkdir -p .make
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "tmux is required to run Inngest in the background"; \
		exit 1; \
	fi
	@if tmux has-session -t "$(INNGEST_TMUX_SESSION)" 2>/dev/null; then \
		echo "Inngest already running in tmux session $(INNGEST_TMUX_SESSION)"; \
		exit 0; \
	fi
	@rm -f "$(INNGEST_LOG_FILE)"
	@tmux new-session -d -s "$(INNGEST_TMUX_SESSION)" "cd '$(CURDIR)' && exec npx inngest-cli@latest dev -u '$(INNGEST_DEV_URL)' >>'$(INNGEST_LOG_FILE)' 2>&1"
	@for attempt in 1 2 3 4 5 6 7 8 9 10; do \
		if curl --fail --silent --show-error "$(INNGEST_HEALTH_URL)" >/dev/null 2>&1; then \
			echo "Inngest running in tmux session $(INNGEST_TMUX_SESSION)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Inngest failed to start"; \
	cat "$(INNGEST_LOG_FILE)"; \
	tmux kill-session -t "$(INNGEST_TMUX_SESSION)" >/dev/null 2>&1 || true; \
	exit 1

stop-inngest:
	@tmux kill-session -t "$(INNGEST_TMUX_SESSION)" >/dev/null 2>&1 || true

logs-inngest:
	@if [ -f "$(INNGEST_LOG_FILE)" ]; then \
		tail -f "$(INNGEST_LOG_FILE)"; \
	else \
		echo "No Inngest log found at $(INNGEST_LOG_FILE)"; \
	fi

verify-stack:
	@curl --fail --silent --show-error "http://localhost:3000/health" >/dev/null
	@curl --fail --silent --show-error "http://localhost:3020/health" >/dev/null
	@curl --fail --silent --show-error "http://localhost:3021/health" >/dev/null
	@curl --fail --silent --show-error "$(INNGEST_HEALTH_URL)" >/dev/null
	@curl --fail --silent --show-error "http://localhost:3001/?showArchived=false" >/dev/null
	@echo "Stack verification passed"

build-all: $(SERVICES:%=build-%) build-db

test-all:
	@$(MAKE) infra-up
	@$(MAKE) migrate-db
	@$(MAKE) test-api
	@$(MAKE) test-dashboard
	@$(MAKE) test-agents
	@$(MAKE) test-mcp

run-all:
	@$(MAKE) infra-up
	@$(MAKE) migrate-db
	@$(MAKE) run-mcp
	@$(MAKE) run-agents
	@$(MAKE) run-api
	@$(MAKE) run-dashboard
	@$(MAKE) run-inngest
	@$(MAKE) verify-stack

stop-all:
	@$(MAKE) stop-dashboard
	@$(MAKE) stop-api
	@$(MAKE) stop-agents
	@$(MAKE) stop-mcp
	@$(MAKE) stop-inngest
	@$(MAKE) infra-down

clean-all:
	@$(MAKE) stop-all
	@$(MAKE) clean-api
	@$(MAKE) clean-dashboard
	@$(MAKE) clean-agents
	@$(MAKE) clean-mcp
	@$(MAKE) clean-db
