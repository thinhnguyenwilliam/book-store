.DEFAULT_GOAL := help

.PHONY: help tools generate proto swagger graphql tidy fmt test test-integration vet lint lint-tool build check perf e2e-checkout e2e-checkout-local migrate \
	docker-config docker-build up run up-foreground down down-volumes restart ps logs \
	dev dev-foreground dev-build dev-logs dev-ps dev-down dev-restart \
	air-tool elasticsearch-pull infra-core-up infra-up infra-stop infra-ps infra-logs app-stop local-prepare \
	local-gateway local-auth local-user local-book local-order local-payment local-worker local-notification local-comment local-chat local-analytics local-search search-reindex \
	watch-gateway watch-auth watch-user watch-book watch-order watch-payment watch-worker watch-notification watch-comment watch-chat watch-analytics watch-search hooks-install

TOOLS_DIR := $(CURDIR)/.tools
CACHE_DIR := $(CURDIR)/.cache
BUF := $(TOOLS_DIR)/buf
PROTOC_GEN_GO := $(TOOLS_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(TOOLS_DIR)/protoc-gen-go-grpc
SWAG := $(TOOLS_DIR)/swag
AIR := $(TOOLS_DIR)/air
AIR_VERSION := v1.66.1
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint
GOLANGCI_LINT_VERSION := v2.12.2
DEV_COMPOSE := docker compose -f docker-compose.yml -f docker-compose.dev.yml
LOCAL_CONFIG := config/local.yml
LOCAL_SECRET_CONFIG ?= config/local.secret.yml
LOCAL_SECRET_ARG = $(if $(wildcard $(LOCAL_SECRET_CONFIG)),-secrets $(LOCAL_SECRET_CONFIG),)
INFRA_SERVICES := postgres pgadmin redis redisinsight rabbitmq mailpit kafka-volume-init kafka kafka-init kafka-ui elasticsearch
INFRA_CORE_SERVICES := postgres pgadmin redis redisinsight rabbitmq mailpit kafka-volume-init kafka kafka-init kafka-ui
APP_SERVICES := gateway auth-service user-service book-service order-service payment-service worker-service notification-service comment-service chat-service analytics-service search-service
ELASTICSEARCH_PULL_RETRIES ?= 5
ELASTICSEARCH_PULL_DELAY_SECONDS ?= 10
REPO_ROOT := $(abspath $(CURDIR)/..)

help: ## Hiển thị các lệnh có thể sử dụng
	@awk 'BEGIN {FS = ":.*## "; printf "Book Store Backend\n\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

tools: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(SWAG) $(GOLANGCI_LINT) ## Cài tool vào backend/.tools

$(BUF):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install github.com/bufbuild/buf/cmd/buf@v1.57.2

$(PROTOC_GEN_GO):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.8

$(PROTOC_GEN_GO_GRPC):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

$(SWAG):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install github.com/swaggo/swag/cmd/swag@v1.16.6

lint-tool: $(GOLANGCI_LINT) ## Cài golangci-lint bản cố định vào backend/.tools

$(GOLANGCI_LINT):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

air-tool: $(AIR) ## Cài Air vào backend/.tools để chạy hot reload local

$(AIR):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install github.com/air-verse/air@$(AIR_VERSION)

generate: proto swagger graphql ## Sinh protobuf, Swagger và GraphQL code

proto: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) ## Sinh Go code từ protobuf
	PATH=$(TOOLS_DIR):$(PATH) XDG_CACHE_HOME=$(CACHE_DIR) $(BUF) generate

swagger: $(SWAG) ## Sinh Swagger docs từ HTTP annotations
	GOCACHE=$(CACHE_DIR)/go-build $(SWAG) init -g main.go -d cmd/gateway,internal/gateway/http -o docs --parseInternal --outputTypes go,json,yaml

graphql: ## Sinh GraphQL runtime và model type-safe từ schema
	GOCACHE=$(CACHE_DIR)/go-build go tool gqlgen generate

hooks-install: ## Cài Git hooks được quản lý trong repository
	git -C $(REPO_ROOT) config core.hooksPath .githooks
	chmod +x $(REPO_ROOT)/.githooks/pre-commit
	@echo "Git hooks installed from .githooks"

tidy: ## Đồng bộ go.mod và go.sum
	GOCACHE=$(CACHE_DIR)/go-build go mod tidy

fmt: ## Format source Go
	gofmt -w cmd internal gen docs

test: ## Chạy unit tests
	GOCACHE=$(CACHE_DIR)/go-build go test ./...

