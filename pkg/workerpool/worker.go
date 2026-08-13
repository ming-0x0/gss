package workerpool

import (
	"context"
	"fmt"
)

// run is the main loop for each worker goroutine.
// It continuously pulls envelopes from the queue until the pool context
// is cancelled (Stop) or the queue channel is closed (Shutdown drain complete).
func (p *Pool) run() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			// Pool was force-stopped via Stop() → exit immediately.
			return
		case env, ok := <-p.queue:
			if !ok {
				// Queue channel closed after graceful Shutdown drain → no more work.
				return
			}
			// Guard against a race: Stop() may have cancelled p.ctx between
			// the channel receive and here. Avoid starting stale work.
			if p.ctx.Err() != nil {
				return
			}
			p.exec(env)
		}
	}
}

// exec runs a single task with panic recovery and optional timeout.
//
// Lifecycle of a task execution:
//  1. Increment active counter
//  2. Set up panic recovery (deferred)
//  3. Build task context (pool ctx + optional timeout + caller cancellation)
//  4. Invoke task function
//  5. Record success/failure in metrics
func (p *Pool) exec(env envelope) {
	p.metrics.active.Add(1)
	defer p.metrics.active.Add(-1)

	// Panic recovery must be deferred before task invocation so it catches
	// panics from both the task and context setup.
	defer func() {
		if recovered := recover(); recovered != nil {
			p.metrics.failed.Add(1)
			p.metrics.panicked.Add(1)
			p.handlePanic(recovered)
		}
	}()

	taskCtx, taskCancel := p.deriveTaskCtx(env.ctx)
	if taskCancel != nil {
		defer taskCancel()
	}

	if err := env.task(taskCtx); err != nil {
		p.metrics.failed.Add(1)
	} else {
		p.metrics.completed.Add(1)
	}
}

// handlePanic invokes the user-defined panic handler, or falls back to printing.
// It also guards against panics inside the custom handler itself to prevent
// crashing the worker goroutine.
func (p *Pool) handlePanic(recovered any) {
	if p.panicFn == nil {
		fmt.Printf("[workerpool] worker recovered from panic: %v\n", recovered)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[workerpool] panic handler panicked: %v\n", r)
		}
	}()

	p.panicFn(recovered)
}

// deriveTaskCtx constructs the execution context for a single task by merging
// three cancellation signals:
//
//	┌─────────────┐
//	│   p.ctx      │ ← pool lifecycle (cancelled on Stop)
//	│  (parent)    │
//	└──────┬──────┘
//	       │
//	       ▼
//	┌─────────────┐
//	│  taskCtx     │ ← WithCancel or WithTimeout (if taskTimeout > 0)
//	└──────┬──────┘
//	       │
//	       │  AfterFunc
//	       ◄────────── callerCtx (e.g. HTTP request context)
//
// The resulting taskCtx is cancelled when ANY of these fires:
//   - Pool is stopped (p.ctx cancelled)
//   - Task timeout expires (if configured)
//   - Caller's context is cancelled (e.g. client disconnects)
func (p *Pool) deriveTaskCtx(callerCtx context.Context) (context.Context, context.CancelFunc) {
	if callerCtx == nil {
		callerCtx = context.Background()
	}

	// Derive directly from the appropriate context constructor to avoid
	// creating an unnecessary intermediate WithCancel when timeout is set.
	var (
		taskCtx    context.Context
		taskCancel context.CancelFunc
	)
	if p.taskTimeout > 0 {
		// WithTimeout already creates a cancellable child of p.ctx.
		taskCtx, taskCancel = context.WithTimeout(p.ctx, p.taskTimeout)
	} else {
		taskCtx, taskCancel = context.WithCancel(p.ctx)
	}

	// If the caller's context is cancellable (e.g. HTTP request with deadline),
	// propagate its cancellation into taskCtx via AfterFunc.
	// This ensures the task stops if the caller goes away.
	if callerCtx.Done() != nil {
		unregister := context.AfterFunc(callerCtx, func() { taskCancel() })
		baseCancel := taskCancel
		taskCancel = func() {
			unregister()  // Remove the AfterFunc callback to avoid leaking it.
			baseCancel()  // Cancel the task context itself.
		}
	}

	return taskCtx, taskCancel
}

