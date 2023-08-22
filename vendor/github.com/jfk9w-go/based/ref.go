package based

import (
	"context"
	"sync/atomic"

	"github.com/pkg/errors"
)

// Ref references a value which may require calculation.
type Ref[T any] func(ctx context.Context) (T, error)

// Safe calls Ref with panic handling and other safeguards.
func (ref Ref[T]) Safe(ctx context.Context) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			if r, ok := r.(error); ok {
				err = r
				return
			}

			err = errors.Errorf("panic: %v", r)
		}
	}()

	if ref == nil {
		err = errors.New("ref is nil")
		return
	}

	result, err = ref(ctx)
	return
}

// Lazy creates a Ref with on-demand execution of the provided Ref.
// The result of the first execution will be cached and returned immediately for all consequent calls.
func Lazy[T any](ref Ref[T]) Ref[T] {
	var (
		result T
		err    error
		mu     RWMutex

		zero    T
		started atomic.Bool
		done    atomic.Bool
	)

	return func(ctx context.Context) (T, error) {
		if !done.Load() {
			if started.CompareAndSwap(false, true) {
				ctx, cancel := mu.Lock(ctx)
				defer cancel()
				if err := ctx.Err(); err != nil {
					return zero, err
				}

				result, err = ref.Safe(ctx)
				done.Store(true)
			} else {
				ctx, cancel := mu.RLock(ctx)
				defer cancel()
				if err := ctx.Err(); err != nil {
					return zero, err
				}
			}
		}

		return result, err
	}
}

// Future represents a result of an asynchronous computation that may or may not be available yet.
func Future[T any](ctx context.Context, ref Ref[T]) Ref[T] {
	var (
		result T
		err    error
		mu     RWMutex

		zero T
		done atomic.Bool
	)

	ctx, cancel := mu.Lock(ctx)
	go func() {
		defer cancel()
		if err = ctx.Err(); err != nil {
			return
		}

		result, err = ref.Safe(ctx)
		done.Store(true)
	}()

	return func(ctx context.Context) (T, error) {
		if !done.Load() {
			ctx, cancel := mu.RLock(ctx)
			defer cancel()
			if err := ctx.Err(); err != nil {
				return zero, err
			}
		}

		return result, err
	}
}