test-integration: ## Chạy integration test PostgreSQL, gồm kiểm tra chống oversell
	BOOKSTORE_TEST_DATABASE_URL='postgres://bookstore:bookstore@localhost:5432/bookstore?sslmode=disable' \
		GOCACHE=$(CACHE_DIR)/go-build go test -tags=integration -count=1 \
		./internal/book/adapter/postgres ./internal/order/adapter/postgres

vet: ## Chạy static analysis
	GOCACHE=$(CACHE_DIR)/go-build go vet ./...

lint: $(GOLANGCI_LINT) ## Chạy golangci-lint với cấu hình production
	@mkdir -p $(CACHE_DIR)/golangci-lint
	GOCACHE=$(CACHE_DIR)/go-build GOLANGCI_LINT_CACHE=$(CACHE_DIR)/golangci-lint $(GOLANGCI_LINT) run ./...

build: ## Build tất cả Go services
	GOCACHE=$(CACHE_DIR)/go-build go build -buildvcs=false ./cmd/...

check: lint test vet build docker-config ## Chạy toàn bộ kiểm tra trước khi commit

perf: ## Kiểm tra p95 của các read API local phải dưới 200ms
	./scripts/perf-check.sh

e2e-checkout: ## Kiểm thử local wallet -> cart -> order -> payment -> ledger
	./scripts/e2e-checkout.sh

e2e-checkout-local: ## Tự chạy/tắt service local và kiểm thử checkout đầy đủ
	./scripts/e2e-checkout-local.sh

docker-config: ## Kiểm tra cú pháp Docker Compose
	docker compose config --quiet

migrate: ## Áp dụng các migration idempotent vào PostgreSQL local đang chạy
	docker compose exec -T postgres sh -c 'set -e; for migration in /docker-entrypoint-initdb.d/*.sql; do psql -v ON_ERROR_STOP=1 -U bookstore -d bookstore -f "$$migration"; done'

docker-build: ## Build toàn bộ Docker images
	docker compose build

up: ## Build và chạy toàn bộ stack ở background
	docker compose up -d --build

run: up ## Alias của make up

up-foreground: ## Build và chạy stack, hiển thị log trực tiếp
	docker compose up --build

down: ## Dừng stack, giữ nguyên database volumes
	docker compose down

down-volumes: ## Dừng stack và xóa toàn bộ local volumes
	docker compose down --volumes

restart: down up ## Khởi động lại toàn bộ stack

ps: ## Xem trạng thái container
	docker compose ps

logs: ## Theo dõi log của toàn bộ stack
	docker compose logs -f --tail=200

dev: ## Chạy toàn bộ stack với Air hot reload ở background
	$(DEV_COMPOSE) up -d --build

dev-foreground: ## Chạy Air hot reload và hiển thị log trực tiếp
	$(DEV_COMPOSE) up --build

dev-build: ## Build lại image development có Air
	$(DEV_COMPOSE) build

dev-logs: ## Theo dõi log hot reload của các Go service
	$(DEV_COMPOSE) logs -f --tail=200 gateway auth-service user-service book-service order-service payment-service worker-service notification-service comment-service chat-service analytics-service search-service

dev-ps: ## Xem trạng thái container development
	$(DEV_COMPOSE) ps

dev-down: ## Dừng development stack nhưng giữ volumes
	$(DEV_COMPOSE) down

dev-restart: ## Khởi động lại development stack
	$(DEV_COMPOSE) down
	$(DEV_COMPOSE) up -d --build

elasticsearch-pull: ## Tải Elasticsearch với retry khi registry tạm thời không ổn định
	@attempt=1; \
	while [ $$attempt -le $(ELASTICSEARCH_PULL_RETRIES) ]; do \
		echo "Pulling Elasticsearch (attempt $$attempt/$(ELASTICSEARCH_PULL_RETRIES))..."; \
		if docker compose pull --policy missing elasticsearch; then \
			exit 0; \
		fi; \
		if [ $$attempt -eq $(ELASTICSEARCH_PULL_RETRIES) ]; then \
			echo "Elasticsearch registry is still unavailable after $(ELASTICSEARCH_PULL_RETRIES) attempts."; \
			exit 1; \
		fi; \
		attempt=$$((attempt + 1)); \
		echo "Retrying in $(ELASTICSEARCH_PULL_DELAY_SECONDS)s; downloaded layers will be reused..."; \
		sleep $(ELASTICSEARCH_PULL_DELAY_SECONDS); \
	done

infra-core-up: ## Chạy infrastructure không gồm Elasticsearch khi registry đang lỗi
	docker compose up -d --no-build $(INFRA_CORE_SERVICES)

