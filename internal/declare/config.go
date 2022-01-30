package declare

import (
	"github.com/zhangel/go-framework/uri"
)

var (
	allFlagsWithNs = map[string][]Flag{}
)

type Flag struct {
	Name             string
	DefaultValue     interface{}
	Description      string
	Env              string
	UriField         uri.UriFieldType
	UriFieldHandler  func(string) string
	Sensitive        bool
	Deprecated       bool
	pluginFlagType   bool
	pluginType       string
	pluginName       string
	pluginDeprecated bool
}

type ModifiableConfigSource interface {
	Get(k string) (interface{}, error)
	Put(k string, v interface{})
}

func Flags(ns string, flags ...Flag) {
	allFlagsWithNs[ns] = append(allFlagsWithNs[ns], flags...)
}
