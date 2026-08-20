#!/usr/bin/env bash

set -euo pipefail

base_url="${PERF_BASE_URL:-http://localhost:8080}"
requests="${PERF_REQUESTS:-100}"
concurrency="${PERF_CONCURRENCY:-10}"
target_ms="${PERF_P95_TARGET_MS:-200}"

endpoints=("/healthz" "/api/v1/books?limit=20")
if [[ -n "${PERF_BOOK_ID:-}" ]]; then
  endpoints+=("/api/v1/books/${PERF_BOOK_ID}")
fi
if [[ -n "${PERF_ACCESS_TOKEN:-}" ]]; then
  endpoints+=("/api/v1/users/me" "/api/v1/admin/customers?limit=20")
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

failed=0
for endpoint in "${endpoints[@]}"; do
  result_file="$work_dir/timings.txt"
  : >"$result_file"
  url="${base_url}${endpoint}"
  auth_args=()
  if [[ -n "${PERF_ACCESS_TOKEN:-}" ]]; then
    auth_args=(-H "Authorization: Bearer ${PERF_ACCESS_TOKEN}")
  fi

  curl --fail --silent --show-error --output /dev/null --max-time 2 "${auth_args[@]}" "$url"
  export PERF_CHECK_URL="$url" PERF_CHECK_OUTPUT="$result_file" PERF_CHECK_TOKEN="${PERF_ACCESS_TOKEN:-}"
  seq "$requests" | xargs -n1 -P "$concurrency" sh -c '
    if [ -n "$PERF_CHECK_TOKEN" ]; then
      curl --fail --silent --show-error --output /dev/null --max-time 2 \
        -H "Authorization: Bearer $PERF_CHECK_TOKEN" \
        --write-out "%{time_total}\n" "$PERF_CHECK_URL"
    else
      curl --fail --silent --show-error --output /dev/null --max-time 2 \
        --write-out "%{time_total}\n" "$PERF_CHECK_URL"
    fi
  ' _ >>"$result_file"

  p95_ms="$(sort -n "$result_file" | awk -v count="$requests" '
    { values[NR] = $1 }
    END {
      rank = int(count * 0.95 + 0.999999)
      printf "%.3f", values[rank] * 1000
    }
  ')"
  average_ms="$(awk '{ total += $1 } END { printf "%.3f", total * 1000 / NR }' "$result_file")"

  printf '%-45s avg=%8sms p95=%8sms target=%sms\n' "$endpoint" "$average_ms" "$p95_ms" "$target_ms"
  if ! awk -v actual="$p95_ms" -v target="$target_ms" 'BEGIN { exit !(actual < target) }'; then
    failed=1
  fi
done

if (( failed != 0 )); then
  echo "Performance gate failed: at least one endpoint has p95 >= ${target_ms}ms" >&2
  exit 1
fi

echo "Performance gate passed"
