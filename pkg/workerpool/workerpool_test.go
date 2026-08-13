package workerpool_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gss/pkg/workerpool"
)

func TestNewPool(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		p, err := workerpool.New()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer p.Stop()

		if p.Workers() <= 0 {
			t.Errorf("expected worker count > 0, got %d", p.Workers())
		}
	})

	t.Run("custom options", func(t *testing.T) {
		p, err := workerpool.New(
			workerpool.WithWorkers(4),
			workerpool.WithQueueSize(50),
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer p.Stop()

		if p.Workers() != 4 {
			t.Errorf("expected 4 workers, got %d", p.Workers())
		}
	})

	t.Run("invalid worker count", func(t *testing.T) {
		_, err := workerpool.New(workerpool.WithWorkers(0))
		if err != workerpool.ErrInvalidWorkerCount {
			t.Errorf("expected ErrInvalidWorkerCount, got %v", err)
		}
	})

	t.Run("invalid queue size", func(t *testing.T) {
		_, err := workerpool.New(workerpool.WithQueueSize(-1))
		if err != workerpool.ErrInvalidQueueSize {
			t.Errorf("expected ErrInvalidQueueSize, got %v", err)
		}
	})

	t.Run("invalid task timeout", func(t *testing.T) {
		_, err := workerpool.New(workerpool.WithTaskTimeout(-time.Second))
		if err != workerpool.ErrInvalidTaskTimeout {
			t.Errorf("expected ErrInvalidTaskTimeout, got %v", err)
		}
	})
}

func TestSubmitAndExecute(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(4), workerpool.WithQueueSize(100))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer p.Stop()

	const taskCount = 50
	var counter atomic.Int64

	ctx := context.Background()
	for i := 0; i < taskCount; i++ {
		err := p.Submit(ctx, func(ctx context.Context) error {
			time.Sleep(2 * time.Millisecond)
			counter.Add(1)
			return nil
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", i, err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if counter.Load() != taskCount {
		t.Errorf("expected %d completed tasks, got %d", taskCount, counter.Load())
	}

	if p.Completed() != taskCount {
		t.Errorf("expected stats completed %d, got %d", taskCount, p.Completed())
	}
}

func TestSubmitWithResult(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(2))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer p.Stop()

	ctx := context.Background()

	t.Run("success result", func(t *testing.T) {
		resCh, err := workerpool.SubmitWithResult(p, ctx, func(ctx context.Context) (string, error) {
			return "hello worker pool", nil
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}

		res := <-resCh
		if res.Err != nil {
			t.Fatalf("expected no error, got %v", res.Err)
		}
		if res.Value != "hello worker pool" {
			t.Errorf("expected 'hello worker pool', got %q", res.Value)
		}
	})

	t.Run("error result", func(t *testing.T) {
		expectedErr := errors.New("custom error")
		resCh, err := workerpool.SubmitWithResult(p, ctx, func(ctx context.Context) (int, error) {
			return 0, expectedErr
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}

		res := <-resCh
		if res.Err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, res.Err)
		}
	})
}

func TestPanicRecovery(t *testing.T) {
	var panicHandled atomic.Bool
	var recoveredValue any

	var mu sync.Mutex

	panicHandler := func(r any) {
		mu.Lock()
		panicHandled.Store(true)
		recoveredValue = r
		mu.Unlock()
	}

	p, err := workerpool.New(
		workerpool.WithWorkers(2),
		workerpool.WithPanicHandler(panicHandler),
	)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer p.Stop()

	ctx := context.Background()

	// Submit task that panics
	_ = p.Submit(ctx, func(ctx context.Context) error {
		panic("boom!")
	})

	// Submit normal task afterwards to prove pool is still healthy
	var normalExecuted atomic.Bool
	_ = p.Submit(ctx, func(ctx context.Context) error {
		normalExecuted.Store(true)
		return nil
	})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = p.Shutdown(shutdownCtx)

	if !panicHandled.Load() {
		t.Error("expected panic handler to be invoked")
	}

	mu.Lock()
	if fmt.Sprintf("%v", recoveredValue) != "boom!" {
		t.Errorf("expected recovered panic 'boom!', got %v", recoveredValue)
	}
	mu.Unlock()

	if !normalExecuted.Load() {
		t.Error("expected normal task to execute after panic recovery")
	}

	if p.Failed() != 1 {
		t.Errorf("expected 1 failed task from panic, got %d", p.Failed())
	}
}

