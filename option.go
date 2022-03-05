package framework

import (
	"github.com/zhangel/go-framework.git/config"
	"time"
)

type Options struct {
	defaultConfigSource  []config.Source
	configPreparer       func(config.Config) (config.Config, error)
	finalizeTimeout      time.Duration
	compactUsage         bool
	flagsToShow          []string
	flagsToHide          []string
	beforeConfigPreparer []func()
}

type Option func(*Options) error

//func list
