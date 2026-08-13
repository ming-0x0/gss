package workerpool

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

// Pool manages a fixed set of worker goroutines executing tasks concurrently.
type Pool struct {
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

// New creates, initializes, and starts a new worker pool with the provided options.
func New(opts ...Option) (*Pool, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
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

	p.startWorkers()

	go func() {
		p.wg.Wait()
		close(p.done)
	}()

	return p, nil
}

// startWorkers launches the worker goroutines.
func (p *Pool) startWorkers() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.run()
	}
}

// Shutdown gracefully stops the pool. It rejects new submissions, waits for
// queued tasks to finish, or returns early if ctx is cancelled.
func (p *Pool) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if !p.sealSubmissions() {
		return ErrPoolClosed
	}

	p.drainAndClose()

	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		// Graceful timeout → force-cancel active tasks.
		p.cancel()
		return ctx.Err()
	}
}

// Stop immediately cancels all active tasks and stops workers.
func (p *Pool) Stop() {
	p.sealSubmissions()
	p.cancel()
	p.drainAndClose()
	<-p.done
}

// sealSubmissions prevents new submissions and signals blocked senders.
func (p *Pool) sealSubmissions() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.closed.CompareAndSwap(false, true) {
		return false
	}
	close(p.draining)
	return true
}

// drainAndClose waits for in-flight senders to finish, then closes the queue channel.
func (p *Pool) drainAndClose() {
	p.inflight.Wait()
	p.closeQueue.Do(func() {
		close(p.queue)
	})
}

// Size returns the configured number of worker goroutines.
func (p *Pool) Size() int {
	return p.workers
}

// Stats returns a point-in-time snapshot of pool metrics.
func (p *Pool) Stats() Snapshot {
	return Snapshot{
		Active:    p.metrics.active.Load(),
		QueueLen:  len(p.queue),
		Submitted: p.metrics.submitted.Load(),
		Completed: p.metrics.completed.Load(),
		Failed:    p.metrics.failed.Load(),
		Panicked:  p.metrics.panicked.Load(),
	}
}