func TestTrySubmit(t *testing.T) {
	p, err := workerpool.New(
		workerpool.WithWorkers(1),
		workerpool.WithQueueSize(1),
	)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer p.Stop()

	ctx := context.Background()
	blockCh := make(chan struct{})

	// Task 1: occupies the single worker
	_ = p.Submit(ctx, func(ctx context.Context) error {
		<-blockCh
		return nil
	})

	// Task 2: fills up the queue of capacity 1
	_ = p.Submit(ctx, func(ctx context.Context) error {
		return nil
	})

	// Task 3: should fail with ErrQueueFull
	err = p.TrySubmit(ctx, func(ctx context.Context) error {
		return nil
	})

	if err != workerpool.ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	close(blockCh)
}

func TestSubmitNilTask(t *testing.T) {
	p, err := workerpool.New()
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer p.Stop()

	err = p.Submit(context.Background(), nil)
	if err != workerpool.ErrNilTask {
		t.Errorf("expected ErrNilTask, got %v", err)
	}
}

func TestSubmitRejectsAlreadyCanceledContext(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(1), workerpool.WithQueueSize(1))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := p.Submit(ctx, func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit error = %v, want context.Canceled", err)
	}
	if err := p.TrySubmit(ctx, func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("TrySubmit error = %v, want context.Canceled", err)
	}
	if p.Submitted() != 0 {
		t.Fatalf("Submitted = %d, want 0", p.Submitted())
	}
}

func TestSubmitClosedPool(t *testing.T) {
	p, err := workerpool.New()
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	p.Stop()

	err = p.Submit(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err != workerpool.ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestWithTaskTimeout(t *testing.T) {
	p, err := workerpool.New(
		workerpool.WithWorkers(1),
		workerpool.WithTaskTimeout(20*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer p.Stop()

	errCh := make(chan error, 1)
	_ = p.Submit(context.Background(), func(ctx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond):
			errCh <- nil
		case <-ctx.Done():
			errCh <- ctx.Err()
		}
		return ctx.Err()
	})

	err = <-errCh
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestSubmittingContextCancelsRunningTask(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan error, 1)
	if err := p.Submit(ctx, func(taskCtx context.Context) error {
		close(started)
		<-taskCtx.Done()
		finished <- taskCtx.Err()
		return taskCtx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()

	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("task context error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("submitting context did not cancel running task")
	}
}

func TestStopCancelsTaskAndSkipsQueuedWork(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(1), workerpool.WithQueueSize(1))
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	cancelled := make(chan struct{})
	var queuedRan atomic.Bool
	if err := p.Submit(nil, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := p.Submit(context.Background(), func(context.Context) error {
		queuedRan.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	p.Stop()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the active task")
	}
	if queuedRan.Load() {
		t.Fatal("Stop executed queued work")
	}
}

func TestShutdownUnblocksConcurrentSubmit(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(1), workerpool.WithQueueSize(0))
	if err != nil {
		t.Fatal(err)
	}

	block := make(chan struct{})
	started := make(chan struct{})
	if err := p.Submit(context.Background(), func(context.Context) error {
		close(started)
		<-block
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	<-started

	submitDone := make(chan error, 1)
	go func() {
		submitDone <- p.Submit(context.Background(), func(context.Context) error { return nil })
	}()

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- p.Shutdown(context.Background()) }()

	select {
	case err := <-submitDone:
		if err != workerpool.ErrPoolClosed {
			t.Fatalf("Submit error = %v, want %v", err, workerpool.ErrPoolClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not release blocked Submit")
	}

	close(block)
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not finish")
	}
}

func TestConcurrentSubmitAndStop(t *testing.T) {
	for range 25 {
		p, err := workerpool.New(workerpool.WithWorkers(2), workerpool.WithQueueSize(1))
		if err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		var submitters sync.WaitGroup
		for range 16 {
			submitters.Add(1)
			go func() {
				defer submitters.Done()
				<-start
				_ = p.Submit(context.Background(), func(context.Context) error { return nil })
			}()
		}

		close(start)
		p.Stop()
		submitters.Wait()

		if err := p.Submit(context.Background(), func(context.Context) error { return nil }); err != workerpool.ErrPoolClosed {
			t.Fatalf("Submit after Stop error = %v, want %v", err, workerpool.ErrPoolClosed)
		}
	}
}

func TestSubmitWithResultPanicReturnsResult(t *testing.T) {
	p, err := workerpool.New(workerpool.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	results, err := workerpool.SubmitWithResult(p, context.Background(), func(context.Context) (int, error) {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	result := <-results
	if result.Err == nil || result.Err.Error() != "task panicked: boom" {
		t.Fatalf("unexpected result error: %v", result.Err)
	}
}

func BenchmarkWorkerPoolVsGoroutines(b *testing.B) {
	p, _ := workerpool.New(
		workerpool.WithWorkers(8),
		workerpool.WithQueueSize(1000),
	)
	defer p.Stop()

	ctx := context.Background()

	b.Run("WorkerPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = p.Submit(ctx, func(ctx context.Context) error {
				return nil
			})
		}
	})
}
