package log

import (
	"context"
	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/log/fields"
	"github.com/zhangel/go-framework/log/level"
	"github.com/zhangel/go-framework/log/logger"
	"time"
)

func WithContext(ctx context.Context) logger.Logger {
	return DefaultLogger().WithContext(ctx)
}

func WithConfig(config config.Config) logger.Logger {
	return DefaultLogger().WithConfig(config)
}

func LogLevel() level.Level {
	return DefaultLogger().LogLevel()
}

func WithField(key string, value interface{}) logger.Logger {
	return DefaultLogger().WithField(key, value)
}

func WithFields(fields fields.Fields) logger.Logger {
	return DefaultLogger().WithFields(fields)
}

func WithLevel(level level.Level) logger.Logger {
	return DefaultLogger().WithLevel(level)
}

func WithRateLimit(interval time.Duration) logger.Logger {
	return DefaultLogger().WithRateLimit(interval)
}

//logger function import
func Debug(args ...interface{}) {
	DefaultLogger().Debug(args...)
}

func Warn(args ...interface{}) {
	DefaultLogger().Warn(args...)
}

func Info(args ...interface{}) {
	DefaultLogger().Info(args...)
}

func Fatal(args ...interface{}) {
	DefaultLogger().Fatal(args...)
}

func Error(args ...interface{}) {
	DefaultLogger().Error(args...)
}

func Fatalf(format string, args ...interface{}) {
	DefaultLogger().Fatalf(format, args...)
}

func Warnf(format string, args ...interface{}) {
	DefaultLogger().Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	DefaultLogger().Errorf(format, args...)
}

func Infof(format string, args ...interface{}) {
	DefaultLogger().Infof(format, args...)
}

func Debugf(format string, args ...interface{}) {
	DefaultLogger().Debugf(format, args...)
}
