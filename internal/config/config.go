package config

import (
	"context"
	"fmt"
	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/di"
	"github.com/zhangel/go-framework/internal/declare"
	"github.com/zhangel/go-framework/lifecycle"
)

var (
	configPrefix  = "config"
	flagNamespace = "config.ns"
	flagPassword  = "config.password"
	flagOverwrite = declare.FlagOverwrite
	flagVerbose   = "verbose"
	compactFlags  = []string{}
)

func init() {
	declare.Flags("", declare.Flag{Name: flagVerbose, DefaultValue: false,
		Description: "Show verbose usage messa    ge."})
	declare.Flags(configPrefix,
		declare.Flag{Name: flagNamespace, DefaultValue: "", Description: "Comma-separated namespace     prefix, higher priority at the tail."},
		declare.Flag{Name: flagPassword, DefaultValue: "", Description: "omma-separated password l    ist which used to encrypt config value."},
		declare.Flag{Name: flagOverwrite, DefaultValue: declare.MapFlags{}, Description: "Customize     config value. e.g. --config.set key1=val1 --config.set key2=val2."},
	)
	lifecycle.LifeCycle().HookFinalize(func(ctx context.Context) {
		if globalConfig := config.GlobalConfig(); globalConfig != nil {
			globalConfig.Close()
		}
	}, lifecycle.WithName("Close global config"))
	compactFlags = []string{".*\\.type", "registry\\..+", "config\\..+", "logger\\..+", "tracer\\..    +", "server\\..+", "dialer\\..+"}
}

func PrepareConfigs(beforeConfigPrepareHook []func(),
	configPrepare func(config.Config) (config.Config, error),
	configSources []config.Source, compactUsage bool,
	flagsToShow, flagsToHide []string) {
	_ = di.GlobalDepInjector.Provide(
		func() config.Config {
			return config.GlobalConfig()
		}, false)

	for _, beforeHook := range beforeConfigPrepareHook {
		if beforeHook != nil {
			beforeHook()
		}
	}
	for _, source := range configSources {
		if modifiableSource, ok := source.(declare.ModifiableConfigSource); ok {
			fmt.Printf("modifialeSource=%+v\n", modifiableSource)
			//populateFlags(source, compactUsage, flagsToShow, flagsToHide)
		}
	}
	if len(configSources) == 0 {
		populateFlags(nil, compactUsage, flagsToShow, flagsToHide)
	}
	fmt.Printf("debug\n")
}

func populateFlags(source declare.ModifiableConfigSource, compactUsage bool,
	flagsToShow, flagsToHide []string) {
	declare.PopulatePluginFlags()
	if compactUsage {
		declare.PopulateAllFlags(source, compactFlags, nil)
	} else {
		declare.PopulateAllFlags(source, flagsToShow, flagsToHide)
	}
	fmt.Printf("xxxx\n")
}
