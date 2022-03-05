// +build windows

package option

import (
	"strings"

	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/server/internal"
)

var AddrFlag = declare.Flag{Name: internal.FlagAddr, DefaultValue: internal.AutoSelectAddr, Description: "Bind address of the grpc server. optionals: address, 'auto'. Use 'pipe://{pipe_name}' to specify a naming pipe to listen."}

func WithAddr(addr string) Option {
	return func(opts *Options) (err error) {
		if strings.HasPrefix(addr, "pipe://") {
			opts.Addr = strings.Replace(addr, "pipe://", "\\\\.\\\\pipe\\", 1)
		} else {
			opts.Addr = addr
		}

		return nil
	}
}

func WithHttpAddr(addr string) Option {
	return func(opts *Options) (err error) {
		if strings.HasPrefix(addr, "pipe://") {
			opts.HttpAddr = strings.Replace(addr, "pipe://", "\\\\.\\\\pipe\\", 1)
		} else {
			opts.HttpAddr = addr
		}

		return nil
	}
}

func IsPipeAddress(addr string) bool {
	return strings.HasPrefix(addr, "\\\\.\\\\pipe\\")
}
