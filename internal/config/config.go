package config

import (
	"fmt"
	"github.com/zhangel/go-framework/config"
)

func PrepareConfigs(beforeConfigPrepareHook []func(),
	configPrepare func(config.Config) (config.Config, error),
	configSources []config.Source, compactUsage bool,
	flagsToShow, flagsToHide []string) {

}
