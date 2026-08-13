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

// List of package level errors returned by Worker Pool operations.
var (
	// ErrPoolClosed occurs when attempting to submit a task to a worker pool that has been closed.
	ErrPoolClosed = errors.New("worker pool is closed")

	// ErrQueueFull occurs when attempting to submit a task via TrySubmit to a full queue.
	ErrQueueFull = errors.New("worker pool queue is full")

	// ErrInvalidWorkerCount occurs when setting worker count to zero or a negative number.
	ErrInvalidWorkerCount = errors.New("worker count must be greater than zero")

	// ErrInvalidQueueSize occurs when setting queue size to a negative number.
	ErrInvalidQueueSize = errors.New("queue size cannot be negative")

	// ErrInvalidTaskTimeout occurs when setting task execution timeout to a negative duration.
	ErrInvalidTaskTimeout = errors.New("task timeout cannot be negative")

	// ErrNilTask occurs when attempting to submit a nil task function.
	ErrNilTask = errors.New("task cannot be nil")
)

// Task represents an executable unit of work that accepts a context and returns an error.
type Task func(ctx context.Context) error

// PanicHandler is a callback function invoked when a worker goroutine recovers from a task panic.
type PanicHandler func(panicValue any)

// Options holds configuration options for initializing a Pool instance.
type Options struct {
	workers     int
	queueSize   int
	taskTimeout time.Duration
	onPanic     PanicHandler
}

// Option represents a functional option for configuring Pool settings.
type Option func(options *Options)

// WithWorkers sets the number of concurrent worker goroutines.
func WithWorkers(workerCount int) Option {
	return func(options *Options) {
		options.workers = workerCount
	}
}

// WithQueueSize sets the task queue channel buffer capacity.
func WithQueueSize(queueCapacity int) Option {
	return func(options *Options) {
		options.queueSize = queueCapacity
	}
}

// WithTaskTimeout sets a maximum execution timeout for each individual task.
func WithTaskTimeout(duration time.Duration) Option {
	return func(options *Options) {
		options.taskTimeout = duration
	}
}

// WithPanicHandler sets a custom panic handler callback function.
func WithPanicHandler(handler PanicHandler) Option {
	return func(options *Options) {
		options.onPanic = handler
	}
}

// job encapsulates a task along with its associated parent context.
type job struct {
	ctx  context.Context
	task Task
}

// Pool manages a fixed pool of worker goroutines executing tasks concurrently.
type Pool struct {
	// Configuration & Options
	workers     int
	queueSize   int
	taskTimeout time.Duration
	onPanic     PanicHandler

	// Core Channels & Lifecycle Management
	tasks  chan job
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	closed atomic.Bool

	// Submission Synchronization & Coordination
	submitMu   sync.RWMutex
	submitters sync.WaitGroup
	closing    chan struct{}
	closeTasks sync.Once
	done       chan struct{}

	// Realtime Operational Metrics
	activeWorkers atomic.Int64
	submitted     atomic.Int64
	completed     atomic.Int64
	failed        atomic.Int64
	panics        atomic.Int64
}

// New creates, initializes, and starts a new worker pool with the provided options.
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
	if options.queueSize < 0 {
		return nil, ErrInvalidQueueSize
	}
	if options.taskTimeout < 0 {
		return nil, ErrInvalidTaskTimeout
	}

	parentCtx, cancelFunc := context.WithCancel(context.Background())

	pool := &Pool{
		workers:     options.workers,
		queueSize:   options.queueSize,
		tasks:       make(chan job, options.queueSize),
		ctx:         parentCtx,
		cancel:      cancelFunc,
		taskTimeout: options.taskTimeout,
		onPanic:     options.onPanic,
		closing:     make(chan struct{}),
		done:        make(chan struct{}),
	}

	pool.start()

	go func() {
		pool.wg.Wait()
		close(pool.done)
	}()

	return pool, nil
}

// Submit queues a task for execution. It blocks if the task queue is full until space becomes available
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

	pendingJob := job{ctx: ctx, task: task}

	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-p.closing:
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.tasks <- pendingJob:
		p.submitted.Add(1)
		return nil
	}
}

// TrySubmit attempts to queue a task for execution without blocking.
// Returns ErrQueueFull immediately if the task queue buffer is full.
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

	pendingJob := job{ctx: ctx, task: task}

	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-p.closing:
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.tasks <- pendingJob:
		p.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown gracefully shuts down the worker pool. It stops accepting new tasks and
