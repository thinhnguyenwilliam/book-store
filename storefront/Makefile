.PHONY: help install dev build preview check test lint type-check clean docker-build docker-run

help:
	@echo "install       Cài dependency bằng pnpm"
	@echo "dev           Chạy Vite hot reload tại :5173"
	@echo "check         Type-check, lint, test và production build"
	@echo "docker-build  Build production image"
	@echo "docker-run    Chạy production image tại :4173"

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
	docker build --build-arg VITE_API_BASE_URL=http://localhost:8080 -t bookstore-storefront .

docker-run:
	docker run --rm -p 4173:80 bookstore-storefront
