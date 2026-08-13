package workerpool

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrPoolClosed is returned when attempting to submit a task to a closed pool.
	ErrPoolClosed = errors.New("worker pool is closed")

	// ErrQueueFull is returned when the task queue is full during non-blocking submit.
	ErrQueueFull = errors.New("worker pool queue is full")

	// ErrInvalidWorkerCount is returned when worker count is invalid.
	ErrInvalidWorkerCount = errors.New("worker count must be greater than zero")

	// ErrInvalidQueueSize is returned when queue size is negative.
	ErrInvalidQueueSize = errors.New("queue size cannot be negative")

	// ErrInvalidTaskTimeout is returned when task timeout is negative.
	ErrInvalidTaskTimeout = errors.New("task timeout cannot be negative")

	// ErrNilTask is returned when attempting to submit a nil task.
	ErrNilTask = errors.New("task cannot be nil")
)

// Task represents a unit of work to be executed by a worker.
type Task func(ctx context.Context) error

// PanicHandler handles panics that occur inside worker tasks.
type PanicHandler func(r any)

// Options holds configuration for Pool.
type Options struct {
	workers     int
	queueSize   int
	taskTimeout time.Duration
	onPanic     PanicHandler
}

// Option modifies Options.
type Option func(*Options)

// WithWorkers sets the number of concurrent worker goroutines.
func WithWorkers(n int) Option {
	return func(o *Options) { o.workers = n }
}

// WithQueueSize sets the task queue channel buffer capacity.
func WithQueueSize(size int) Option {
	return func(o *Options) { o.queueSize = size }
}

// WithTaskTimeout sets a maximum execution timeout for tasks.
func WithTaskTimeout(d time.Duration) Option {
	return func(o *Options) { o.taskTimeout = d }
}

// WithPanicHandler sets a custom panic handler function.
func WithPanicHandler(handler PanicHandler) Option {
	return func(o *Options) { o.onPanic = handler }
}

type job struct {
	ctx  context.Context
	task Task
}

// Pool manages a pool of worker goroutines executing tasks concurrently.
type Pool struct {
	workers     int
	tasks       chan job
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	closed      atomic.Bool
	taskTimeout time.Duration
	onPanic     PanicHandler

	// Submission is coordinated separately from execution so a task channel
	// can never be closed while a sender is using it.
	submitMu   sync.Mutex
	submitters sync.WaitGroup
	closing    chan struct{}
	closeTasks sync.Once
	done       chan struct{}

	// Metrics
	activeWorkers atomic.Int64
	submitted     atomic.Int64
	completed     atomic.Int64
	failed        atomic.Int64
}

// New creates and starts a new worker pool.
func New(opts ...Option) (*Pool, error) {
	options := Options{workers: runtime.NumCPU(), queueSize: 100}
	for _, opt := range opts {
		opt(&options)
	}

	if options.workers <= 0 {
		return nil, ErrInvalidWorkerCount
	}
	if options.queueSize < 0 {
		return nil, ErrInvalidQueueSize
	}
	if options.taskTimeout < 0 {
		return nil, ErrInvalidTaskTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers:     options.workers,
		tasks:       make(chan job, options.queueSize),
		ctx:         ctx,
		cancel:      cancel,
		taskTimeout: options.taskTimeout,
		onPanic:     options.onPanic,
		closing:     make(chan struct{}),
		done:        make(chan struct{}),
	}

	p.start()
	go func() {
		p.wg.Wait()
		close(p.done)
	}()
	return p, nil
}

// start launches the worker goroutines.
func (p *Pool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker is the main loop for each worker goroutine.
func (p *Pool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case j, ok := <-p.tasks:
			if !ok {
				return
			}
			// If Stop races with a receive, do not start another queued task.
			if p.ctx.Err() != nil {
				return
			}
			p.execute(j)
		}
	}
}

// execute runs a single task with panic protection and timeout handling.
func (p *Pool) execute(j job) {
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	defer func() {
		if r := recover(); r != nil {
			p.failed.Add(1)
			p.reportPanic(r)
		}
	}()

	taskCtx, cancel := p.taskContext(j.ctx)
	defer cancel()
	if p.taskTimeout > 0 {
		var timeoutCancel context.CancelFunc
		taskCtx, timeoutCancel = context.WithTimeout(taskCtx, p.taskTimeout)
		defer timeoutCancel()
	}

	if err := j.task(taskCtx); err != nil {
		p.failed.Add(1)
	} else {
		p.completed.Add(1)
	}
}

