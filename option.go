package framework

import (
	"github.com/zhangel/go-framework/config"
	"time"
)

type Options struct {
	defaultConfigSource []config.Source
	finalizeTimeout     time.Duration
	compactUsage        bool
	flagsToShow         []string
	flagsToHide         []string
	beforeConfigParser  []func()
}

type Option func(*Options) error

//func list
