package internal

import (
	"strings"
	"time"

	"github.com/zhangel/go-framework.git/internal"
)

func Walk(prefix string, content map[string]interface{}, config *map[string]string) {
	withPrefix := func(k string) string {
		if prefix == "" {
			return k
		} else {
			return prefix + "." + k
		}
	}

	for k, v := range content {
		switch v := v.(type) {
		case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, time.Duration:
			(*config)[withPrefix(k)] = internal.Stringify(v)
		case map[string]interface{}:
			Walk(withPrefix(k), v, config)
		case []interface{}:
			strList, ok := withSlice(v)
			if ok {
				(*config)[withPrefix(k)] = strList
			}
		}
	}
}

func withSlice(slice []interface{}) (string, bool) {
	var result []string

	for _, val := range slice {
		result = append(result, internal.Stringify(val))
	}

	if len(result) > 0 {
		return strings.Join(result, ","), true
	} else {
		return "", false
	}
}
