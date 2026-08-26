#!/usr/bin/env bash

set -euo pipefail

base_url="${E2E_BASE_URL:-http://localhost:8080}"
email="${E2E_EMAIL:-checkout.e2e@example.com}"
password="${E2E_PASSWORD:-local-checkout-password123}"
seller_id="33333333-3333-4333-8333-333333333333"
run_id="$(date +%s)-${RANDOM}"
work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

request_status() {
  local output_file="$1"
  shift
  curl --silent --show-error --output "${output_file}" --write-out '%{http_code}' "$@"
}

assert_status() {
  local actual="$1"
  local expected="$2"
  local response_file="$3"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "unexpected HTTP status: got ${actual}, want ${expected}" >&2
    jq -c '{error: .error}' "${response_file}" >&2 || true
    exit 1
  fi
}

register_status="$(request_status "${work_dir}/register.json" \
  --request POST "${base_url}/api/v1/auth/register" \
  --header 'Content-Type: application/json' \
  --data "{\"email\":\"${email}\",\"password\":\"${password}\",\"display_name\":\"Checkout E2E\"}")"
if [[ "${register_status}" != "201" && "${register_status}" != "409" ]]; then
  assert_status "${register_status}" "201" "${work_dir}/register.json"
fi

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U bookstore -d bookstore \
  --command "UPDATE auth.accounts SET roles = ARRAY['customer','admin']::TEXT[], updated_at = NOW() WHERE email = '${email}'" \
  >/dev/null

login_status="$(request_status "${work_dir}/login.json" \
  --request POST "${base_url}/api/v1/auth/login" \
  --header 'Content-Type: application/json' \
  --data "{\"email\":\"${email}\",\"password\":\"${password}\"}")"
assert_status "${login_status}" "200" "${work_dir}/login.json"
access_token="$(jq -er '.access_token' "${work_dir}/login.json")"
user_id="$(jq -er '.user_id' "${work_dir}/login.json")"
authorization="Authorization: Bearer ${access_token}"

wallet_status="$(request_status "${work_dir}/wallet.json" \
  --request POST "${base_url}/api/v1/wallets/me" \
  --header "${authorization}")"
assert_status "${wallet_status}" "201" "${work_dir}/wallet.json"

fund_status="$(request_status "${work_dir}/fund.json" \
  --request PUT "${base_url}/api/v1/admin/wallets/${user_id}/balance" \
  --header "${authorization}" \
  --header 'Content-Type: application/json' \
  --header "Idempotency-Key: fund-${run_id}" \
  --data '{"delta_cents":1000000,"reason":"automated local checkout test"}')"
assert_status "${fund_status}" "200" "${work_dir}/fund.json"

book_status="$(request_status "${work_dir}/book.json" \
  --request POST "${base_url}/api/v1/admin/books" \
  --header "${authorization}" \
  --header 'Content-Type: application/json' \
  --data "{\"title\":\"Checkout E2E ${run_id}\",\"author\":\"Book Store\",\"isbn\":\"e2e-${run_id}\",\"price_cents\":10000,\"stock\":5,\"seller_id\":\"${seller_id}\"}")"
assert_status "${book_status}" "201" "${work_dir}/book.json"
book_id="$(jq -er '.id' "${work_dir}/book.json")"

for attempt in 1 2; do
  get_book_status="$(request_status "${work_dir}/get-book-${attempt}.json" \
    "${base_url}/api/v1/books/${book_id}")"
  assert_status "${get_book_status}" "200" "${work_dir}/get-book-${attempt}.json"
done

for attempt in 1 2; do
  list_books_status="$(request_status "${work_dir}/list-books-${attempt}.json" \
    "${base_url}/api/v1/books?limit=20")"
  assert_status "${list_books_status}" "200" "${work_dir}/list-books-${attempt}.json"
done

cart_status="$(request_status "${work_dir}/cart.json" \
  --request POST "${base_url}/api/v1/cart/items" \
  --header "${authorization}" \
  --header 'Content-Type: application/json' \
  --data "{\"bookId\":\"${book_id}\",\"quantity\":2}")"
assert_status "${cart_status}" "201" "${work_dir}/cart.json"

for attempt in 1 2; do
  list_cart_status="$(request_status "${work_dir}/list-cart-${attempt}.json" \
    "${base_url}/api/v1/cart/items" \
    --header "${authorization}")"
  assert_status "${list_cart_status}" "200" "${work_dir}/list-cart-${attempt}.json"
done

order_status="$(request_status "${work_dir}/order.json" \
  --request POST "${base_url}/api/v1/orders" \
  --header "${authorization}" \
  --header "Idempotency-Key: order-${run_id}")"
assert_status "${order_status}" "201" "${work_dir}/order.json"
order_id="$(jq -er '.id' "${work_dir}/order.json")"
jq -e '.status == "stock_reserved" and .total_cents == 20000' "${work_dir}/order.json" >/dev/null

payment_status="$(request_status "${work_dir}/payment.json" \
  --request POST "${base_url}/api/v1/payments" \
  --header "${authorization}" \
  --header 'Content-Type: application/json' \
  --header "Idempotency-Key: payment-${run_id}" \
  --data "{\"orderId\":\"${order_id}\"}")"
assert_status "${payment_status}" "201" "${work_dir}/payment.json"
payment_id="$(jq -er '.id' "${work_dir}/payment.json")"
jq -e '.status == "succeeded" and .amount_cents == 20000 and .platform_fee_cents == 2000' \
  "${work_dir}/payment.json" >/dev/null

get_order_status="$(request_status "${work_dir}/confirmed-order.json" \
  "${base_url}/api/v1/orders/${order_id}" \
  --header "${authorization}")"
assert_status "${get_order_status}" "200" "${work_dir}/confirmed-order.json"
jq -e --arg payment_id "${payment_id}" '.status == "confirmed" and .payment_id == $payment_id' \
  "${work_dir}/confirmed-order.json" >/dev/null

ledger_sum="$(docker compose exec -T postgres psql -At -U bookstore -d bookstore \
  --command "SELECT COALESCE(SUM(entry.amount_cents), 0) FROM payments.ledger_entries entry JOIN payments.ledger_transactions txn ON txn.id = entry.transaction_id WHERE txn.kind = 'payment' AND txn.reference_id = '${payment_id}'")"
if [[ "${ledger_sum}" != "0" ]]; then
  echo "ledger is unbalanced for payment ${payment_id}: ${ledger_sum}" >&2
  exit 1
fi

reservation_status="$(docker compose exec -T postgres psql -At -U bookstore -d bookstore \
  --command "SELECT status FROM catalog.stock_reservations WHERE order_id = '${order_id}' AND book_id = '${book_id}'")"
if [[ "${reservation_status}" != "committed" ]]; then
  echo "stock reservation status is ${reservation_status}, want committed" >&2
  exit 1
fi

echo "checkout e2e passed: order=${order_id} payment=${payment_id} ledger_sum=${ledger_sum}"
