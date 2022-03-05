package balancer

import (
	"context"
	"fmt"
	"strconv"
)

var consistHashKey = struct{}{}

func WithConsistHashKey(ctx context.Context, id interface{}) context.Context {
	return context.WithValue(ctx, consistHashKey, id)
}

func ConsistHashKey(ctx context.Context) string {
	switch v := ctx.Value(consistHashKey).(type) {
	case string:
		return v
	case int:
	case uint:
	case int16:
	case uint16:
	case int32:
	case uint32:
	case int64:
	case uint64:
		return strconv.FormatUint(uint64(v), 10)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%+v", v)
	}

	return ""
}
