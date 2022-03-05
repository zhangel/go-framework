package encoder

import (
	"fmt"
	"github.com/zhangel/go-framework.git/log/entry"
	"path/filepath"
	"strings"
)

type SimpleTextEncoder struct {
	sep            string
	withFixedParts bool
}

func NewSimpleTextEncoder(sep string, withFixedParts bool) *SimpleTextEncoder {
	return &SimpleTextEncoder{sep, withFixedParts}
}

func (s *SimpleTextEncoder) Encode(entry *entry.Entry) ([]byte, error) {
	logItem := []string{}
	if s.withFixedParts {
		_, fileName := filepath.Split(entry.Caller.File)
		logItem = append(logItem, fmt.Sprintf("%s",
			entry.Time.Format("2006-01-02 15:04:05.000000")))
		logItem = append(logItem, fmt.Sprintf("[%s]", entry.Level.String()))
		logItem = append(logItem, fmt.Sprintf("<%s:%d>", fileName, entry.Caller.Line))
	}
	for _, field := range entry.Fields {
		logItem = append(logItem, fmt.Sprintf("%s:%v", field.K, field.V))
	}
	logItem = append(logItem, fmt.Sprintf("msg:%s", entry.Msg))
	return []byte(strings.Join(logItem, s.sep)), nil
}
