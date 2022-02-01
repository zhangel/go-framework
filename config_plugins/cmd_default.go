package config_plugins

import (
	"flag"
	"github.com/zhangel/go-framework/config/watcher"
	"github.com/zhangel/go-framework/internal/declare"
)

type CmdDefaultConfigSource struct{}

func NewCmdDefaultConfigSource() *CmdDefaultConfigSource {
	return &CmdDefaultConfigSource{}
}

func (s *CmdDefaultConfigSource) Sync() (map[string]string, error) {
	val := map[string]string{}
	declare.FrameworkFlagSet.VisitAll(func(f *flag.Flag) {
		val[f.Name] = f.DefValue
	})

	return val, nil
}

func (s *CmdDefaultConfigSource) Watch(watcher.SourceWatcher) (cancel func()) {
	return func() {}
}

func (s *CmdDefaultConfigSource) AppendPrefix(_ []string) error {
	return nil
}

func (s *CmdDefaultConfigSource) Close() error {
	return nil
}

func (s *CmdDefaultConfigSource) IgnoreNamespace() {}
