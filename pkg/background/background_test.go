package background_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gss/pkg/background"
)

func TestNewRunner(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		runner, err := background.New()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer runner.Stop()

		if runner.Size() <= 0 {
			t.Errorf("expected worker count > 0, got %d", runner.Size())
		}
	})

	t.Run("custom options", func(t *testing.T) {
		runner, err := background.New(
			background.WithWorkers(4),
			background.WithQueueSize(50),
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer runner.Stop()

		if runner.Size() != 4 {
			t.Errorf("expected 4 workers, got %d", runner.Size())
		}
	})

	t.Run("invalid worker count", func(t *testing.T) {
		_, err := background.New(background.WithWorkers(0))
		if !errors.Is(err, background.ErrInvalidWorkerCount) {
			t.Errorf("expected ErrInvalidWorkerCount, got %v", err)
		}
	})

	t.Run("invalid queue size", func(t *testing.T) {
		_, err := background.New(background.WithQueueSize(-1))
		if !errors.Is(err, background.ErrInvalidQueueSize) {
			t.Errorf("expected ErrInvalidQueueSize, got %v", err)
		}
	})

	t.Run("invalid task timeout", func(t *testing.T) {
		_, err := background.New(background.WithTaskTimeout(-time.Second))
		if !errors.Is(err, background.ErrInvalidTaskTimeout) {
			t.Errorf("expected ErrInvalidTaskTimeout, got %v", err)
		}
	})
}

