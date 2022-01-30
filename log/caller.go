package log

import (
	"runtime"
	"strings"
	"sync"
)

var (
	packageName string
	once        sync.Once
	funcSkips   = map[string]interface{}{
		"Trace":     struct{}{},
		"Debug":     struct{}{},
		"Info":      struct{}{},
		"Warn":      struct{}{},
		"Error":     struct{}{},
		"Fatal":     struct{}{},
		"Tracef":    struct{}{},
		"Debugf":    struct{}{},
		"Infof":     struct{}{},
		"Warnf":     struct{}{},
		"Errorf":    struct{}{},
		"Fatalf":    struct{}{},
		"Infoln":    struct{}{},
		"Warningln": struct{}{},
		"Warningf":  struct{}{},
		"Errorln":   struct{}{},
		"Fatalln":   struct{}{},
	}
)

func GetPackageName(p string) string {
	for {
		lastPeriod := strings.LastIndex(p, ".")
		lastSlash := strings.LastIndex(p, "/")
		if lastPeriod > lastSlash {
			p = p[:lastPeriod]
		} else {
			break
		}
	}
	return p
}

func compactFunctionName(funcName string) string {
	lastPeriod := strings.LastIndex(funcName, ".")
	if lastPeriod == -1 {
		return funcName
	} else if lastPeriod+1 >= len(funcName) {
		return funcName
	} else {
		return funcName[lastPeriod+1:]
	}
}

func GetCaller(skipLogFunc bool) *runtime.Frame {
	pcs := make([]uintptr, 10)
	depth := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:depth])
	once.Do(func() {
		packageName = GetPackageName(runtime.FuncForPC(pcs[0]).Name())
	})

	for f, again := frames.Next(); again; f, again = frames.Next() {
		pkg := GetPackageName(f.Function)
		if pkg != packageName && (!skipLogFunc ||
			funcSkips[compactFunctionName(f.Function)] == nil) {
			return &f
		}
	}
	return nil
}
