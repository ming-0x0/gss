package workerpool

import "context"

// Submit queues a task for execution. It blocks if the queue is full until space
// becomes available or the provided context / pool context is cancelled.
func (p *Pool) Submit(ctx context.Context, task Task) error {
	ctx, env, err := p.prepareSubmit(ctx, task)
	if err != nil {
		return err
	}
	defer p.inflight.Done()

	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-p.draining:
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.queue <- env:
		p.metrics.submitted.Add(1)
		return nil
	}
}

// TrySubmit attempts to queue a task without blocking.
// Returns ErrQueueFull immediately if the queue buffer is full.
func (p *Pool) TrySubmit(ctx context.Context, task Task) error {
	ctx, env, err := p.prepareSubmit(ctx, task)
	if err != nil {
		return err
	}
	defer p.inflight.Done()

	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-p.draining:
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.queue <- env:
		p.metrics.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// prepareSubmit validates inputs and acquires the submission guard.
// Caller MUST call p.inflight.Done() after the returned envelope is sent (or discarded).
func (p *Pool) prepareSubmit(ctx context.Context, task Task) (context.Context, envelope, error) {
	if task == nil {
		return nil, envelope{}, ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, envelope{}, err
	}
	if err := p.acquireSubmit(); err != nil {
		return nil, envelope{}, err
	}
	return ctx, envelope{ctx: ctx, task: task}, nil
}

// acquireSubmit registers an in-flight sender so Shutdown/Stop can wait
// before closing the queue channel.
func (p *Pool) acquireSubmit() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed.Load() {
		return ErrPoolClosed
	}
	p.inflight.Add(1)
	return nil
}
