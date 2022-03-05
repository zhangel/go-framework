package retry

import "context"

type alwaysConcurrencyPolicy struct {
}

func AlwaysConcurrencyPolicy() *alwaysConcurrencyPolicy {
	return &alwaysConcurrencyPolicy{}
}

func (s *alwaysConcurrencyPolicy) Calc(ctx context.Context, taskContext *TaskContext) bool {
	return true
}

type neverConcurrencyPolicy struct {
}

func NeverConcurrencyPolicy() *neverConcurrencyPolicy {
	return &neverConcurrencyPolicy{}
}

func (s *neverConcurrencyPolicy) Calc(ctx context.Context, taskContext *TaskContext) bool {
	return false
}
