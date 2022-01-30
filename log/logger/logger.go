package logger

import (
	"context"
	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/log/fields"
	"github.com/zhangel/go-framework/log/level"
	"time"
)

type Logger interface {
	WithContext(context.Context) Logger
	WithField(key string, value interface{}) Logger
	WithFields(fields fields.Fields) Logger
	WithLevel(level level.Level) Logger
	WithRateLimit(interval time.Duration) Logger
	WithConfig(config config.Config) Logger

	LogLevel() level.Level
	Close() error

	//other aften use function
	Debugf(format string, args ...interface{})
	Tracef(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Infof(format string, args ...interface{})

	Trace(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Warn(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
}
