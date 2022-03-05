package framework

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/zhangel/go-framework.git/balancer"
	"github.com/zhangel/go-framework.git/config"
	_ "github.com/zhangel/go-framework.git/config"
	_ "github.com/zhangel/go-framework.git/config_plugins"
	_ "github.com/zhangel/go-framework.git/control"
	_ "github.com/zhangel/go-framework.git/control_plugins"
	_ "github.com/zhangel/go-framework.git/credentials"
	_ "github.com/zhangel/go-framework.git/db"
	"github.com/zhangel/go-framework.git/declare"
	internal_config "github.com/zhangel/go-framework.git/internal/config"
	"github.com/zhangel/go-framework.git/lifecycle"
	framework_logger "github.com/zhangel/go-framework.git/log"
	_ "github.com/zhangel/go-framework.git/profile"
	_ "github.com/zhangel/go-framework.git/prometheus"
	_ "github.com/zhangel/go-framework.git/registry"
	_ "github.com/zhangel/go-framework.git/retry"
	_ "github.com/zhangel/go-framework.git/tracing"
	"go.uber.org/automaxprocs/maxprocs"
)

var (
	once                          sync.Once
	presetOpt                     []Option
	initialized                   = uint32(0)
	flagIdleMemoryReleaseInterval = "framework.idle_mem_release_interval"
	frameworkPrefix               = "framework"
	flagForceGC                   = "framework.force_gc"
)

func init() {
	//declare code ...
	declare.Flags(frameworkPrefix,
		declare.Flag{
			Name:         flagIdleMemoryReleaseInterval,
			DefaultValue: -1,
			Description:  "Interval in seconds of force release idle memory to os -1 means never.",
		},
		declare.Flag{
			Name:         flagForceGC,
			DefaultValue: false,
			Description:  "Whether do forceGC while     try to release idle memory to os.",
		},
	)
	log.SetFlags(log.Lshortfile)
	_, _ = maxprocs.Set(maxprocs.Logger(func(string, ...interface{}) {}))
}

func Finalize() {
	lifecycle.LifeCycle().Finalize()
}

func Init(opt ...Option) func() {
	once.Do(func() {
		opts := &Options{}
		for _, o := range append(presetOpt, opt...) {
			if err := o(opts); err != nil {
				log.Fatal("[LOAD_ERROR]", opt)
			}
		}
		internal_config.PrepareConfigs(opts.beforeConfigPreparer, opts.configPreparer,
			opts.defaultConfigSource, opts.compactUsage,
			opts.flagsToShow, opts.flagsToHide)
		lifecycle.LifeCycle().Initialize(opts.finalizeTimeout)
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			select {
			case <-sig:
				fmt.Println("Got SIGINT|SIGTERM signal, cleaning then terminate")
				lifecycle.Exit(1)
			}
		}()
		memReleaseInterval := config.Int(flagIdleMemoryReleaseInterval)
		forceGC := config.Bool(flagForceGC)
		if memReleaseInterval > 0 {
			if memReleaseInterval < 10 {
				memReleaseInterval = 10
			}
			go func() {
				for {
					select {
					case <-time.After(time.Duration(config.Int(flagIdleMemoryReleaseInterval)) * time.Second):
						if forceGC {
							runtime.GC()
						}
						debug.FreeOSMemory()
						framework_logger.Debugf("Framework: free idle memory to os, forceGC = %v", forceGC)
					}
				}
			}()
		}
		//framework_logger.Debugf("Framework: debug info opts=%+v", opts)
		//log.Printf("%+v\n", opts.finalizeTimeout)
		//config.Int(flagIdleMemoryReleaseInterval)
		atomic.CompareAndSwapUint32(&initialized, 0, 1)
	})
	return Finalize
}
