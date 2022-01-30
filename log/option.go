package log

import (
	"github.com/zhangel/go-framework/log/encoder"
	"github.com/zhangel/go-framework/log/level"
	"github.com/zhangel/go-framework/log/logger"
	"github.com/zhangel/go-framework/log/writer"
)

type Options struct {
	encoder    encoder.Encoder
	writer     writer.Writer
	minLevelOp func(logger.Logger) logger.Logger
}

type Option func(*Options) error

func WithEncoder(encoder encoder.Encoder) Option {
	return func(opts *Options) error {
		opts.encoder = encoder
		return nil
	}
}

func WithWriter(writer writer.Writer) Option {
	return func(opts *Options) error {
		opts.writer = writer
		return nil
	}
}

func WithMinLevel(minLevel level.Level) Option {
	return func(opts *Options) error {
		opts.minLevelOp = func(l logger.Logger) logger.Logger {
			return l.WithLevel(minLevel)
		}
		return nil
	}
}
