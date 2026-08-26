package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	content := []byte(`
gateway:
  http_address: ":8080"
  allowed_origins: ["http://localhost:5173"]
  refresh_cookie_name: "bookstore_refresh"
  refresh_cookie_secure: false
  refresh_cookie_same_site: "lax"
  request_timeout: "2s"
  performance_target: "200ms"
  read_header_timeout: "2s"
  read_timeout: "5s"
  write_timeout: "10s"
  idle_timeout: "60s"
grpc:
  auth_address: "auth:50051"
  user_address: "user:50052"
  book_address: "book:50053"
  order_address: "order:50054"
  payment_address: "payment:50055"
  auth_listen_address: ":50051"
  user_listen_address: ":50052"
  book_listen_address: ":50053"
  order_listen_address: ":50054"
  payment_listen_address: ":50055"
  call_timeout: "1500ms"
postgres:
  url: "postgres://bookstore:bookstore@postgres:5432/bookstore"
  max_open_connections: 25
  max_idle_connections: 10
  connection_max_lifetime: "30m"
  connection_max_idle_time: "5m"
auth:
  jwt_secret: "12345678901234567890123456789012"
  jwt_issuer: "book-store-auth"
  access_token_ttl: "15m"
  refresh_token_ttl: "168h"
payment:
  currency: "VND"
  platform_owner_id: "platform"
  funding_owner_id: "system:funding"
  clearing_owner_id: "gateway:vnpay:clearing"
  default_provider: "wallet"
  platform_fee_bps: 1000
  reconcile_interval: "1m"
  reconcile_grace: "2m"
  reconcile_batch_size: 100
  vnpay:
    enabled: false
    pay_url: "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"
    api_url: "https://sandbox.vnpayment.vn/merchant_webapi/api/transaction"
    tmn_code: ""
    hash_secret: ""
    return_url: "http://localhost:5173/thanh-toan/ket-qua"
    server_ip: "127.0.0.1"
    timezone: "Asia/Ho_Chi_Minh"
    expire_after: "15m"
    http_timeout: "5s"
commerce:
  stock_reservation_ttl: "15m"
  reconcile_interval: "5s"
  reconcile_batch_size: 100
  payment_reconcile_grace: "30s"
redis:
  enabled: true
  address: "redis:6379"
  password: ""
  database: 0
  namespace: "bookstore-test"
  dial_timeout: "500ms"
  read_timeout: "50ms"
  write_timeout: "50ms"
  pool_size: 5
  book_ttl: "1m"
  cart_ttl: "5m"
  lock_ttl: "3s"
rabbitmq:
  url: "amqp://bookstore:bookstore@rabbitmq:5672/"
  exchange: "bookstore.events"
  user_profile_queue: "user.profile.create"
  account_registered_routing_key: "account.registered"
  account_deleted_routing_key: "account.deleted"
  payment_events_queue: "order.payment-events"
  payment_succeeded_routing_key: "payment.succeeded"
  payment_failed_routing_key: "payment.failed"
  payment_refunded_routing_key: "payment.refunded"
  payment_consumer_name: "test-payment-worker"
  consumer_name: "test-worker"
  consumer_concurrency: 2
  prefetch: 4
outbox:
  outbox_poll_interval: "2s"
shutdown:
  timeout: "12s"
logging:
  directory: "logs"
  level: "info"
  format: "json"
  timezone: "Asia/Ho_Chi_Minh"
  also_stdout: true
  max_size_mb: 100
  max_age_days: 14
  max_backups: 30
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Postgres.URL == "" || cfg.Redis.Address != "redis:6379" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Shutdown.Timeout != "12s" {
		t.Fatalf("Shutdown.Timeout = %q, want %q", cfg.Shutdown.Timeout, "12s")
	}
	if cfg.GRPC.CallTimeout != "1500ms" {
		t.Fatalf("GRPC.CallTimeout = %q, want %q", cfg.GRPC.CallTimeout, "1500ms")
	}
}

func TestLoadWithSecretOverride(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "config.yml")
	overridePath := filepath.Join(t.TempDir(), "local.secret.yml")
	base, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "local.yml.example"))
	if err != nil {
		t.Fatalf("read base config: %v", err)
	}
	if err := os.WriteFile(basePath, base, 0o600); err != nil {
		t.Fatalf("write base config: %v", err)
	}
	if err := os.WriteFile(overridePath, []byte(`
auth:
  facebook_app_id: "facebook-app"
  facebook_app_secret: "facebook-secret"
  facebook_graph_version: "v25.0"
`), 0o600); err != nil {
		t.Fatalf("write secret override: %v", err)
	}

	cfg, err := LoadWithOverride(basePath, overridePath)
	if err != nil {
		t.Fatalf("LoadWithOverride() error = %v", err)
	}
	if cfg.Auth.FacebookAppID != "facebook-app" || cfg.Auth.FacebookAppSecret != "facebook-secret" {
		t.Fatalf("Facebook credentials were not merged: %+v", cfg.Auth)
	}
}
