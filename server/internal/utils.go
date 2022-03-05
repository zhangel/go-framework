package internal

import (
	"fmt"
	"time"
)

func InvokeWithTimeout(fn func() error, timeout time.Duration) error {
	ch := make(chan error)
	toTimer := time.NewTimer(timeout)
	defer toTimer.Stop()

	go func(ch chan error) {
		ch <- fn()
	}(ch)

	select {
	case resp := <-ch:
		return resp
	case <-toTimer.C:
		return fmt.Errorf("timeout (to = %v)", timeout)
	}
}
