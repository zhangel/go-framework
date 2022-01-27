package lifecycle

import (
	"time"
)

type Options struct {
	name     string
	priority int32
	timeout  time.Duration
}

type Option func(*Options)