func TestSubmitAndExecute(t *testing.T) {
	runner, err := background.New(background.WithWorkers(4), background.WithQueueSize(100))
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Stop()

	const taskCount = 50
	var completedCounter atomic.Int64

	parentCtx := context.Background()
	for i := 0; i < taskCount; i++ {
		err := runner.Submit(parentCtx, func(ctx context.Context) error {
			time.Sleep(2 * time.Millisecond)
			completedCounter.Add(1)
			return nil
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", i, err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := runner.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if completedCounter.Load() != taskCount {
		t.Errorf("expected %d completed tasks, got %d", taskCount, completedCounter.Load())
	}

	stats := runner.Stats()
	if stats.Completed != taskCount {
		t.Errorf("expected stats completed %d, got %d", taskCount, stats.Completed)
	}
}

func TestSubmitWithResult(t *testing.T) {
	runner, err := background.New(background.WithWorkers(2))
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Stop()

	parentCtx := context.Background()

	t.Run("success result", func(t *testing.T) {
		resultChan, err := background.SubmitWithResult(runner, parentCtx, func(ctx context.Context) (string, error) {
			return "hello background runner", nil
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}

		res := <-resultChan
		if res.Err != nil {
			t.Fatalf("expected no error, got %v", res.Err)
		}
		if res.Value != "hello background runner" {
			t.Errorf("expected 'hello background runner', got %q", res.Value)
		}
	})

	t.Run("error result", func(t *testing.T) {
		expectedErr := errors.New("custom error")
		resultChan, err := background.SubmitWithResult(runner, parentCtx, func(ctx context.Context) (int, error) {
			return 0, expectedErr
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}

		res := <-resultChan
		if !errors.Is(res.Err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, res.Err)
		}
	})
}

func TestPanicRecovery(t *testing.T) {
	var panicHandled atomic.Bool
	var recoveredValue any
	var mu sync.Mutex

	panicHandler := func(panicVal any) {
		mu.Lock()
		panicHandled.Store(true)
		recoveredValue = panicVal
		mu.Unlock()
	}

	runner, err := background.New(
		background.WithWorkers(2),
		background.WithPanicHandler(panicHandler),
	)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Stop()

	parentCtx := context.Background()

	// Submit task that panics
	_ = runner.Submit(parentCtx, func(ctx context.Context) error {
		panic("boom!")
	})

	// Submit normal task afterwards to prove runner is still healthy
	var normalExecuted atomic.Bool
	_ = runner.Submit(parentCtx, func(ctx context.Context) error {
		normalExecuted.Store(true)
		return nil
	})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = runner.Shutdown(shutdownCtx)

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

	if runner.Stats().Failed != 1 {
		t.Errorf("expected 1 failed task from panic, got %d", runner.Stats().Failed)
	}
}

func TestTrySubmit(t *testing.T) {
	runner, err := background.New(
		background.WithWorkers(1),
		background.WithQueueSize(1),
	)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Stop()

	parentCtx := context.Background()
	blockChan := make(chan struct{})

	// Task 1: occupies the single worker
	_ = runner.Submit(parentCtx, func(ctx context.Context) error {
		<-blockChan
		return nil
	})

	// Task 2: fills up the queue of capacity 1
	_ = runner.Submit(parentCtx, func(ctx context.Context) error {
		return nil
	})

	// Task 3: should fail with ErrQueueFull
	err = runner.TrySubmit(parentCtx, func(ctx context.Context) error {
		return nil
	})

	if !errors.Is(err, background.ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	close(blockChan)
}

func TestSubmitNilTask(t *testing.T) {
	runner, err := background.New()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Stop()

	err = runner.Submit(context.Background(), nil)
	if !errors.Is(err, background.ErrNilTask) {
		t.Errorf("expected ErrNilTask, got %v", err)
	}
}

func TestSubmitRejectsAlreadyCanceledContext(t *testing.T) {
	runner, err := background.New(background.WithWorkers(1), background.WithQueueSize(1))
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Stop()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := runner.Submit(cancelCtx, func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit error = %v, want context.Canceled", err)
	}
	if err := runner.TrySubmit(cancelCtx, func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("TrySubmit error = %v, want context.Canceled", err)
	}
	if runner.Stats().Submitted != 0 {
		t.Fatalf("Submitted = %d, want 0", runner.Stats().Submitted)
	}
}

func TestSubmitClosedRunner(t *testing.T) {
	runner, err := background.New()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	runner.Stop()

	err = runner.Submit(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if !errors.Is(err, background.ErrClosed) {
		t.Errorf("expected ErrClosed, got %v", err)
	}
}

func TestWithTaskTimeout(t *testing.T) {
	runner, err := background.New(
		background.WithWorkers(1),
		background.WithTaskTimeout(20*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Stop()

	errChan := make(chan error, 1)
	_ = runner.Submit(context.Background(), func(ctx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond):
			errChan <- nil
		case <-ctx.Done():
			errChan <- ctx.Err()
		}
		return ctx.Err()
	})

	err = <-errChan
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestSubmittingContextCancelsRunningTask(t *testing.T) {
	runner, err := background.New(background.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Stop()

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startedChan := make(chan struct{})
	finishedChan := make(chan error, 1)

	if err := runner.Submit(parentCtx, func(taskCtx context.Context) error {
		close(startedChan)
		<-taskCtx.Done()
		finishedChan <- taskCtx.Err()
		return taskCtx.Err()
	}); err != nil {
		t.Fatal(err)
	}

	<-startedChan
	cancel()

	select {
	case err := <-finishedChan:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("task context error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("submitting context did not cancel running task")
	}
}

func TestStopCancelsTaskAndSkipsQueuedWork(t *testing.T) {
	runner, err := background.New(background.WithWorkers(1), background.WithQueueSize(1))
	if err != nil {
		t.Fatal(err)
	}

	startedChan := make(chan struct{})
	cancelledChan := make(chan struct{})
	var queuedRan atomic.Bool

	if err := runner.Submit(nil, func(ctx context.Context) error {
		close(startedChan)
		<-ctx.Done()
		close(cancelledChan)
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}

	<-startedChan

	if err := runner.Submit(context.Background(), func(context.Context) error {
		queuedRan.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	runner.Stop()

	select {
	case <-cancelledChan:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the active task")
	}

	if queuedRan.Load() {
		t.Fatal("Stop executed queued work")
	}
}

func TestShutdownUnblocksConcurrentSubmit(t *testing.T) {
	runner, err := background.New(background.WithWorkers(1), background.WithQueueSize(0))
	if err != nil {
		t.Fatal(err)
	}

	blockChan := make(chan struct{})
	startedChan := make(chan struct{})

	if err := runner.Submit(context.Background(), func(context.Context) error {
		close(startedChan)
		<-blockChan
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	<-startedChan

	submitDoneChan := make(chan error, 1)
	go func() {
		submitDoneChan <- runner.Submit(context.Background(), func(context.Context) error { return nil })
	}()

	shutdownDoneChan := make(chan error, 1)
	go func() {
		shutdownDoneChan <- runner.Shutdown(context.Background())
	}()

	select {
	case err := <-submitDoneChan:
		if !errors.Is(err, background.ErrClosed) {
			t.Fatalf("Submit error = %v, want %v", err, background.ErrClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not release blocked Submit")
	}

	close(blockChan)
	select {
	case err := <-shutdownDoneChan:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not finish")
	}
}

func TestConcurrentSubmitAndStop(t *testing.T) {
	for i := 0; i < 25; i++ {
		runner, err := background.New(background.WithWorkers(2), background.WithQueueSize(1))
		if err != nil {
			t.Fatal(err)
		}

		startChan := make(chan struct{})
		var submittersWg sync.WaitGroup

		for j := 0; j < 16; j++ {
			submittersWg.Add(1)
			go func() {
				defer submittersWg.Done()
				<-startChan
				_ = runner.Submit(context.Background(), func(context.Context) error { return nil })
			}()
		}

		close(startChan)
		runner.Stop()
		submittersWg.Wait()

		if err := runner.Submit(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, background.ErrClosed) {
			t.Fatalf("Submit after Stop error = %v, want %v", err, background.ErrClosed)
		}
	}
}

func TestSubmitWithResultPanicReturnsResult(t *testing.T) {
	runner, err := background.New(background.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Stop()

	resultsChan, err := background.SubmitWithResult(runner, context.Background(), func(context.Context) (int, error) {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}

	res := <-resultsChan
	if res.Err == nil || res.Err.Error() != "task panicked: boom" {
		t.Fatalf("unexpected result error: %v", res.Err)
	}
}

func BenchmarkBackgroundVsGoroutines(b *testing.B) {
	runner, _ := background.New(
		background.WithWorkers(8),
		background.WithQueueSize(1000),
	)
	defer runner.Stop()

	parentCtx := context.Background()

	b.Run("BackgroundRunner", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = runner.Submit(parentCtx, func(ctx context.Context) error {
				return nil
			})
		}
	})
}
