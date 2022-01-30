package log

import (
	"context"
	//"fmt"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log/fields"
	"github.com/zhangel/go-framework/log/logger"
)

var (
	ctxFieldProviders []CtxFieldProvider
)

type CtxFieldProvider func(ctx context.Context) fields.Fields

func DefaultLogger() logger.Logger {
	//默认Logger配置实例
	if !lifecycle.LifeCycle().IsInitialized() {
		l, _ := NewConsoleLogger(true)()
		return l
	}

	return nil
}
