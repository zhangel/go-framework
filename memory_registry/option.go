package memory_registry

import (
	"google.golang.org/grpc"
)

type Option func(*Options) error

type Options struct {
	aliasNames []string
	unaryInt   grpc.UnaryServerInterceptor
	streamInt  grpc.StreamServerInterceptor
	aliasOnly  bool
}

func WithAliasNames(aliasNames ...string) Option {
	return func(opts *Options) error {
		opts.aliasNames = aliasNames
		return nil
	}
}

func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return func(opts *Options) error {
		opts.unaryInt = interceptor
		return nil
	}
}

func WithStreamInterceptor(interceptor grpc.StreamServerInterceptor) Option {
	return func(opts *Options) error {
		opts.streamInt = interceptor
		return nil
	}
}

func WithAliasOnly(aliasOnly bool) Option {
	return func(opts *Options) error {
		opts.aliasOnly = aliasOnly
		return nil
	}
}

type DialOption func(*DialOptions) error

type DialOptions struct {
	unaryInt  grpc.UnaryClientInterceptor
	streamInt grpc.StreamClientInterceptor
}

func DialWithUnaryInterceptor(interceptor grpc.UnaryClientInterceptor) DialOption {
	return func(opts *DialOptions) error {
		opts.unaryInt = interceptor
		return nil
	}
}

func DialWithStreamInterceptor(interceptor grpc.StreamClientInterceptor) DialOption {
	return func(opts *DialOptions) error {
		opts.streamInt = interceptor
		return nil
	}
}
