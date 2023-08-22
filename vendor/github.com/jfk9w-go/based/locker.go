package based

import (
	"context"
	"math"
	"sync"
	"time"
)

// Locker provides synchronization semantics similar to mutex.
// Implementations must provide interruptibility and reentrant locking.
type Locker interface {

	// Lock acquires the lock.
	// It returns a new Context which should be checked for error before proceeding with the execution.
	// Context scope must be limited to holding the lock.
	// CancelFunc must not be nil and is used in defer statement to release the lock or perform other cleanup,
	// even in cases of context errors.
	//
	// Example usage:
	//  ctx, cancel := mu.Lock(ctx)
	//  defer cancel()
	//  if err := ctx.Err(); err != nil {
	//    return err
	//  }
	Lock(ctx context.Context) (context.Context, context.CancelFunc)
}

// LockerFunc is a functional Locker adapter.
type LockerFunc func(ctx context.Context) (context.Context, context.CancelFunc)

func (fn LockerFunc) Lock(ctx context.Context) (context.Context, context.CancelFunc) {
	return fn(ctx)
}

// Lockers provides a way to simultaneously acquire and release multiple locks.
type Lockers []Locker

func (ls Lockers) Lock(ctx context.Context) (context.Context, context.CancelFunc) {
	return ReentrantLock(ctx, ls, 1, ls.doLock)
}

func (ls Lockers) doLock(ctx context.Context) (context.Context, context.CancelFunc) {
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

type semaphore struct {
	clock    Clock
	interval time.Duration
	c        chan time.Time
}

// Semaphore allows at most `size` events at given time interval.
// This becomes a classic semaphore when interval = 0, and a simple mutex when size = 1.
func Semaphore(clock Clock, size int, interval time.Duration) Locker {
	if size <= 0 {
		return Unlocker
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
	return ReentrantLock(ctx, s, 1, s.doLock)
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
					return ctx, Nop
				}
			}
		}

		return ctx, func() { s.c <- s.clock.Now() }
	case <-ctx.Done():
		return ctx, Nop
	}
}

// RWMutex is a sync.RWMutex implementation.
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
	return ReentrantLock(ctx, m, 1, m.doLock)
}

func (m *RWMutex) RLock(ctx context.Context) (context.Context, context.CancelFunc) {
	return ReentrantLock(ctx, m, 2, m.doRLock)
}

func (m *RWMutex) doLock(ctx context.Context) (context.Context, context.CancelFunc) {
	m.once.Do(m.init)
	select {
	case m.w <- true:
		return ctx, func() { <-m.w }
	case <-ctx.Done():
		return ctx, Nop
	}
}

func (m *RWMutex) doRLock(ctx context.Context) (context.Context, context.CancelFunc) {
	m.once.Do(m.init)
	var rs int
	select {
	case m.w <- true:
	case rs = <-m.r:
	case <-ctx.Done():
		return ctx, Nop
	}

	rs++
	m.r <- rs
	return ctx, func() {
		rs := <-m.r
		rs--
		if rs == 0 {
			<-m.w
		} else {
			m.r <- rs
		}
	}
}

// Unlocker does nothing.
var Unlocker LockerFunc = func(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, Nop
}

// ReentrantLock will check context to see if the lock with required level or higher is already acquired.
// If not, it will call provided LockerFunc.
func ReentrantLock(ctx context.Context, key any, requiredLevel uint, lock LockerFunc) (context.Context, context.CancelFunc) {
	contextLevel := uint(math.MaxUint)
	if level, ok := ctx.Value(key).(uint); ok {
		contextLevel = level
	}

	if contextLevel <= requiredLevel {
		return ctx, Nop
	}

	ctx, cancel := lock(ctx)
	if ctx.Err() != nil {
		return ctx, Nop
	}

	var once sync.Once
	return context.WithValue(ctx, key, requiredLevel), func() { once.Do(cancel) }
}
