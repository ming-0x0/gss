package background

import "errors"

// Sentinel errors returned by background runner operations.
var (
	// ErrClosed occurs when attempting to submit a task to a runner that has been closed.
	ErrClosed = errors.New("background runner is closed")

	// ErrQueueFull occurs when attempting to submit a task via TrySubmit to a full queue.
	ErrQueueFull = errors.New("background task queue is full")

	// ErrInvalidWorkerCount occurs when setting worker count to zero or a negative number.
	ErrInvalidWorkerCount = errors.New("worker count must be greater than zero")

	// ErrInvalidQueueSize occurs when setting queue size to a negative number.
	ErrInvalidQueueSize = errors.New("queue size cannot be negative")

	// ErrInvalidTaskTimeout occurs when setting task timeout to a negative duration.
	ErrInvalidTaskTimeout = errors.New("task timeout cannot be negative")

	// ErrNilTask occurs when attempting to submit a nil task function.
	ErrNilTask = errors.New("task cannot be nil")
)
