package retry

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v5"
)

type Options struct {
	MaxCount    uint
	Interval    time.Duration
	MaxInterval time.Duration
	Exponent    float64
}

func CreateOptions() Options {
	return Options{
		MaxCount:    3,
		Interval:    1 * time.Second,
		MaxInterval: 30 * time.Second,
		Exponent:    1.0,
	}
}

func (o Options) WithMaxCount(count uint) Options {
	o.MaxCount = count
	return o
}

func (o Options) WithInterval(interval time.Duration) Options {
	o.Interval = interval
	return o
}

func (o Options) WithMaxInterval(interval time.Duration) Options {
	o.MaxInterval = interval
	return o
}

func (o Options) WithExponent(exponent float64) Options {
	o.Exponent = exponent
	return o
}

type Handler[R any] func() (R, error)

func Do[R any](ctx context.Context, handler Handler[R], options Options) (R, error) {
	operation := func() (R, error) {
		return handler()
	}

	return backoff.Retry(ctx, operation, backoff.WithBackOff(&backoff.ExponentialBackOff{
		InitialInterval: options.Interval,
		Multiplier:      options.Exponent,
		MaxInterval:     options.MaxInterval,
	}), backoff.WithMaxTries(options.MaxCount))
}
