package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
	"golang.org/x/sync/singleflight"
)

const bookCacheVersionKey = "cache:books:version"

func (s *Service) getCached(ctx context.Context, id string) (*domain.Book, error) {
	if s.cache == nil {
		return s.repository.FindByID(ctx, id)
	}
	version, err := s.cache.Version(ctx, bookCacheVersionKey)
	if err != nil {
		cacheWarning(ctx, "read book cache version", err)
		return s.repository.FindByID(ctx, id)
	}
	key := fmt.Sprintf("cache:books:v%d:id:%s", version, id)
	return cacheLoad(ctx, s.cache, &s.cacheFlight, key, s.cacheTTL, s.cacheLockTTL, func() (*domain.Book, error) {
		return s.repository.FindByID(ctx, id)
	})
}

func (s *Service) listCached(
	ctx context.Context,
	rawCursor string,
	limit int32,
	cursor *BookCursor,
) (BookPage, error) {
	load := func() (BookPage, error) {
		books, err := s.repository.List(ctx, limit+1, cursor)
		if err != nil {
			return BookPage{}, err
		}
		hasMore := len(books) > int(limit)
		if hasMore {
			books = books[:limit]
		}
		page := BookPage{Books: books, HasMore: hasMore}
		if !hasMore || len(books) == 0 {
			return page, nil
		}
		lastBook := books[len(books)-1]
		page.NextCursor, err = encodeCursor(BookCursor{CreatedAt: lastBook.CreatedAt, ID: lastBook.ID})
		return page, err
	}
	if s.cache == nil {
		return load()
	}
	version, err := s.cache.Version(ctx, bookCacheVersionKey)
	if err != nil {
		cacheWarning(ctx, "read book list cache version", err)
		return load()
	}
	cursorHash := sha256.Sum256([]byte(rawCursor))
	key := fmt.Sprintf("cache:books:v%d:list:%d:%x", version, limit, cursorHash[:8])
	return cacheLoad(ctx, s.cache, &s.cacheFlight, key, s.cacheTTL, s.cacheLockTTL, load)
}

func (s *Service) invalidateCache(ctx context.Context) {
	if s.cache == nil {
		return
	}
	if err := s.cache.BumpVersion(ctx, bookCacheVersionKey); err != nil {
		cacheWarning(ctx, "invalidate book cache", err)
	}
}

func cacheLoad[T any](
	ctx context.Context,
	cache Cache,
	flight *singleflight.Group,
	key string,
	ttl, lockTTL time.Duration,
	load func() (T, error),
) (T, error) {
	if value, hit, err := cacheRead[T](ctx, cache, key); err == nil && hit {
		return value, nil
	} else if err != nil {
		cacheWarning(ctx, "read cache", err)
	}

	result, err, _ := flight.Do(key, func() (any, error) {
		if value, hit, cacheErr := cacheRead[T](ctx, cache, key); cacheErr == nil && hit {
			return value, nil
		}
		token, locked, lockErr := cache.TryLock(ctx, key, lockTTL)
		if lockErr != nil {
			cacheWarning(ctx, "acquire cache stampede lock", lockErr)
		} else if locked {
			defer releaseCacheLock(ctx, cache, key, token)
		} else if value, hit := waitForCache[T](ctx, cache, key); hit {
			return value, nil
		}

		value, loadErr := load()
		if loadErr != nil {
			return value, loadErr
		}
		if cacheErr := cache.SetJSON(ctx, key, value, ttl); cacheErr != nil {
			cacheWarning(ctx, "write cache", cacheErr)
		}
		return value, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	value, ok := result.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("unexpected cached value type")
	}
	return value, nil
}

func cacheRead[T any](ctx context.Context, cache Cache, key string) (T, bool, error) {
	value := new(T)
	hit, err := cache.GetJSON(ctx, key, value)
	return *value, hit, err
}

func waitForCache[T any](ctx context.Context, cache Cache, key string) (T, bool) {
	for range 3 {
		timer := time.NewTimer(15 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			var zero T
			return zero, false
		case <-timer.C:
		}
		value, hit, err := cacheRead[T](ctx, cache, key)
		if err == nil && hit {
			return value, true
		}
	}
	var zero T
	return zero, false
}

func releaseCacheLock(parent context.Context, cache Cache, key, token string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 250*time.Millisecond)
	defer cancel()
	if err := cache.Unlock(ctx, key, token); err != nil {
		slog.Warn("release cache stampede lock failed", "error", err)
	}
}

func cacheWarning(ctx context.Context, operation string, err error) {
	slog.WarnContext(ctx, operation+" failed; using PostgreSQL", "error", err)
}