func (p *Pool) reportPanic(r any) {
	if p.onPanic == nil {
		fmt.Printf("[workerpool] worker recovered from panic: %v\n", r)
		return
	}

	// A faulty observer must not terminate a worker goroutine.
	defer func() {
		if handlerPanic := recover(); handlerPanic != nil {
			fmt.Printf("[workerpool] panic handler panicked: %v\n", handlerPanic)
		}
	}()
	p.onPanic(r)
}

// taskContext is cancelled when either the submitting context is cancelled or
// Stop is called. The pool context is the direct parent so Stop propagates
// cancellation through the context tree without scheduling a callback.
func (p *Pool) taskContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	taskCtx, cancel := context.WithCancel(p.ctx)
	if parent.Done() == nil {
		return taskCtx, cancel
	}
	stopParentCancel := context.AfterFunc(parent, cancel)
	return taskCtx, func() {
		stopParentCancel()
		cancel()
	}
}

// beginSubmit registers a sender before Shutdown or Stop can close the queue.
func (p *Pool) beginSubmit() error {
	p.submitMu.Lock()
	defer p.submitMu.Unlock()
	if p.closed.Load() {
		return ErrPoolClosed
	}
	p.submitters.Add(1)
	return nil
}

// closeSubmissions prevents future submissions and releases blocked ones.
func (p *Pool) closeSubmissions() bool {
	p.submitMu.Lock()
	defer p.submitMu.Unlock()
	if !p.closed.CompareAndSwap(false, true) {
		return false
	}
	close(p.closing)
	return true
}

func (p *Pool) closeTaskQueue() {
	p.submitters.Wait()
	p.closeTasks.Do(func() { close(p.tasks) })
}

// Submit queues a task for execution. It blocks if the task queue is full until space is available
// or the provided context / pool context is cancelled.
func (p *Pool) Submit(ctx context.Context, task Task) error {
	if task == nil {
		return ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.beginSubmit(); err != nil {
		return err
	}
	defer p.submitters.Done()

	j := job{ctx: ctx, task: task}
	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-p.closing:
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.tasks <- j:
		p.submitted.Add(1)
		return nil
	}
}

// TrySubmit attempts to queue a task for execution without blocking.
// Returns ErrQueueFull if the task queue buffer is full.
func (p *Pool) TrySubmit(ctx context.Context, task Task) error {
	if task == nil {
		return ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.beginSubmit(); err != nil {
		return err
	}
	defer p.submitters.Done()

	j := job{ctx: ctx, task: task}
	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-p.closing:
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.tasks <- j:
		p.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown gracefully shuts down the worker pool. It stops accepting new tasks and
// waits for queued tasks to finish execution or until the provided context is cancelled.
func (p *Pool) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !p.closeSubmissions() {
		return ErrPoolClosed
	}
	p.closeTaskQueue()

	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		// Timed-out graceful shutdown becomes a stop: active task contexts are
		// cancelled and queued tasks are not executed.
		p.cancel()
		return ctx.Err()
	}
}

// Stop stops workers and cancels active task contexts. Tasks must honor their
// context cancellation for Stop to return promptly.
func (p *Pool) Stop() {
	p.closeSubmissions()
	p.cancel()
	p.closeTaskQueue()
	<-p.done
}

// ActiveWorkers returns the current number of workers executing tasks.
func (p *Pool) ActiveWorkers() int64 { return p.activeWorkers.Load() }

// QueueLen returns the current number of tasks waiting in the queue.
func (p *Pool) QueueLen() int { return len(p.tasks) }

// Submitted returns the total number of tasks submitted to the pool.
func (p *Pool) Submitted() int64 { return p.submitted.Load() }

// Completed returns the total number of tasks successfully completed.
func (p *Pool) Completed() int64 { return p.completed.Load() }

// Failed returns the total number of tasks that returned an error or panicked.
func (p *Pool) Failed() int64 { return p.failed.Load() }

// Workers returns the number of worker goroutines configured in the pool.
func (p *Pool) Workers() int { return p.workers }
