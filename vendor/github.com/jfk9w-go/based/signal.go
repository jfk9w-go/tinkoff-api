package based

import (
	"context"
	"os"
	"os/signal"
)

func AwaitSignal(ctx context.Context, signals ...os.Signal) error {
	c := make(chan os.Signal, 1)
	go signal.Notify(c, signals...)

	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
