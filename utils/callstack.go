package utils

import (
	"bytes"
	"runtime/pprof"
)

func FullCallStack() string {
	Bytes := bytes.NewBuffer(nil)
	pprof.Lookup("goroutine").WriteTo(Bytes, 1)
	return string(Bytes.Bytes())
}
