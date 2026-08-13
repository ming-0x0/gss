package workerpool

import (
	"context"
	"fmt"
)

// Result holds the return value or error from executing a generic task function.
type Result[T any] struct {
	Value T
	Err   error
}

// SubmitWithResult submits a function returning (T, error) to the worker pool for execution.
// It returns a buffered channel of capacity 1 that delivers the Result[T] upon completion.
func SubmitWithResult[T any](
	pool *Pool,
	ctx context.Context,
	taskFunc func(ctx context.Context) (T, error),
) (<-chan Result[T], error) {
	if taskFunc == nil {
		return nil, ErrNilTask
	}

	resultChan := make(chan Result[T], 1)

	wrappedTask := func(taskCtx context.Context) error {
		defer func() {
			if panicVal := recover(); panicVal != nil {
				resultChan <- Result[T]{
					Err: fmt.Errorf("task panicked: %v", panicVal),
				}
				close(resultChan)
				// Re-panic so Pool records the failure metrics and invokes its panic handler.
				panic(panicVal)
			}
		}()

		resultVal, err := taskFunc(taskCtx)
		resultChan <- Result[T]{
			Value: resultVal,
			Err:   err,
		}
		close(resultChan)
		return err
	}

	if err := pool.Submit(ctx, wrappedTask); err != nil {
		return nil, err
	}

	return resultChan, nil
}
