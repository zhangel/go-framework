package callstack

import (
	"bytes"
	"runtime/pprof"
)

func FullCallStack() {
	Bytes := bytes.NewBuffer(nil)
	pprof.Lookup("goroutine").WriteTo(Bytes, 1)
	return string(Bytes.Bytes())
}
