package encoder

import (
	"github.com/zhangel/go-framework/log/entry"
)

type Encoder interface {
	Encode(entry *entry.Entry) ([]byte, error)
}
