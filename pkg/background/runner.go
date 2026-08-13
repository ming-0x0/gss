package background

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents an executable unit of work that accepts a context and returns an error.
type Task func(ctx context.Context) error

// PanicHandler is a callback invoked when a worker goroutine recovers from a task panic.
type PanicHandler func(v any)

// envelope wraps a task together with the caller's context for queuing.
type envelope struct {
	ctx  context.Context
	task Task
}

// Runner manages a fixed set of worker goroutines executing background tasks concurrently.
type Runner struct {
	// Configuration
	workers     int
	queueSize   int
	taskTimeout time.Duration
	panicFn     PanicHandler

	// Core lifecycle
	queue  chan envelope
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	closed atomic.Bool

	// Submission coordination
	mu         sync.RWMutex
	inflight   sync.WaitGroup
	draining   chan struct{}
	closeQueue sync.Once
	done       chan struct{}

	// Metrics
	metrics metrics
}

// New creates, initializes, and starts a new background runner with the provided options.
func New(opts ...Option) (*Runner, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := &Runner{
		workers:     cfg.workers,
		queueSize:   cfg.queueSize,
		taskTimeout: cfg.taskTimeout,
		panicFn:     cfg.panicFn,
		queue:       make(chan envelope, cfg.queueSize),
		ctx:         ctx,
		cancel:      cancel,
		draining:    make(chan struct{}),
		done:        make(chan struct{}),
	}

	r.startWorkers()

	go func() {
		r.wg.Wait()
		close(r.done)
	}()

	return r, nil
}

// startWorkers launches the worker goroutines.
func (r *Runner) startWorkers() {
	for i := 0; i < r.workers; i++ {
		r.wg.Add(1)
		go r.run()
	}
}

// Shutdown gracefully stops the background runner. It rejects new submissions, waits for
// queued tasks to finish, or returns early if ctx is cancelled.
func (r *Runner) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if !r.sealSubmissions() {
		return ErrClosed
	}

	r.drainAndClose()

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		// Graceful timeout → force-cancel active tasks.
		r.cancel()
		return ctx.Err()
	}
}

// Stop immediately cancels all active tasks and stops workers.
func (r *Runner) Stop() {
	r.sealSubmissions()
	r.cancel()
	r.drainAndClose()
	<-r.done
}

// sealSubmissions prevents new submissions and signals blocked senders.
func (r *Runner) sealSubmissions() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.closed.CompareAndSwap(false, true) {
		return false
	}
	close(r.draining)
	return true
}

// drainAndClose waits for in-flight senders to finish, then closes the queue channel.
func (r *Runner) drainAndClose() {
	r.inflight.Wait()
	r.closeQueue.Do(func() {
		close(r.queue)
	})
}

// Size returns the configured number of worker goroutines.
func (r *Runner) Size() int {
	return r.workers
}

// Stats returns a point-in-time snapshot of runner metrics.
func (r *Runner) Stats() Snapshot {
	return Snapshot{
		Active:    r.metrics.active.Load(),
		QueueLen:  len(r.queue),
		Submitted: r.metrics.submitted.Load(),
		Completed: r.metrics.completed.Load(),
		Failed:    r.metrics.failed.Load(),
		Panicked:  r.metrics.panicked.Load(),
	}
}
