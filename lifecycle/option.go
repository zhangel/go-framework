package lifecycle

import (
	"time"
)

type Options struct {
	name     string
	priority int32
	timeout  time.Duration
}

type Option func(*Options)

const (
	PriorityDefault = 0
)

func WithName(name string) Option {
	return func(opts *Options) {
		opts.name = name
	}
}

func generateOptions(opt ...Option) *Options {
	opts := &Options{
		priority: PriorityDefault,
		timeout:  10 * time.Second,
	}
	for _, o := range opt {
		o(opts)
	}
	return opts
}
