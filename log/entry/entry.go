package entry

import (
	"github.com/zhangel/go-framework/log/fields"
	"github.com/zhangel/go-framework/log/level"
	"runtime"
	"time"
)

type Entry struct {
	Fields fields.Fields
	Time   time.Time
	Level  level.Level
	Caller *runtime.Frame
	Msg    string
}
