package background

import (
	"runtime"
	"time"
)

// config holds internal configuration resolved from functional options.
type config struct {
	workers     int
	queueSize   int
	taskTimeout time.Duration
	panicFn     PanicHandler
}

func defaultConfig() config {
	return config{
		workers:   runtime.NumCPU(),
		queueSize: 100,
	}
}

func (c config) validate() error {
	if c.workers <= 0 {
		return ErrInvalidWorkerCount
	}
	if c.queueSize < 0 {
		return ErrInvalidQueueSize
	}
	if c.taskTimeout < 0 {
		return ErrInvalidTaskTimeout
	}
	return nil
}

// Option is a functional option for configuring a Runner.
type Option func(*config)

// WithWorkers sets the number of concurrent worker goroutines.
func WithWorkers(n int) Option {
	return func(c *config) { c.workers = n }
}

// WithQueueSize sets the task queue channel buffer capacity.
func WithQueueSize(n int) Option {
	return func(c *config) { c.queueSize = n }
}

// WithTaskTimeout sets a maximum execution duration for each individual task.
func WithTaskTimeout(d time.Duration) Option {
	return func(c *config) { c.taskTimeout = d }
}

// WithPanicHandler sets a custom panic handler callback.
func WithPanicHandler(fn PanicHandler) Option {
	return func(c *config) { c.panicFn = fn }
}
