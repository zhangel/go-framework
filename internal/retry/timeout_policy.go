package retry

import (
	"context"
	"time"
)

type fixedTimeoutTimePolicy struct {
	timeout time.Duration
}

func FixedTimeoutPolicy(timeout time.Duration) TimePolicy {
	return &fixedTimeoutTimePolicy{timeout}
}

func (s *fixedTimeoutTimePolicy) Calc(ctx context.Context, taskContext *TaskContext) time.Duration {
	return s.timeout
}
