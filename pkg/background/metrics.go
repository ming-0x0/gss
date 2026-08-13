package background

import "sync/atomic"

// metrics tracks operational counters using lock-free atomics.
type metrics struct {
	active    atomic.Int64
	submitted atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
	panicked  atomic.Int64
}

// Snapshot is a point-in-time copy of background runner metrics.
type Snapshot struct {
	Active    int64
	QueueLen  int
	Submitted int64
	Completed int64
	Failed    int64
	Panicked  int64
}
