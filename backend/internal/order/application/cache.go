package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	"golang.org/x/sync/singleflight"
)

func (s *Service) listCartCached(ctx context.Context, userID string) ([]*domain.CartItem, error) {
	if s.cache == nil {
		return s.repository.ListCart(ctx, userID)
	}
	versionKey := cartVersionKey(userID)
	version, err := s.cache.Version(ctx, versionKey)
	if err != nil {
		cartCacheWarning(ctx, "read cart cache version", err)
		return s.repository.ListCart(ctx, userID)
	}
	key := fmt.Sprintf("cache:carts:user:%s:v%d", userID, version)
	return loadCartCache(ctx, s.cache, &s.cacheFlight, key, s.cacheTTL, s.lockTTL, func() ([]*domain.CartItem, error) {
		return s.repository.ListCart(ctx, userID)
	})
}

func (s *Service) invalidateCart(ctx context.Context, userID string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.BumpVersion(ctx, cartVersionKey(userID)); err != nil {
		cartCacheWarning(ctx, "invalidate cart cache", err)
	}
}

func cartVersionKey(userID string) string {
	return "cache:carts:user:" + userID + ":version"
}

func loadCartCache(
	ctx context.Context,
	cache Cache,
	flight *singleflight.Group,
	key string,
	ttl, lockTTL time.Duration,
	load func() ([]*domain.CartItem, error),
) ([]*domain.CartItem, error) {
	if value, hit, err := readCartCache(ctx, cache, key); err == nil && hit {
		return value, nil
	} else if err != nil {
		cartCacheWarning(ctx, "read cart cache", err)
	}

	result, err, _ := flight.Do(key, func() (any, error) {
		if value, hit, cacheErr := readCartCache(ctx, cache, key); cacheErr == nil && hit {
			return value, nil
		}
		token, locked, lockErr := cache.TryLock(ctx, key, lockTTL)
		if lockErr != nil {
			cartCacheWarning(ctx, "acquire cart cache stampede lock", lockErr)
		} else if locked {
			defer releaseCartCacheLock(ctx, cache, key, token)
		} else if value, hit := waitForCartCache(ctx, cache, key); hit {
			return value, nil
		}

		value, loadErr := load()
		if loadErr != nil {
			return nil, loadErr
		}
		if cacheErr := cache.SetJSON(ctx, key, value, ttl); cacheErr != nil {
			cartCacheWarning(ctx, "write cart cache", cacheErr)
		}
		return value, nil
	})
	if err != nil {
		return nil, err
	}
	value, ok := result.([]*domain.CartItem)
	if !ok {
		return nil, fmt.Errorf("unexpected cached cart value type")
	}
	return value, nil
}

func readCartCache(ctx context.Context, cache Cache, key string) ([]*domain.CartItem, bool, error) {
	var value []*domain.CartItem
	hit, err := cache.GetJSON(ctx, key, &value)
	return value, hit, err
}

func waitForCartCache(ctx context.Context, cache Cache, key string) ([]*domain.CartItem, bool) {
	for range 3 {
		timer := time.NewTimer(15 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, false
		case <-timer.C:
		}
		value, hit, err := readCartCache(ctx, cache, key)
		if err == nil && hit {
			return value, true
		}
	}
	return nil, false
}

func releaseCartCacheLock(parent context.Context, cache Cache, key, token string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 250*time.Millisecond)
	defer cancel()
	if err := cache.Unlock(ctx, key, token); err != nil {
		slog.Warn("release cart cache stampede lock failed", "error", err)
	}
}

func cartCacheWarning(ctx context.Context, operation string, err error) {
	slog.WarnContext(ctx, operation+" failed; using PostgreSQL", "error", err)
}
