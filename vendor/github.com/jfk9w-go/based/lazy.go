package based

import "context"

type LazyFunc[T any] func(ctx context.Context) (T, error)

type Lazy[T any] struct {
	Fn    LazyFunc[T]
	value T
	err   error
	done  bool
	mu    RWMutex
}

func (l *Lazy[T]) Get(ctx context.Context) (T, error) {
	if done, value, err := l.get(ctx); done {
		return value, err
	}

	ctx, cancel := l.mu.Lock(ctx)
	defer cancel()
	if ctx.Err() != nil {
		var zero T
		return zero, ctx.Err()
	}

	if !l.done {
		l.value, l.err = l.Fn(ctx)
		l.done = true
	}

	return l.value, l.err
}

func (l *Lazy[T]) get(ctx context.Context) (done bool, value T, err error) {
	ctx, cancel := l.mu.RLock(ctx)
	defer cancel()
	if ctx.Err() != nil {
		var zero T
		return true, zero, ctx.Err()
	}

	return l.done, l.value, l.err
}
