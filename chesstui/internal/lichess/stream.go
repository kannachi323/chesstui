package lichess

import (
	"context"
	"time"
)

// Reconnect keeps a stream open, reopening it with capped exponential backoff
// after network drops or server errors. It returns when ctx ends or when the
// stream fails in a way retrying cannot fix. onRetry runs before each wait.
func Reconnect(ctx context.Context, open func(context.Context) error, onRetry func(wait time.Duration, err error)) error {
	wait := 2 * time.Second
	for {
		start := time.Now()
		err := open(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && !Retryable(err) {
			return err
		}
		if time.Since(start) > time.Minute {
			wait = 2 * time.Second // the stream was healthy; start over
		}
		delay := wait
		if rate, ok := err.(*RateLimitError); ok {
			delay = rate.Wait()
		}
		if onRetry != nil {
			onRetry(delay, err)
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		wait = min(wait*2, 30*time.Second)
	}
}
