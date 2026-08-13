package workerpool

import (
	"context"
)

// Result holds the output or error from a generic task execution.
type Result[T any] struct {
	Value T
	Err   error
}

// SubmitWithResult submits a function that returns (T, error) to the worker pool.
// It returns a buffered channel of capacity 1 which receives a Result[T] upon completion.
func SubmitWithResult[T any](
	p *Pool,
	ctx context.Context,
	fn func(ctx context.Context) (T, error),
) (<-chan Result[T], error) {
	if fn == nil {
		return nil, ErrNilTask
	}

	resultCh := make(chan Result[T], 1)

	task := func(taskCtx context.Context) error {
		val, err := fn(taskCtx)
		resultCh <- Result[T]{
			Value: val,
			Err:   err,
		}
		close(resultCh)
		return err
	}

	if err := p.Submit(ctx, task); err != nil {
		return nil, err
	}

	return resultCh, nil
}