// waits for all queued tasks to finish execution or until the provided context is cancelled.
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
		// Timed-out graceful shutdown converts into an immediate stop, canceling active tasks.
		p.cancel()
		return ctx.Err()
	}
}

// Stop immediately stops all worker goroutines and cancels active task contexts.
func (p *Pool) Stop() {
	p.closeSubmissions()
	p.cancel()
	p.closeTaskQueue()
	<-p.done
}

// start launches the worker goroutines.
func (p *Pool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker is the main processing loop for each worker goroutine.
func (p *Pool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case targetJob, ok := <-p.tasks:
			if !ok {
				return
			}
			// If Stop races with a receive, do not start another queued task.
			if p.ctx.Err() != nil {
				return
			}
			p.execute(targetJob)
		}
	}
}

// execute runs a single job with panic recovery and timeout handling.
func (p *Pool) execute(targetJob job) {
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	defer func() {
		if panicVal := recover(); panicVal != nil {
			p.failed.Add(1)
			p.panics.Add(1)
			p.reportPanic(panicVal)
		}
	}()

	taskCtx, cancelFunc := p.taskContext(targetJob.ctx)
	if cancelFunc != nil {
		defer cancelFunc()
	}

	if err := targetJob.task(taskCtx); err != nil {
		p.failed.Add(1)
	} else {
		p.completed.Add(1)
	}
}

// reportPanic safely invokes the user-defined panic handler or prints a fallback message.
func (p *Pool) reportPanic(panicValue any) {
	if p.onPanic == nil {
		fmt.Printf("[workerpool] worker recovered from panic: %v\n", panicValue)
		return
	}

	// Protect worker goroutines against panics inside custom panic handlers.
	defer func() {
		if handlerPanic := recover(); handlerPanic != nil {
			fmt.Printf("[workerpool] panic handler panicked: %v\n", handlerPanic)
		}
	}()

	p.onPanic(panicValue)
}

// taskContext constructs a derived task context, attaching timeout and parent cancellation signals.
func (p *Pool) taskContext(parentContext context.Context) (context.Context, context.CancelFunc) {
	if parentContext == nil {
		parentContext = context.Background()
	}

	taskCtx, cancelFunc := context.WithCancel(p.ctx)

	if p.taskTimeout > 0 {
		var timeoutCancel context.CancelFunc
		taskCtx, timeoutCancel = context.WithTimeout(taskCtx, p.taskTimeout)
		baseCancel := cancelFunc
		cancelFunc = func() {
			timeoutCancel()
			baseCancel()
		}
	}

	if parentContext.Done() == nil {
		return taskCtx, cancelFunc
	}

	stopParentCancel := context.AfterFunc(parentContext, cancelFunc)
	return taskCtx, func() {
		stopParentCancel()
		cancelFunc()
	}
}

// beginSubmit registers an active sender before Shutdown or Stop can close the task queue.
func (p *Pool) beginSubmit() error {
	p.submitMu.RLock()
	if p.closed.Load() {
		p.submitMu.RUnlock()
		return ErrPoolClosed
	}
	p.submitters.Add(1)
	p.submitMu.RUnlock()
	return nil
}

// closeSubmissions prevents new task submissions and signals active blocked senders.
func (p *Pool) closeSubmissions() bool {
	p.submitMu.Lock()
	defer p.submitMu.Unlock()

	if !p.closed.CompareAndSwap(false, true) {
		return false
	}
	close(p.closing)
	return true
}

// closeTaskQueue waits for all active submitters to finish sending before closing the tasks channel.
func (p *Pool) closeTaskQueue() {
	p.submitters.Wait()
	p.closeTasks.Do(func() {
		close(p.tasks)
	})
}

// Workers returns the configured number of worker goroutines.
func (p *Pool) Workers() int {
	return p.workers
}

// ActiveWorkers returns the current number of worker goroutines actively executing tasks.
func (p *Pool) ActiveWorkers() int64 {
	return p.activeWorkers.Load()
}

// QueueLen returns the current number of tasks waiting in the queue buffer.
func (p *Pool) QueueLen() int {
	return len(p.tasks)
}

// Submitted returns the total number of tasks submitted to the pool.
func (p *Pool) Submitted() int64 {
	return p.submitted.Load()
}

// Completed returns the total number of tasks successfully executed.
func (p *Pool) Completed() int64 {
	return p.completed.Load()
}

// Failed returns the total number of tasks that returned an error or panicked.
func (p *Pool) Failed() int64 {
	return p.failed.Load()
}

// Panics returns the total number of task panics recovered by the pool.
func (p *Pool) Panics() int64 {
	return p.panics.Load()
}
