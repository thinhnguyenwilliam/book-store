#!/usr/bin/env bash
# Start or stop the full local Book Store stack from one command:
# Docker infrastructure + Go services on the host + Vue frontends.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="${ROOT}/backend"
STOREFRONT="${ROOT}/storefront"
ADMIN="${ROOT}/admin-portal"
RUN_DIR="${ROOT}/.run"
LOG_DIR="${RUN_DIR}/logs"
PID_DIR="${RUN_DIR}/pids"
WATCH="${WATCH:-1}"
LOCAL_CONFIG="config/local.yml"
LOCAL_SECRET_CONFIG="${LOCAL_SECRET_CONFIG:-config/local.secret.yml}"

GO_SERVICES=(
  auth-service
  user-service
  book-service
  payment-service
  order-service
  worker-service
  notification-service
  comment-service
  chat-service
  analytics-service
  search-service
  gateway
)

declare -A SERVICE_PORTS=(
  [auth-service]=50051
  [user-service]=50052
  [book-service]=50053
  [order-service]=50054
  [payment-service]=50055
  [notification-service]=50056
  [comment-service]=50057
  [chat-service]=50058
  [analytics-service]=50059
  [search-service]=50060
  [gateway]=8080
  [storefront]=5173
  [admin-portal]=5174
)

usage() {
  cat <<'EOF'
Usage: scripts/local.sh <command>

Commands:
  up        Start infra, Go services, storefront and admin portal
  down      Stop Go services and frontends; keep Docker infrastructure
  down-all  Stop apps and Docker infrastructure (volumes are kept)
  logs      Follow process logs
  status    Show which local processes are running

Environment:
  WATCH=1   Use Air hot reload for Go services (default)
  WATCH=0   Use go run without hot reload
EOF
}

log() { printf '==> %s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing command: $1"
}

secret_args() {
  if [[ -f "${BACKEND}/${LOCAL_SECRET_CONFIG}" ]]; then
    printf '%s' "-secrets ${LOCAL_SECRET_CONFIG}"
  fi
}

pid_file() { printf '%s/%s.pid' "${PID_DIR}" "$1"; }
log_file() { printf '%s/%s.log' "${LOG_DIR}" "$1"; }

