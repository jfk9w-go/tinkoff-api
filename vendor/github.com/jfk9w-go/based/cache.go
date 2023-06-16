package based

import (
	"context"

	"github.com/pkg/errors"
)

type WriteThroughCacheStorage[K comparable, V comparable] interface {
	Load(ctx context.Context, key K) (V, error)
	Update(ctx context.Context, key K, value V) error
}

type WriteThroughCacheStorageFunc[K comparable, V comparable] struct {
	LoadFn   func(ctx context.Context, key K) (V, error)
	UpdateFn func(ctx context.Context, key K, value V) error
}

func (f WriteThroughCacheStorageFunc[K, V]) Load(ctx context.Context, key K) (V, error) {
	return f.LoadFn(ctx, key)
}

func (f WriteThroughCacheStorageFunc[K, V]) Update(ctx context.Context, key K, value V) error {
	return f.UpdateFn(ctx, key, value)
}

type WriteThroughCache[K comparable, V comparable] struct {
	storage WriteThroughCacheStorage[K, V]
	values  map[K]V
	mu      RWMutex
}

func NewWriteThroughCache[K comparable, V comparable](storage WriteThroughCacheStorage[K, V]) *WriteThroughCache[K, V] {
	return &WriteThroughCache[K, V]{
		storage: storage,
		values:  make(map[K]V),
	}
}

func (c *WriteThroughCache[K, V]) Update(ctx context.Context, key K, value V) error {
	ctx, cancel := c.mu.Lock(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := c.storage.Update(ctx, key, value); err != nil {
		return errors.Wrap(err, "update value in storage")
	}

	c.values[key] = value
	return nil
}

func (c *WriteThroughCache[K, V]) Get(ctx context.Context, key K) (V, error) {
	var zero V
	if value, err := c.getFromCache(ctx, key); value != zero || err != nil {
		return value, err
	}

	ctx, cancel := c.mu.Lock(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	if value, ok := c.values[key]; ok {
		return value, nil
	}

	value, err := c.storage.Load(ctx, key)
	if err == nil && value != zero {
		c.values[key] = value
	}

	return value, errors.Wrap(err, "get value from storage")
}

func (c *WriteThroughCache[K, V]) getFromCache(ctx context.Context, key K) (V, error) {
	var zero V
	ctx, cancel := c.mu.RLock(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	if value, ok := c.values[key]; ok {
		return value, nil
	}

	return zero, nil
}

type WriteThroughCached[V comparable] struct {
	getFn    func(ctx context.Context) (V, error)
	updateFn func(ctx context.Context, value V) error
}

func NewWriteThroughCached[K comparable, V comparable](storage WriteThroughCacheStorage[K, V], key K) *WriteThroughCached[V] {
	cache := NewWriteThroughCache[K, V](storage)
	return &WriteThroughCached[V]{
		getFn:    func(ctx context.Context) (V, error) { return cache.Get(ctx, key) },
		updateFn: func(ctx context.Context, value V) error { return cache.Update(ctx, key, value) },
	}
}

func (c *WriteThroughCached[V]) Get(ctx context.Context) (V, error) {
	return c.getFn(ctx)
}

func (c *WriteThroughCached[V]) Update(ctx context.Context, value V) error {
	return c.updateFn(ctx, value)
}
