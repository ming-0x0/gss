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
	ErrPoolClosed         = errors.New("worker pool is closed")
	ErrQueueFull          = errors.New("worker pool queue is full")
	ErrInvalidWorkerCount = errors.New("worker count must be greater than zero")
	ErrNilTask            = errors.New("task cannot be nil")
)

type Task func(context.Context) error
type PanicHandler func(any)

type Options struct {
	workers     int
	queueSize   int
	taskTimeout time.Duration
	onPanic     PanicHandler
}

type Option func(*Options)

func WithWorkers(n int) Option { return func(o *Options) { o.workers = n } }
func WithQueueSize(n int) Option {
	return func(o *Options) {
		if n >= 0 {
			o.queueSize = n
		}
	}
}
func WithTaskTimeout(d time.Duration) Option { return func(o *Options) { o.taskTimeout = d } }
func WithPanicHandler(h PanicHandler) Option { return func(o *Options) { o.onPanic = h } }

type job struct {
	ctx  context.Context
	task Task
}

type Pool struct {
	workers     int
	tasks       chan job
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	closed      atomic.Bool
	taskTimeout time.Duration
	onPanic     PanicHandler

	activeWorkers atomic.Int64
	submitted     atomic.Int64
	completed     atomic.Int64
	failed        atomic.Int64
}

func New(opts ...Option) (*Pool, error) {
	o := Options{workers: runtime.NumCPU(), queueSize: 100}
	for _, opt := range opts {
		opt(&o)
	}
	if o.workers <= 0 {
		return nil, ErrInvalidWorkerCount
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers: o.workers, tasks: make(chan job, o.queueSize), ctx: ctx, cancel: cancel,
		taskTimeout: o.taskTimeout, onPanic: o.onPanic,
	}
	for range p.workers {
		p.wg.Add(1)
		go p.worker()
	}
	return p, nil
}

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

func (p *Pool) execute(j job) {
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)
	defer func() {
		if r := recover(); r != nil {
			p.failed.Add(1)
			if p.onPanic != nil {
				p.onPanic(r)
			} else {
				fmt.Printf("[workerpool] worker recovered from panic: %v\n", r)
			}
		}
	}()

	ctx := j.ctx
	if ctx == nil {
		ctx = p.ctx
	}
	if p.taskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.taskTimeout)
		defer cancel()
	}
	if err := j.task(ctx); err != nil {
		p.failed.Add(1)
	} else {
		p.completed.Add(1)
	}
}

func (p *Pool) Submit(ctx context.Context, task Task) error {
	if task == nil {
		return ErrNilTask
	}
	if p.closed.Load() {
		return ErrPoolClosed
	}
	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.tasks <- job{ctx, task}:
		p.submitted.Add(1)
		return nil
	}
}

func (p *Pool) TrySubmit(ctx context.Context, task Task) error {
	if task == nil {
		return ErrNilTask
	}
	if p.closed.Load() {
		return ErrPoolClosed
	}
	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.tasks <- job{ctx, task}:
		p.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *Pool) Shutdown(ctx context.Context) error {
	if !p.closed.CompareAndSwap(false, true) {
		return ErrPoolClosed
	}
	close(p.tasks)
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
		p.cancel()
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}

func (p *Pool) Stop() {
	if p.closed.CompareAndSwap(false, true) {
		close(p.tasks)
	}
	p.cancel()
	p.wg.Wait()
}

func (p *Pool) ActiveWorkers() int64 { return p.activeWorkers.Load() }
func (p *Pool) QueueLen() int        { return len(p.tasks) }
func (p *Pool) Submitted() int64     { return p.submitted.Load() }
func (p *Pool) Completed() int64     { return p.completed.Load() }
func (p *Pool) Failed() int64        { return p.failed.Load() }
func (p *Pool) Workers() int         { return p.workers }
