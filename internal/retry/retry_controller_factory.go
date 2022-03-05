package retry

import "time"

func ConcurrencySimpleRetryController(retryTimes uint, retryInterval time.Duration, timeLimit time.Duration) RetryController {
	timeOut := (timeLimit+retryInterval)/time.Duration(retryTimes) - retryInterval
	return NewRetryController(
		WithTimeLimit(timeLimit),
		WithTimeoutPolicy(FixedTimeoutPolicy(timeOut)),
		WithRetryIntervalPolicy(FixedRetryIntervalPolicy(retryInterval)),
		WithRetryPolicy(FixedRetryCountPolicy(retryTimes)),
		WithConcurrencyPolicy(AlwaysConcurrencyPolicy()),
	)
}

func SerializationSimpleRetryController(retryTimes uint, retryInterval time.Duration, timeLimit time.Duration) RetryController {
	timeOut := (timeLimit+retryInterval)/time.Duration(retryTimes) - retryInterval
	return NewRetryController(
		WithTimeLimit(timeLimit),
		WithTimeoutPolicy(FixedTimeoutPolicy(timeOut)),
		WithRetryIntervalPolicy(FixedRetryIntervalPolicy(retryInterval)),
		WithRetryPolicy(FixedRetryCountPolicy(retryTimes)),
		WithConcurrencyPolicy(NeverConcurrencyPolicy()),
	)
}
