package declare

import (
	"github.com/zhangel/go-framework/internal/declare"
)

type Flag declare.Flag

func Flags(ns string, flags ...Flag) {
	var internalFlags []declare.Flag
	for _, f := range flags {
		internalFlags = append(internalFlags, declare.Flag(f))
	}
	declare.Flags(ns, internalFlags...)
}
