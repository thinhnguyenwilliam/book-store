.DEFAULT_GOAL := help

.PHONY: help local stop down logs status

help: ## Hiển thị các lệnh local
	@awk 'BEGIN {FS = ":.*## "; printf "Book Store\n\nUsage:\n  make local\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

local: ## Chạy toàn bộ stack local bằng một lệnh (infra Docker + Go trên máy + Vue)
	@./scripts/local.sh up

stop: ## Dừng Go services và frontend; giữ PostgreSQL/Redis/Kafka/...
	@./scripts/local.sh down

down: ## Dừng apps và Docker infrastructure, giữ volumes
	@./scripts/local.sh down-all

logs: ## Theo dõi log các process local
	@./scripts/local.sh logs

status: ## Xem process local đang chạy
	@./scripts/local.sh status
