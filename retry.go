package tinkoff

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type retryKey struct{}

type retryStrategy struct {
	backOff    retryTimeoutFunc
	maxRetries int
}

func (rs *retryStrategy) do(ctx context.Context) (context.Context, error) {
	retry, _ := ctx.Value(retryKey{}).(int)
	if rs.maxRetries > 0 && retry >= rs.maxRetries {
		return ctx, errMaxRetriesExceeded
	}

	select {
	case <-time.After(rs.backOff(retry)):
		return context.WithValue(ctx, retryKey{}, retry+1), nil
	case <-ctx.Done():
		return ctx, ctx.Err()
	}
}

type retryTimeoutFunc func(retry int) time.Duration

func exponentialRetryTimeout(base time.Duration, factor, jitter float64) retryTimeoutFunc {
	return func(retry int) time.Duration {
		timeout := base * time.Duration(math.Pow(factor, float64(retry)))
		return timeout * time.Duration(1+jitter*(0.5-rand.Float64()))
	}
}

func constantRetryTimeout(timeout time.Duration) retryTimeoutFunc {
	return func(retry int) time.Duration {
		return timeout
	}
}
