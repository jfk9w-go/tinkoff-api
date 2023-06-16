package based

import (
	"context"
	"math"
	"sync"
	"time"
)

// Locker may be locked (interruptibly).
type Locker interface {
	// Lock locks something.
	// It returns a context which should be checked for errors (via context.Context.Err()) and used for further
	// execution under the doLock. CancelFunc must be called to release the lock (usually inside defer).
	Lock(ctx context.Context) (context.Context, context.CancelFunc)
}

type Lockers []Locker

func (ls Lockers) Lock(ctx context.Context) (context.Context, context.CancelFunc) {
	var (
		cancels []context.CancelFunc
		cancel  context.CancelFunc
	)

	for _, l := range ls {
		ctx, cancel = l.Lock(ctx)
		cancels = append(cancels, cancel)
		if ctx.Err() != nil {
			break
		}
	}

	return ctx, func() {
		for i := len(cancels) - 1; i >= 0; i-- {
			cancels[i]()
		}
	}
}

// RWLocker supports read-write locks.
type RWLocker interface {
	Locker

	// RLock acts the same way as Lock does, but it allows for multiple readers to hold the lock (or a single writer).
	RLock(ctx context.Context) (context.Context, context.CancelFunc)
}

type semaphore struct {
	clock    Clock
	interval time.Duration
	c        chan time.Time
}

// Semaphore allows at most `size` events at given time interval.
// This becomes a classic semaphore when interval = 0, and a simple mutex when size = 1.
// This is a reentrantLock Locker.
func Semaphore(clock Clock, size int, interval time.Duration) Locker {
	if size <= 0 {
		return unlock{}
	}

	c := make(chan time.Time, size)
	for i := 0; i < size; i++ {
		c <- clock.Now().Add(-interval)
	}

	return &semaphore{
		clock:    clock,
		interval: interval,
		c:        c,
	}
}

func (s *semaphore) Lock(ctx context.Context) (context.Context, context.CancelFunc) {
	return reentrantLock(ctx, s, 1, s.doLock)
}

func (s *semaphore) doLock(ctx context.Context) (context.Context, context.CancelFunc) {
	select {
	case occ := <-s.c:
		if s.interval > 0 {
			wait := occ.Add(s.interval).Sub(s.clock.Now())
			if wait > 0 {
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return ctx, noop
				}
			}
		}

		var once sync.Once
		return ctx, func() { once.Do(func() { s.c <- s.clock.Now() }) }
	case <-ctx.Done():
		return ctx, noop
	}
}

// RWMutex is an interruptible reentrant sync.RWMutex implementation.
type RWMutex struct {
	w    chan bool
	r    chan int
	once sync.Once
}

func (m *RWMutex) init() {
	m.w = make(chan bool, 1)
	m.r = make(chan int, 1)
}

func (m *RWMutex) Lock(ctx context.Context) (context.Context, context.CancelFunc) {
	return reentrantLock(ctx, m, 1, m.doLock)
}

func (m *RWMutex) RLock(ctx context.Context) (context.Context, context.CancelFunc) {
	return reentrantLock(ctx, m, 2, m.doRLock)
}

func (m *RWMutex) doLock(ctx context.Context) (context.Context, context.CancelFunc) {
	m.once.Do(m.init)
	select {
	case m.w <- true:
		var once sync.Once
		return ctx, func() { once.Do(func() { <-m.w }) }
	case <-ctx.Done():
		return ctx, noop
	}
}

func (m *RWMutex) doRLock(ctx context.Context) (context.Context, context.CancelFunc) {
	m.once.Do(m.init)
	var rs int
	select {
	case m.w <- true:
	case rs = <-m.r:
	case <-ctx.Done():
		return ctx, noop
	}

	rs++
	m.r <- rs
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			rs := <-m.r
			rs--
			if rs == 0 {
				<-m.w
			} else {
				m.r <- rs
			}
		})
	}
}

// Unlock does nothing.
var Unlock Locker = unlock{}

type unlock struct{}

func (unlock) Lock(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, noop
}

func reentrantLock(ctx context.Context, key any, level int, lock ContextFunc) (context.Context, context.CancelFunc) {
	contextLevel := math.MaxInt
	if level, ok := ctx.Value(key).(int); ok {
		contextLevel = level
	}

	if contextLevel <= level {
		return ctx, noop
	}

	ctx, cancel := lock(ctx)
	if ctx.Err() != nil {
		return ctx, noop
	}

	return context.WithValue(ctx, key, level), cancel
}

func noop() {}

type WaitGroup struct {
	sync.WaitGroup
}

func (wg *WaitGroup) Spawn(ctx context.Context) (context.Context, context.CancelFunc) {
	wg.Add(1)
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			wg.Done()
		})
	}
}

func Go(ctx context.Context, contextFn ContextFunc, fn func(ctx context.Context)) context.CancelFunc {
	ctx, cancel := contextFn(ctx)
	go func(ctx context.Context, cancel context.CancelFunc) {
		defer cancel()
		fn(ctx)
	}(ctx, cancel)

	return cancel
}

func GoWithFeedback(ctx context.Context, contextFn ContextFunc, fn func(ctx context.Context)) context.CancelFunc {
	ctx, cancel := contextFn(ctx)
	var wg WaitGroup
	_ = Go(ctx, func(ctx context.Context) (context.Context, context.CancelFunc) {
		ctx, done := wg.Spawn(ctx)
		return ctx, func() {
			done()
			cancel()
		}
	}, fn)

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			wg.Wait()
		})
	}
}
