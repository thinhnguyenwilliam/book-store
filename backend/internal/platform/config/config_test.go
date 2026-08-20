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
  auth_listen_address: ":50051"
  user_listen_address: ":50052"
  book_listen_address: ":50053"
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
redis:
  address: "redis:6379"
  password: ""
  database: 0
rabbitmq:
  url: "amqp://bookstore:bookstore@rabbitmq:5672/"
  exchange: "bookstore.events"
  user_profile_queue: "user.profile.create"
  account_registered_routing_key: "account.registered"
  account_deleted_routing_key: "account.deleted"
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
}
