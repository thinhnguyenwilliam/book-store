#!/usr/bin/env bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
runtime_dir="$(mktemp -d)"
pids=()

cleanup() {
  local pid
  for pid in "${pids[@]:-}"; do
    kill -TERM "${pid}" 2>/dev/null || true
  done
  for pid in "${pids[@]:-}"; do
    wait "${pid}" 2>/dev/null || true
  done
  rm -rf "${runtime_dir}"
}
trap cleanup EXIT INT TERM

cd "${project_dir}"

docker compose up -d --no-build postgres redis rabbitmq >/dev/null
postgres_ready=false
for _ in $(seq 1 40); do
  if docker compose exec -T postgres pg_isready -U bookstore -d bookstore >/dev/null 2>&1; then
    postgres_ready=true
    break
  fi
  sleep 0.25
done
if [[ "${postgres_ready}" != "true" ]]; then
  echo "PostgreSQL did not become ready" >&2
  exit 1
fi
redis_ready=false
for _ in $(seq 1 40); do
  redis_response="$(docker compose exec -T redis redis-cli ping 2>/dev/null || true)"
  if [[ "${redis_response}" == "PONG" ]]; then
    redis_ready=true
    break
  fi
  sleep 0.25
done
if [[ "${redis_ready}" != "true" ]]; then
  echo "Redis did not become ready" >&2
  exit 1
fi
make migrate >/dev/null

for service in auth-service book-service payment-service order-service gateway; do
  GOCACHE="${project_dir}/.cache/go-build" go build -buildvcs=false \
    -o "${runtime_dir}/${service}" "./cmd/${service}"
done

start_service() {
  local service="$1"
  "${runtime_dir}/${service}" -config "${project_dir}/config/local.yml" \
    >"${runtime_dir}/${service}.log" 2>&1 &
  pids+=("$!")
}

start_service auth-service
start_service book-service
start_service payment-service
start_service order-service
start_service gateway

ready=false
for _ in $(seq 1 60); do
  if curl --silent --fail "${E2E_BASE_URL:-http://localhost:8080}/healthz" >/dev/null; then
    ready=true
    break
  fi
  sleep 0.25
done

if [[ "${ready}" != "true" ]]; then
  echo "local checkout stack did not become ready" >&2
  for service in auth-service book-service payment-service order-service gateway; do
    echo "--- ${service} ---" >&2
    tail -n 30 "${runtime_dir}/${service}.log" >&2 || true
  done
  exit 1
fi

sleep 2
if ! "${project_dir}/scripts/e2e-checkout.sh"; then
  echo "checkout failed; service logs:" >&2
  for service in auth-service book-service payment-service order-service gateway; do
    echo "--- ${service} ---" >&2
    tail -n 50 "${runtime_dir}/${service}.log" >&2 || true
  done
  exit 1
fi

mapfile -t cache_keys < <(docker compose exec -T redis redis-cli --scan \
  --pattern 'bookstore-local:cache:*')
if (( ${#cache_keys[@]} < 2 )); then
  echo "Redis cache verification failed: expected book and cart keys" >&2
  exit 1
fi
echo "Redis cache verification passed: ${#cache_keys[@]} keys"
