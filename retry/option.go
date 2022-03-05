package retry

import "time"

type RetriablePredict interface {
	IsRetriable(err error) bool
}
type RetriablePredictFunc func(err error) bool

func (predict RetriablePredictFunc) IsRetriable(err error) bool {
	return predict(err)
}

type Options struct {
	numberOfRetries    uint
	perTryTimeout      time.Duration
	overlap            bool
	backoffDuration    time.Duration
	backoffJitter      float64
	retriablePredict   RetriablePredict
	invokerName        string
	backoffExponential bool
}

type Option func(*Options) error

func WithRetry(numberOfRetries uint, perTryTimeout time.Duration, retriablePredict RetriablePredict) Option {
	return func(opts *Options) error {
		opts.numberOfRetries = numberOfRetries
		opts.perTryTimeout = perTryTimeout
		opts.retriablePredict = retriablePredict
		return nil
	}
}

func WithOverlap(overlap bool) Option {
	return func(opts *Options) error {
		opts.overlap = overlap
		return nil
	}
}

func WithBackoffDuration(backoffDuration time.Duration) Option {
	return func(opts *Options) error {
		opts.backoffDuration = backoffDuration
		return nil
	}
}

func WithBackoffJitter(backoffJitter float64) Option {
	return func(opts *Options) error {
		opts.backoffJitter = backoffJitter
		return nil
	}
}

func WithBackoffExponential(backoffExponential bool) Option {
	return func(opts *Options) error {
		opts.backoffExponential = backoffExponential
		return nil
	}
}

func WithInvokerName(name string) Option {
	return func(opts *Options) error {
		opts.invokerName = name
		return nil
	}
}
