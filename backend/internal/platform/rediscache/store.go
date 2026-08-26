package rediscache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

type Config struct {
	Address      string
	Password     string
	Database     int
	Namespace    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

type Store struct {
	client    *redis.Client
	namespace string
}

var unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func Open(ctx context.Context, config Config) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           config.Database,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolSize:     config.PoolSize,
	})
	store := &Store{client: client, namespace: strings.Trim(strings.TrimSpace(config.Namespace), ":")}
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) GetJSON(ctx context.Context, key string, destination any) (bool, error) {
	payload, err := s.client.Get(ctx, s.key(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get cache key: %w", err)
	}
	if err := json.Unmarshal(payload, destination); err != nil {
		// A corrupt or schema-incompatible cache entry must never break the API.
		_ = s.client.Del(ctx, s.key(key)).Err()
		return false, fmt.Errorf("decode cache key: %w", err)
	}
	return true, nil
}

func (s *Store) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache value: %w", err)
	}
	if err := s.client.Set(ctx, s.key(key), payload, jitterTTL(ttl)).Err(); err != nil {
		return fmt.Errorf("set cache key: %w", err)
	}
	return nil
}

func (s *Store) Version(ctx context.Context, key string) (int64, error) {
	value, err := s.client.Get(ctx, s.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get cache version: %w", err)
	}
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version < 0 {
		return 0, fmt.Errorf("decode cache version %q", value)
	}
	return version, nil
}

func (s *Store) BumpVersion(ctx context.Context, key string) error {
	if err := s.client.Incr(ctx, s.key(key)).Err(); err != nil {
		return fmt.Errorf("bump cache version: %w", err)
	}
	return nil
}

func (s *Store) TryLock(ctx context.Context, key string, ttl time.Duration) (string, bool, error) {
	token := uuid.NewString()
	locked, err := s.client.SetNX(ctx, s.key("lock:"+key), token, ttl).Result()
	if err != nil {
		return "", false, fmt.Errorf("acquire cache lock: %w", err)
	}
	return token, locked, nil
}

func (s *Store) Unlock(ctx context.Context, key, token string) error {
	if token == "" {
		return nil
	}
	if err := unlockScript.Run(ctx, s.client, []string{s.key("lock:" + key)}, token).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("release cache lock: %w", err)
	}
	return nil
}

func (s *Store) key(key string) string {
	key = strings.TrimLeft(key, ":")
	if s.namespace == "" {
		return key
	}
	return s.namespace + ":" + key
}

func jitterTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	// Spread expirations by up to 10% so a large group of keys does not expire together.
	maxJitter := ttl / 10
	if maxJitter <= 0 {
		return ttl
	}
	return ttl + time.Duration(time.Now().UnixNano()%int64(maxJitter))
}
