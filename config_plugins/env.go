package config_plugins

import (
	"os"

	"github.com/zhangel/go-framework/config/watcher"
	"github.com/zhangel/go-framework/internal/declare"
)

type EnvConfigSource struct{}

func NewEnvConfigSource() *EnvConfigSource {
	return &EnvConfigSource{}
}

func (s *EnvConfigSource) Sync() (map[string]string, error) {
	val := map[string]string{}
	for k, v := range declare.AllFlags {
		if v.Env == "" {
			continue
		}

		env := os.Getenv(v.Env)
		if env == "" {
			continue
		}

		val[k] = env
	}

	return val, nil
}

func (s *EnvConfigSource) Watch(watcher.SourceWatcher) (cancel func()) {
	return func() {}
}

func (s *EnvConfigSource) AppendPrefix(_ []string) error {
	return nil
}

func (s *EnvConfigSource) Close() error {
	return nil
}

func (s *EnvConfigSource) IgnoreNamespace() {}
