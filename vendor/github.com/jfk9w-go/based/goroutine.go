package based

import "context"

// Goroutine describes goroutine handle.
type Goroutine interface {

	// Cancel signals cancellation to the goroutine.
	Cancel()

	// Join waits for goroutine to finish.
	Join(ctx context.Context) error
}

type goroutine struct {
	cancel context.CancelFunc
	join   Ref[Unit]
}

func (gr *goroutine) Cancel() {
	gr.cancel()
}

func (gr *goroutine) Join(ctx context.Context) error {
	_, err := gr.join(ctx)
	return err
}

// Go starts the provided function in a new goroutine and returns the Goroutine handle.
func Go(ctx context.Context, fn func(ctx context.Context)) Goroutine {
	ctx, cancel := context.WithCancel(ctx)
	join := Future[Unit](ctx, func(ctx context.Context) (_ Unit, _ error) {
		defer cancel()
		fn(ctx)
		return
	})

	return &goroutine{
		cancel: cancel,
		join:   join,
	}
}
