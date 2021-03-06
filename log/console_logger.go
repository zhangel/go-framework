package log

import (
	"github.com/zhangel/go-framework.git/log/encoder"
	"github.com/zhangel/go-framework.git/log/logger"
	"github.com/zhangel/go-framework.git/log/writer"
)

func NewConsoleLogger(stdout bool) func() (logger.Logger, error) {
	return func() (logger.Logger, error) {
		return NewLogger(
			WithEncoder(encoder.NewSimpleTextEncoder(" ", true)),
			WithWriter(func() writer.Writer { w, _ := writer.NewConsoleWriter(stdout); return w }()),
		)
	}
}