pid_running() {
  local pid="${1:-}"
  [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null
}

service_pid() {
  local file
  file="$(pid_file "$1")"
  [[ -f "${file}" ]] || return 1
  cat "${file}"
}

is_running() {
  local pid
  pid="$(service_pid "$1" 2>/dev/null || true)"
  pid_running "${pid}"
}

kill_tree() {
  local signal="$1"
  local pid="$2"
  local child
  [[ -n "${pid}" ]] || return 0
  while IFS= read -r child; do
    kill_tree "${signal}" "${child}"
  done < <(pgrep -P "${pid}" 2>/dev/null || true)
  kill "-${signal}" "${pid}" 2>/dev/null || true
}

wait_pid_gone() {
  local pid="$1"
  local seconds="${2:-12}"
  local i
  for ((i = 0; i < seconds; i++)); do
    pid_running "${pid}" || return 0
    sleep 1
  done
  return 1
}

stop_service() {
  local name="$1"
  local pid
  pid="$(service_pid "${name}" 2>/dev/null || true)"
  if ! pid_running "${pid}"; then
    rm -f "$(pid_file "${name}")"
    return 0
  fi
  kill_tree TERM "${pid}"
  if ! wait_pid_gone "${pid}" 12; then
    kill_tree KILL "${pid}"
    wait_pid_gone "${pid}" 3 || true
  fi
  rm -f "$(pid_file "${name}")"
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    [[ -n "$(ss -H -ltn "sport = :${port}" 2>/dev/null)" ]]
    return
  fi
  bash -c "echo >/dev/tcp/127.0.0.1/${port}" 2>/dev/null
}

wait_port() {
  local name="$1"
  local port="$2"
  local attempts="${3:-90}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if port_in_use "${port}"; then
      return 0
    fi
    sleep 1
  done
  warn "${name} did not listen on :${port}"
  if [[ -f "$(log_file "${name}")" ]]; then
    tail -n 40 "$(log_file "${name}")" >&2 || true
  fi
  return 1
}

wait_http() {
  local name="$1"
  local url="$2"
  local attempts="${3:-90}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl -sf "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  warn "${name} did not become ready at ${url}"
  if [[ -f "$(log_file "${name}")" ]]; then
    tail -n 40 "$(log_file "${name}")" >&2 || true
  fi
  return 1
}

start_process() {
  local name="$1"
  shift
  mkdir -p "${PID_DIR}" "${LOG_DIR}"
  if is_running "${name}"; then
    log "${name} already running"
    return 0
  fi
  : >"$(log_file "${name}")"
  nohup "$@" >>"$(log_file "${name}")" 2>&1 &
  echo $! >"$(pid_file "${name}")"
  log "started ${name} (pid $(cat "$(pid_file "${name}")"))"
}

run_go_service() {
  local name="$1"
  local extra="${2:-}"
  cd "${BACKEND}"
  export GOCACHE="${BACKEND}/.cache/go-build"
  export PATH="${BACKEND}/.tools:${PATH}"
  # shellcheck disable=SC2086
  if [[ "${WATCH}" == "1" ]]; then
    exec "${BACKEND}/.tools/air" -c .air.local.toml \
      --build.cmd "go build -buildvcs=false -o ./.air-local/${name} ./cmd/${name}" \
      --build.entrypoint "./.air-local/${name}" \
      --build.log "${name}-build-errors.log" \
      -- -config "${LOCAL_CONFIG}" ${extra}
  fi
  exec go run "./cmd/${name}" -config "${LOCAL_CONFIG}" ${extra}
}

ensure_local_config() {
  if [[ ! -f "${BACKEND}/${LOCAL_CONFIG}" ]]; then
    cp "${BACKEND}/config/local.yml.example" "${BACKEND}/${LOCAL_CONFIG}"
    log "created backend/config/local.yml from example"
  fi
  if [[ ! -f "${STOREFRONT}/.env" ]]; then
    cp "${STOREFRONT}/.env.example" "${STOREFRONT}/.env"
    log "created storefront/.env from example"
  fi
  if [[ ! -f "${ADMIN}/.env" ]]; then
    cp "${ADMIN}/.env.example" "${ADMIN}/.env"
    log "created admin-portal/.env from example"
  fi
}

ensure_pnpm() {
  if command -v pnpm >/dev/null 2>&1; then
    return 0
  fi
  if command -v corepack >/dev/null 2>&1; then
    log "enabling pnpm via corepack"
    corepack enable >/dev/null
    corepack prepare pnpm@9.2.0 --activate
    return 0
  fi
  die "pnpm is required. Install Node.js 22+ and run: corepack enable"
}

ensure_frontend_deps() {
  local dir="$1"
  if [[ ! -d "${dir}/node_modules" ]]; then
    log "installing $(basename "${dir}") dependencies"
    (cd "${dir}" && pnpm install)
  fi
}

cmd_down_apps() {
  local name
  mkdir -p "${PID_DIR}" "${LOG_DIR}"
  for name in admin-portal storefront "${GO_SERVICES[@]}"; do
    stop_service "${name}"
  done
}

cmd_down_all() {
  cmd_down_apps
  log "stopping Docker infrastructure"
  make -C "${BACKEND}" infra-stop
}

cmd_status() {
  local name pid
  printf '%-22s %-10s %s\n' "SERVICE" "STATUS" "PID"
  for name in "${GO_SERVICES[@]}" storefront admin-portal; do
    if is_running "${name}"; then
      pid="$(service_pid "${name}")"
      printf '%-22s %-10s %s\n' "${name}" "running" "${pid}"
    else
      printf '%-22s %-10s %s\n' "${name}" "stopped" "-"
    fi
  done
}

cmd_logs() {
  mkdir -p "${LOG_DIR}"
  if ! compgen -G "${LOG_DIR}/*.log" >/dev/null; then
    die "No local logs yet. Run: make local"
  fi
  tail -n 50 -F "${LOG_DIR}"/*.log
}

wait_ports_free() {
  local name port busy i
  for ((i = 1; i <= 15; i++)); do
    busy=()
    for name in "${!SERVICE_PORTS[@]}"; do
      port="${SERVICE_PORTS[${name}]}"
      if port_in_use "${port}"; then
        busy+=("${name}:${port}")
      fi
    done
    if ((${#busy[@]} == 0)); then
      return 0
    fi
    sleep 1
  done
  die "Ports already in use: ${busy[*]}. Stop those processes or run: make stop"
}

cmd_up() {
  need_cmd docker
  need_cmd go
  need_cmd curl
  need_cmd make
  ensure_pnpm
  ensure_local_config

  log "stopping leftover local app processes"
  cmd_down_apps

  log "starting Docker infrastructure and freeing Go container ports"
  make -C "${BACKEND}" local-prepare
  wait_ports_free

  log "waiting for PostgreSQL"
  local i
  for ((i = 1; i <= 60; i++)); do
    if docker compose --project-directory "${BACKEND}" \
      -f "${BACKEND}/docker-compose.yml" \
      -f "${BACKEND}/compose/docker-compose.data.yml" \
      exec -T postgres pg_isready -U bookstore -d bookstore >/dev/null 2>&1; then
      break
    fi
    if ((i == 60)); then
      die "PostgreSQL did not become ready"
    fi
    sleep 1
  done

  log "applying database migrations"
  make -C "${BACKEND}" migrate

  if [[ "${WATCH}" == "1" ]]; then
    log "installing Air into backend/.tools"
    make -C "${BACKEND}" air-tool
  fi

  mkdir -p "${BACKEND}/.cache/go-build" "${BACKEND}/.air-local"

  local extra=""
  extra="$(secret_args)"
  local name
  for name in "${GO_SERVICES[@]}"; do
    case "${name}" in
      auth-service | notification-service)
        start_process "${name}" "${ROOT}/scripts/local.sh" __run-go "${name}" "${extra}"
        ;;
      *)
        start_process "${name}" "${ROOT}/scripts/local.sh" __run-go "${name}" ""
        ;;
    esac
  done

  log "waiting for gRPC services and Gateway (first Air build can take a few minutes)"
  local failed=0
  for name in "${GO_SERVICES[@]}"; do
    if [[ -n "${SERVICE_PORTS[${name}]:-}" ]]; then
      wait_port "${name}" "${SERVICE_PORTS[${name}]}" 180 || failed=1
    fi
  done
  wait_http gateway "http://localhost:8080/healthz" 60 || failed=1

  ensure_frontend_deps "${STOREFRONT}"
  ensure_frontend_deps "${ADMIN}"
  start_process storefront "${ROOT}/scripts/local.sh" __run-pnpm "${STOREFRONT}"
  start_process admin-portal "${ROOT}/scripts/local.sh" __run-pnpm "${ADMIN}"
  wait_http storefront "http://localhost:5173" 60 || failed=1
  wait_http admin-portal "http://localhost:5174" 60 || failed=1

  if ((failed != 0)); then
    printf '\nSome processes failed to become ready. Inspect logs with: make logs\n' >&2
    cmd_status
    exit 1
  fi

  cat <<'EOF'

Local stack is running.

  Storefront     http://localhost:5173
  Admin portal   http://localhost:5174
  API Gateway    http://localhost:8080
  Swagger        http://localhost:8080/swagger/index.html

  make logs      follow process logs
  make status    show PIDs
  make stop      stop Go services and frontends
  make down      stop apps and Docker infrastructure

EOF
}

main() {
  local cmd="${1:-}"
  case "${cmd}" in
    __run-go)
      run_go_service "${2:?service name required}" "${3:-}"
      ;;
    __run-pnpm)
      cd "${2:?directory required}"
      exec pnpm dev
      ;;
    up) cmd_up ;;
    down | stop) cmd_down_apps ;;
    down-all) cmd_down_all ;;
    logs) cmd_logs ;;
    status) cmd_status ;;
    -h | --help | help | "") usage ;;
    *)
      usage >&2
      die "unknown command: ${cmd}"
      ;;
  esac
}

main "$@"
