package db

import (
	"sync"

	"github.com/zhangel/go-framework/declare"
)

var (
	Plugin = declare.PluginType{Name: "db"}

	databaseConfigOnce   sync.Once
	globalDatabaseConfig DatabaseConfig = DatabaseConfigFunc(func(ns string) []string { return nil })
)

type ExtraParameter struct {
	Ns string
}

type DatabaseConfig interface {
	AliasInNamespace(ns string) []string
}

type DatabaseConfigFunc func(ns string) []string

func (s DatabaseConfigFunc) AliasInNamespace(ns string) []string {
	return s(ns)
}

func RegisterDatabaseConfig(databaseConfig DatabaseConfig) {
	databaseConfigOnce.Do(func() {
		globalDatabaseConfig = databaseConfig
	})
}

func GlobalDatabaseConfig() DatabaseConfig {
	return globalDatabaseConfig
}
