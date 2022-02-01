package utils

import (
	"context"
	"log"
	"time"
)

func TimeoutGuard(timeout time.Duration, taskName string) func() {
	return TimeoutGuardWithFunc(timeout, func(...interface{}) {
		log.Printf("%s ...\n", taskName)
	}, func(...interface{}) {
		log.Printf("%s done.\n", taskName)
	}, func(...interface{}) {
		log.Fatalf("%s timeout.\n", taskName)
	})
}

func TimeoutGuardWithFunc(timeout time.Duration, onBegin, onEnd, onTimeout func(...interface{}), args ...interface{}) func() {
	if onBegin != nil {
		onBegin(args...)
	}

	timer := time.NewTimer(timeout)

	ctx, canceler := context.WithCancel(context.Background())
	go func() {
		select {
		case <-timer.C:
			if onTimeout != nil {
				onTimeout(args...)
			}
		case <-ctx.Done():
			timer.Stop()
			if onEnd != nil {
				onEnd(args...)
			}
		}
	}()

	return canceler
}
