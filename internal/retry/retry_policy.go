package retry

import "context"

type fixedRetryCountPolicy struct {
	retryTimes uint
}

func FixedRetryCountPolicy(retryTimes uint) *fixedRetryCountPolicy {
	return &fixedRetryCountPolicy{retryTimes}
}

func (s *fixedRetryCountPolicy) Calc(ctx context.Context, taskContext *TaskContext) bool {
	return taskContext.RetryNum < s.retryTimes
}
