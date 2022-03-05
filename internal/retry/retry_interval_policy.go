package retry

import (
	"context"
	"time"
)

type fixedRetryIntervalPolicy struct {
	interval time.Duration
}

func FixedRetryIntervalPolicy(interval time.Duration) TimePolicy {
	return &fixedRetryIntervalPolicy{interval}
}

func (s *fixedRetryIntervalPolicy) Calc(ctx context.Context, taskContext *TaskContext) time.Duration {
	if taskContext.RetryNum == 0 {
		return 0
	} else {
		return s.interval
	}
}
