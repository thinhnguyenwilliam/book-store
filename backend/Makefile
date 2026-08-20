.DEFAULT_GOAL := help

.PHONY: help tools generate proto swagger tidy fmt test vet build check \
	docker-config docker-build up run up-foreground down down-volumes restart ps logs \
	dev dev-foreground dev-build dev-logs dev-ps dev-down dev-restart \
	air-tool infra-up infra-stop infra-ps infra-logs app-stop local-prepare \
	local-gateway local-auth local-user local-book local-worker \
	watch-gateway watch-auth watch-user watch-book watch-worker

TOOLS_DIR := $(CURDIR)/.tools
CACHE_DIR := $(CURDIR)/.cache
BUF := $(TOOLS_DIR)/buf
PROTOC_GEN_GO := $(TOOLS_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(TOOLS_DIR)/protoc-gen-go-grpc
SWAG := $(TOOLS_DIR)/swag
AIR := $(TOOLS_DIR)/air
AIR_VERSION := v1.66.1
DEV_COMPOSE := docker compose -f docker-compose.yml -f docker-compose.dev.yml
LOCAL_CONFIG := config/local.yml
INFRA_SERVICES := postgres pgadmin redis redisinsight rabbitmq
APP_SERVICES := gateway auth-service user-service book-service worker-service

help: ## Hiển thị các lệnh có thể sử dụng
	@awk 'BEGIN {FS = ":.*## "; printf "Book Store Backend\n\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

tools: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(SWAG) ## Cài tool vào backend/.tools

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

air-tool: $(AIR) ## Cài Air vào backend/.tools để chạy hot reload local

$(AIR):
	@mkdir -p $(TOOLS_DIR) $(CACHE_DIR)/go-build
	GOBIN=$(TOOLS_DIR) GOCACHE=$(CACHE_DIR)/go-build go install github.com/air-verse/air@$(AIR_VERSION)

generate: proto swagger ## Sinh protobuf và Swagger docs

proto: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) ## Sinh Go code từ protobuf
	PATH=$(TOOLS_DIR):$(PATH) XDG_CACHE_HOME=$(CACHE_DIR) $(BUF) generate

swagger: $(SWAG) ## Sinh Swagger docs từ HTTP annotations
	GOCACHE=$(CACHE_DIR)/go-build $(SWAG) init -g main.go -d cmd/gateway,internal/gateway/http -o docs --parseInternal --outputTypes go,json,yaml

tidy: ## Đồng bộ go.mod và go.sum
	GOCACHE=$(CACHE_DIR)/go-build go mod tidy

fmt: ## Format source Go
	gofmt -w cmd internal gen docs

test: ## Chạy unit tests
	GOCACHE=$(CACHE_DIR)/go-build go test ./...

vet: ## Chạy static analysis
	GOCACHE=$(CACHE_DIR)/go-build go vet ./...

build: ## Build tất cả Go services
	GOCACHE=$(CACHE_DIR)/go-build go build -buildvcs=false ./cmd/...

check: test vet build docker-config ## Chạy toàn bộ kiểm tra trước khi commit

docker-config: ## Kiểm tra cú pháp Docker Compose
	docker compose config --quiet

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
	$(DEV_COMPOSE) logs -f --tail=200 gateway auth-service user-service book-service worker-service

dev-ps: ## Xem trạng thái container development
	$(DEV_COMPOSE) ps

dev-down: ## Dừng development stack nhưng giữ volumes
	$(DEV_COMPOSE) down

dev-restart: ## Khởi động lại development stack
	$(DEV_COMPOSE) down
	$(DEV_COMPOSE) up -d --build

infra-up: ## Chỉ chạy PostgreSQL, pgAdmin, Redis, RedisInsight và RabbitMQ
	docker compose up -d --no-build $(INFRA_SERVICES)

infra-stop: ## Dừng các container infrastructure, giữ nguyên volumes
	docker compose stop $(INFRA_SERVICES)

infra-ps: ## Xem trạng thái các container infrastructure
	docker compose ps $(INFRA_SERVICES)

infra-logs: ## Theo dõi log các container infrastructure
	docker compose logs -f --tail=200 $(INFRA_SERVICES)

app-stop: ## Dừng 5 container Go để giải phóng các port chạy local
	docker compose stop $(APP_SERVICES)

local-prepare: ## Dừng container Go và chỉ khởi động infrastructure
	docker compose stop $(APP_SERVICES)
	docker compose up -d --no-build $(INFRA_SERVICES)

local-gateway: ## Chạy gateway trực tiếp trên máy, không hot reload
	go run ./cmd/gateway -config $(LOCAL_CONFIG)

local-auth: ## Chạy auth service trực tiếp trên máy, không hot reload
	go run ./cmd/auth-service -config $(LOCAL_CONFIG)

local-user: ## Chạy user service trực tiếp trên máy, không hot reload
	go run ./cmd/user-service -config $(LOCAL_CONFIG)

local-book: ## Chạy book service trực tiếp trên máy, không hot reload
	go run ./cmd/book-service -config $(LOCAL_CONFIG)

local-worker: ## Chạy worker service trực tiếp trên máy, không hot reload
	go run ./cmd/worker-service -config $(LOCAL_CONFIG)

define watch_service
	$(AIR) -c .air.local.toml \
		--build.cmd "go build -buildvcs=false -o ./.air-local/$(1) ./cmd/$(1)" \
		--build.entrypoint "./.air-local/$(1)" \
		--build.log "$(1)-build-errors.log" \
		-- -config $(LOCAL_CONFIG)
endef

watch-gateway: $(AIR) ## Chạy gateway local với Air hot reload
	$(call watch_service,gateway)

watch-auth: $(AIR) ## Chạy auth service local với Air hot reload
	$(call watch_service,auth-service)

watch-user: $(AIR) ## Chạy user service local với Air hot reload
	$(call watch_service,user-service)

watch-book: $(AIR) ## Chạy book service local với Air hot reload
	$(call watch_service,book-service)

watch-worker: $(AIR) ## Chạy worker service local với Air hot reload
	$(call watch_service,worker-service)
