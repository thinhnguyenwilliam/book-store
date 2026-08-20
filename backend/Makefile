.DEFAULT_GOAL := help

.PHONY: help tools generate proto swagger tidy fmt test vet build check \
	docker-config docker-build up run up-foreground down down-volumes restart ps logs

TOOLS_DIR := $(CURDIR)/.tools
CACHE_DIR := $(CURDIR)/.cache
BUF := $(TOOLS_DIR)/buf
PROTOC_GEN_GO := $(TOOLS_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(TOOLS_DIR)/protoc-gen-go-grpc
SWAG := $(TOOLS_DIR)/swag

help: ## Hiển thị các lệnh có thể sử dụng
	@awk 'BEGIN {FS = ":.*## "; printf "Book Store Backend\n\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

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
