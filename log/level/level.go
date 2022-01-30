package level

import (
	"fmt"
	"strings"
)

type Level uint32

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	None
)

func (level Level) String() string {
	switch level {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	case TraceLevel:
		return "TRACE"
	case None:
		return "NONE"
	default:
		return "UNKNOWN"
	}
}

func ParseLevel(level string) (Level, error) {
	switch strings.ToLower(level) {
	case "info":
		return InfoLevel, nil
	case "warn":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "trace":
		return TraceLevel, nil
	case "none":
		return None, nil
	case "debug":
		return DebugLevel, nil
	default:
		return None, fmt.Errorf("unknown error level %s", level)
	}
}