infra-up: elasticsearch-pull ## Chạy PostgreSQL, Redis, RabbitMQ, Kafka, Elasticsearch và các UI local
	docker compose up -d --no-build $(INFRA_SERVICES)

infra-stop: ## Dừng các container infrastructure, giữ nguyên volumes
	docker compose stop $(INFRA_SERVICES)

infra-ps: ## Xem trạng thái các container infrastructure
	docker compose ps $(INFRA_SERVICES)

infra-logs: ## Theo dõi log các container infrastructure
	docker compose logs -f --tail=200 $(INFRA_SERVICES)

app-stop: ## Dừng các container Go để giải phóng các port chạy local
	docker compose stop $(APP_SERVICES)

local-prepare: ## Dừng container Go và chỉ khởi động infrastructure
	docker compose stop $(APP_SERVICES)
	docker compose up -d --no-build $(INFRA_SERVICES)

local-gateway: ## Chạy gateway trực tiếp trên máy, không hot reload
	go run ./cmd/gateway -config $(LOCAL_CONFIG)

local-auth: ## Chạy auth service trực tiếp trên máy, không hot reload
	go run ./cmd/auth-service -config $(LOCAL_CONFIG) $(LOCAL_SECRET_ARG)

local-user: ## Chạy user service trực tiếp trên máy, không hot reload
	go run ./cmd/user-service -config $(LOCAL_CONFIG)

local-book: ## Chạy book service trực tiếp trên máy, không hot reload
	go run ./cmd/book-service -config $(LOCAL_CONFIG)

local-order: ## Chạy order service trực tiếp trên máy, không hot reload
	go run ./cmd/order-service -config $(LOCAL_CONFIG)

local-payment: ## Chạy payment service trực tiếp trên máy, không hot reload
	go run ./cmd/payment-service -config $(LOCAL_CONFIG)

local-worker: ## Chạy worker service trực tiếp trên máy, không hot reload
	go run ./cmd/worker-service -config $(LOCAL_CONFIG)

local-notification: ## Chạy notification service trực tiếp trên máy
	go run ./cmd/notification-service -config $(LOCAL_CONFIG) $(LOCAL_SECRET_ARG)

local-comment: ## Chạy comment service trực tiếp trên máy
	go run ./cmd/comment-service -config $(LOCAL_CONFIG)

local-chat: ## Chạy chat service trực tiếp trên máy
	go run ./cmd/chat-service -config $(LOCAL_CONFIG)

local-analytics: ## Chạy analytics service/Kafka consumer trực tiếp trên máy
	go run ./cmd/analytics-service -config $(LOCAL_CONFIG)

local-search: ## Chạy Search Service/Elasticsearch consumer trực tiếp trên máy
	go run ./cmd/search-service -config $(LOCAL_CONFIG)

search-reindex: ## Đồng bộ lại toàn bộ catalog PostgreSQL sang Elasticsearch local
	go run ./cmd/search-service -config $(LOCAL_CONFIG) -reindex

define watch_service
	$(AIR) -c .air.local.toml \
		--build.cmd "go build -buildvcs=false -o ./.air-local/$(1) ./cmd/$(1)" \
		--build.entrypoint "./.air-local/$(1)" \
		--build.log "$(1)-build-errors.log" \
		-- -config $(LOCAL_CONFIG) $(2)
endef

watch-gateway: $(AIR) ## Chạy gateway local với Air hot reload
	$(call watch_service,gateway)

watch-auth: $(AIR) ## Chạy auth service local với Air hot reload
	$(call watch_service,auth-service,$(LOCAL_SECRET_ARG))

watch-user: $(AIR) ## Chạy user service local với Air hot reload
	$(call watch_service,user-service)

watch-book: $(AIR) ## Chạy book service local với Air hot reload
	$(call watch_service,book-service)

watch-order: $(AIR) ## Chạy order service local với Air hot reload
	$(call watch_service,order-service)

watch-payment: $(AIR) ## Chạy payment service local với Air hot reload
	$(call watch_service,payment-service)

watch-worker: $(AIR) ## Chạy worker service local với Air hot reload
	$(call watch_service,worker-service)

watch-notification: $(AIR) ## Chạy notification service local với Air hot reload
	$(call watch_service,notification-service,$(LOCAL_SECRET_ARG))

watch-comment: $(AIR) ## Chạy comment service local với Air hot reload
	$(call watch_service,comment-service)

watch-chat: $(AIR) ## Chạy chat service local với Air hot reload
	$(call watch_service,chat-service)

watch-analytics: $(AIR) ## Chạy analytics service local với Air hot reload
	$(call watch_service,analytics-service)

watch-search: $(AIR) ## Chạy search service local với Air hot reload
	$(call watch_service,search-service)
