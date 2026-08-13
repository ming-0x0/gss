package background

import "context"

// Submit queues a task for execution. It blocks if the queue is full until space
// becomes available or the provided context / runner context is cancelled.
func (r *Runner) Submit(ctx context.Context, task Task) error {
	ctx, env, err := r.prepareSubmit(ctx, task)
	if err != nil {
		return err
	}
	defer r.inflight.Done()

	select {
	case <-r.ctx.Done():
		return ErrClosed
	case <-r.draining:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	case r.queue <- env:
		r.metrics.submitted.Add(1)
		return nil
	}
}

// TrySubmit attempts to queue a task without blocking.
// Returns ErrQueueFull immediately if the queue buffer is full.
func (r *Runner) TrySubmit(ctx context.Context, task Task) error {
	ctx, env, err := r.prepareSubmit(ctx, task)
	if err != nil {
		return err
	}
	defer r.inflight.Done()

	select {
	case <-r.ctx.Done():
		return ErrClosed
	case <-r.draining:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	case r.queue <- env:
		r.metrics.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// prepareSubmit validates inputs and acquires the submission guard.
// Caller MUST call r.inflight.Done() after the returned envelope is sent (or discarded).
func (r *Runner) prepareSubmit(ctx context.Context, task Task) (context.Context, envelope, error) {
	if task == nil {
		return nil, envelope{}, ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, envelope{}, err
	}
	if err := r.acquireSubmit(); err != nil {
		return nil, envelope{}, err
	}
	return ctx, envelope{ctx: ctx, task: task}, nil
}

// acquireSubmit registers an in-flight sender so Shutdown/Stop can wait
// before closing the queue channel.
func (r *Runner) acquireSubmit() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return ErrClosed
	}
	r.inflight.Add(1)
	return nil
}
