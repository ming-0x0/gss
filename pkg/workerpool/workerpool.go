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

	// ErrNilTask is returned when attempting to submit a nil task.
	ErrNilTask = errors.New("task cannot be nil")
)

// Task represents a unit of work to be executed by a worker.
type Task func(ctx context.Context) error

// PanicHandler handles panics that occur inside worker tasks.
type PanicHandler func(r any)

// Options holds configuration for Pool.
type Options struct {
	workers      int
	queueSize    int
	taskTimeout  time.Duration
	panicHandler PanicHandler
}

// Option modifies Options.
type Option func(*Options)

// WithWorkers sets the number of concurrent worker goroutines.
func WithWorkers(n int) Option {
	return func(o *Options) {
		o.workers = n
	}
}

// WithQueueSize sets the task queue channel buffer capacity.
func WithQueueSize(size int) Option {
	return func(o *Options) {
		if size >= 0 {
			o.queueSize = size
		}
	}
}

// WithTaskTimeout sets a maximum execution timeout for tasks.
func WithTaskTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.taskTimeout = d
	}
}

// WithPanicHandler sets a custom panic handler function.
func WithPanicHandler(handler PanicHandler) Option {
	return func(o *Options) {
		o.panicHandler = handler
	}
}

type job struct {
	ctx  context.Context
	task Task
}

// Pool manages a pool of worker goroutines executing tasks concurrently.
type Pool struct {
	workers      int
	tasks        chan job
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	closed       atomic.Bool
	taskTimeout  time.Duration
	panicHandler PanicHandler

	// Metrics
	activeWorkers atomic.Int64
	submitted     atomic.Int64
	completed     atomic.Int64
	failed        atomic.Int64
}

// New creates and starts a new worker pool.
func New(opts ...Option) (*Pool, error) {
	options := Options{
		workers:   runtime.NumCPU(),
		queueSize: 100,
	}

	for _, opt := range opts {
		opt(&options)
	}

	if options.workers <= 0 {
		return nil, ErrInvalidWorkerCount
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		workers:      options.workers,
		tasks:        make(chan job, options.queueSize),
		ctx:          ctx,
		cancel:       cancel,
		taskTimeout:  options.taskTimeout,
		panicHandler: options.panicHandler,
	}

	p.start()
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
			p.drainTasks()
			return
		case j, ok := <-p.tasks:
			if !ok {
				return
			}
			p.execute(j)
		}
	}
}

// drainTasks executes remaining tasks in the queue when worker pool context is cancelled.
func (p *Pool) drainTasks() {
	for {
		select {
		case j, ok := <-p.tasks:
			if !ok {
				return
			}
			p.execute(j)
		default:
			return
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
			if p.panicHandler != nil {
				p.panicHandler(r)
			} else {
				fmt.Printf("[workerpool] worker recovered from panic: %v\n", r)
			}
		}
	}()

	taskCtx := j.ctx
	if taskCtx == nil {
		taskCtx = p.ctx
	}

	if p.taskTimeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(taskCtx, p.taskTimeout)
		defer cancel()
	}

	if err := j.task(taskCtx); err != nil {
		p.failed.Add(1)
	} else {
		p.completed.Add(1)
	}
}

// Submit queues a task for execution. It blocks if the task queue is full until space is available
// or the provided context / pool context is cancelled.
func (p *Pool) Submit(ctx context.Context, task Task) error {
	if task == nil {
		return ErrNilTask
	}
	if p.closed.Load() {
		return ErrPoolClosed
	}

	j := job{ctx: ctx, task: task}

	select {
	case <-p.ctx.Done():
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
	if p.closed.Load() {
		return ErrPoolClosed
	}

	j := job{ctx: ctx, task: task}

	select {
	case <-p.ctx.Done():
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
	if !p.closed.CompareAndSwap(false, true) {
		return ErrPoolClosed
	}

	close(p.tasks)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.cancel()
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}

// Stop immediately stops all workers, canceling active task contexts.
func (p *Pool) Stop() {
	if p.closed.CompareAndSwap(false, true) {
		close(p.tasks)
	}
	p.cancel()
	p.wg.Wait()
}

// ActiveWorkers returns the current number of workers executing tasks.
func (p *Pool) ActiveWorkers() int64 {
	return p.activeWorkers.Load()
}

// QueueLen returns the current number of tasks waiting in the queue.
func (p *Pool) QueueLen() int {
	return len(p.tasks)
}

// Submitted returns the total number of tasks submitted to the pool.
func (p *Pool) Submitted() int64 {
	return p.submitted.Load()
}

// Completed returns the total number of tasks successfully completed.
func (p *Pool) Completed() int64 {
	return p.completed.Load()
}

// Failed returns the total number of tasks that returned an error or panicked.
func (p *Pool) Failed() int64 {
	return p.failed.Load()
}

// Workers returns the number of worker goroutines configured in the pool.
func (p *Pool) Workers() int {
	return p.workers
}
