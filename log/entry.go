package log

import (
	"context"
	"fmt"
	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log/entry"
	"github.com/zhangel/go-framework/log/fields"
	"github.com/zhangel/go-framework/log/level"
	"github.com/zhangel/go-framework/log/logger"
	"sync/atomic"
	"time"
)

type rateLimitKey struct {
	file string
	line int
}

type _Entry struct {
	logger   *_Logger
	ctx      context.Context
	config   *config.Config
	minLevel *uint32
	interval *time.Duration
	cancel   []func()
	entry.Entry
}

func newEntry(logger *_Logger) *_Entry {
	minLevel := uint32(level.TraceLevel)
	interval := time.Duration(0)
	return &_Entry{
		logger:   logger,
		minLevel: &minLevel,
		interval: &interval,
		Entry: entry.Entry{
			Fields: fields.Fields{},
		},
	}
}

func (s *_Entry) isLevelEnabled(l level.Level) bool {
	return l == level.FatalLevel || l >= level.Level(atomic.LoadUint32(s.minLevel))
}

func (s *_Entry) WithConfig(config config.Config) logger.Logger {
	if config == nil {
		return s
	}
	/*
		e := newEntry(s.logger)
		fmt.Printf("e=%+v\n", e)
	*/
	return nil
}

func (s *_Entry) WithRateLimit(interval time.Duration) logger.Logger {
	e := newEntry(s.logger)
	e.Fields = append(e.Fields, s.Fields...)
	e.config = s.config
	e.minLevel = s.minLevel
	e.interval = &interval
	e.ctx = s.ctx
	return e
}

func (s *_Entry) isRateLimited() bool {
	if *s.interval == 0 {
		return false
	}
	//key := rateLimitKey{s.Caller.File, s.Caller.Line}
	return false
}

func (s _Entry) logF(level level.Level, format string, args ...interface{}) {
	s.Time = time.Now()
	s.Caller = GetCaller(true)
	if s.isRateLimited() {
		return
	}
	s.Level = level
	s.Msg = fmt.Sprintf(format, args...)
	if s.ctx != nil {
		for i := range ctxFieldProviders {
			s.Fields = append(s.Fields, ctxFieldProviders[i](s.ctx)...)
		}
	}
	if marsheld, err := s.logger.opts.encoder.Encode(&s.Entry); err == nil && len(marsheld) > 0 {
		_ = s.logger.opts.writer.Write(marsheld)
	}

}

func (s _Entry) log(level level.Level, args ...interface{}) {
	s.Time = time.Now()
	s.Caller = GetCaller(true)
	if s.isRateLimited() {
		return
	}
	s.Level = level
	s.Msg = fmt.Sprint(args...)
	if s.ctx != nil {
		for i := range ctxFieldProviders {
			s.Fields = append(s.Fields, ctxFieldProviders[i](s.ctx)...)
		}
	}
	if marsheld, err := s.logger.opts.encoder.Encode(&s.Entry); err == nil && len(marsheld) > 0 {
		s.logger.opts.writer.Write(marsheld)
	}
}

func (c *_Entry) LogLevel() level.Level {
	return level.Level(atomic.LoadUint32(c.minLevel))
}

func (c *_Entry) Close() error {
	for _, cancel := range c.cancel {
		cancel()
	}
	return c.logger.Close()
}

func (c *_Entry) WithLevel(level level.Level) logger.Logger {
	e := newEntry(c.logger)
	e.Fields = append(e.Fields, c.Fields...)
	e.ctx = c.ctx
	e.config = c.config
	minLevelU32 := uint32(level)
	e.minLevel = &minLevelU32
	e.interval = c.interval
	return e
}

func (c *_Entry) WithFields(fields fields.Fields) logger.Logger {
	e := newEntry(c.logger)
	e.Fields = append(e.Fields, c.Fields...)
	e.Fields = append(e.Fields, fields...)
	e.ctx = c.ctx
	e.config = c.config
	e.minLevel = c.minLevel
	e.interval = c.interval
	return e
}

func (c *_Entry) WithField(key string, value interface{}) logger.Logger {
	return c.WithFields(fields.Fields{fields.Field{K: key, V: value}})
}

func (c *_Entry) WithContext(ctx context.Context) logger.Logger {
	e := newEntry(c.logger)
	e.Fields = append(e.Fields, c.Fields...)
	e.ctx = ctx
	e.config = c.config
	e.minLevel = c.minLevel
	e.interval = c.interval
	return e
}

func (s *_Entry) Fatal(args ...interface{}) {
	if s.isLevelEnabled(level.FatalLevel) { //check is exists
		s.log(level.FatalLevel, args...)
	}
	lifecycle.Exit(1)
}

func (s *_Entry) Info(args ...interface{}) {
	if !s.isLevelEnabled(level.InfoLevel) { //check is exists
		return
	}
	s.log(level.InfoLevel, args...)
}

func (s *_Entry) Trace(args ...interface{}) {
	if !s.isLevelEnabled(level.TraceLevel) { //check is exists
		return
	}
	s.log(level.TraceLevel, args...)
}

func (s *_Entry) Warn(args ...interface{}) {
	if !s.isLevelEnabled(level.WarnLevel) { //check is exists
		return
	}
	s.log(level.WarnLevel, args...)
}

func (s *_Entry) Debug(args ...interface{}) {
	if !s.isLevelEnabled(level.DebugLevel) {
		return
	}
	s.log(level.DebugLevel, args...)
}

func (s *_Entry) Error(args ...interface{}) {
	if !s.isLevelEnabled(level.ErrorLevel) {
		return
	}
	s.log(level.ErrorLevel, args...)
}

func (s *_Entry) Errorf(format string, args ...interface{}) {
	if !s.isLevelEnabled(level.ErrorLevel) { //check is exists
		return
	}
	s.logF(level.ErrorLevel, format, args...)
}

func (s *_Entry) Warnf(format string, args ...interface{}) {
	if !s.isLevelEnabled(level.WarnLevel) { //check is exists
		return
	}
	s.logF(level.WarnLevel, format, args...)
}

func (s *_Entry) Fatalf(format string, args ...interface{}) {
	if s.isLevelEnabled(level.FatalLevel) { //check is exists
		s.logF(level.FatalLevel, format, args...)
	}
	lifecycle.Exit(1)
}

func (s *_Entry) Infof(format string, args ...interface{}) {
	if !s.isLevelEnabled(level.InfoLevel) { //check is exists
		return
	}
	s.logF(level.InfoLevel, format, args...)
}

func (s *_Entry) Tracef(format string, args ...interface{}) {
	if !s.isLevelEnabled(level.TraceLevel) { //check is exists DebugLevel
		return
	}
	s.logF(level.TraceLevel, format, args...)
}

func (s *_Entry) Debugf(format string, args ...interface{}) {
	if !s.isLevelEnabled(level.DebugLevel) { //check is exists DebugLevel
		return
	}
	s.logF(level.DebugLevel, format, args...)
}
