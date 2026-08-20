.PHONY: help install dev build preview check test lint type-check clean docker-build docker-run

help:
	@echo "install       Cài dependency bằng pnpm"
	@echo "dev           Chạy Vite hot reload tại :5174"
	@echo "check         Format, type-check, lint, test và production build"
	@echo "docker-build  Build production image"
	@echo "docker-run    Chạy production image tại :4174"

install:
	pnpm install

dev:
	pnpm dev

build:
	pnpm build

preview:
	pnpm preview

type-check:
	pnpm type-check

lint:
	pnpm lint

test:
	pnpm test

check:
	pnpm check

clean:
	rm -rf dist

docker-build:
	docker build \
		--build-arg VITE_API_BASE_URL=http://localhost:8080 \
		--build-arg VITE_STOREFRONT_URL=http://localhost:5173 \
		-t bookstore-admin-portal .

docker-run:
	docker run --rm -p 4174:80 bookstore-admin-portal
