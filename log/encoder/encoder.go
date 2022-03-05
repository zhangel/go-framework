package encoder

import (
	"github.com/zhangel/go-framework.git/log/entry"
)

type Encoder interface {
	Encode(entry *entry.Entry) ([]byte, error)
}
