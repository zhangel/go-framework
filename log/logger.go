package log

import (
	"errors"
	"github.com/zhangel/go-framework.git/config"
	"github.com/zhangel/go-framework.git/log/logger"
)

type _Logger struct {
	opts Options
}

func (s *_Logger) Close() error {
	if s.opts.writer != nil {
		s.opts.writer.Close()
	}
	return nil
}

func (s *_Logger) Debugf(format string, parameters ...interface{}) {
	newEntry(s).Debugf(format, parameters...)
}

func (s *_Logger) WithConfig(config config.Config) logger.Logger {
	return newEntry(s).WithConfig(config)
}

func (s *_Logger) WithField(key string, value interface{}) logger.Logger {
	return newEntry(s).WithField(key, value)
}

func NewLogger(opts ...Option) (logger.Logger, error) {
	log := &_Logger{}
	for _, o := range opts {
		if err := o(&log.opts); err != nil {
			return nil, err
		}
	}
	if log.opts.encoder == nil {
		return nil, errors.New("no log encoder available")
	}

	if log.opts.writer == nil {
		return nil, errors.New("no log writer available")
	}
	logger := log.WithConfig(config.GlobalConfig())
	if log.opts.minLevelOp != nil {
		logger = log.opts.minLevelOp(logger)
	}
	return logger, nil
}
