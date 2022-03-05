package retry

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"
)

func Test_SerialRetry(t *testing.T) {
	numGoroutine := runtime.NumGoroutine()
	retryController := SerializationSimpleRetryController(3, time.Duration(50*time.Millisecond), time.Duration(2*time.Second))
	result, err := retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(1 * time.Second))
		select {
		case <-timer.C:
			cb.OnSuccess("TIMEOUT, NOT SUCCESS")
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err == nil {
		t.Error("FAILED")
	}

	result, err = retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(500 * time.Millisecond))
		select {
		case <-timer.C:
			cb.OnSuccess("THIS IS RESULT")
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err != nil {
		t.Error("FAILED")
	}

	result, err = retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(500 * time.Millisecond))
		select {
		case <-timer.C:
			cb.OnError(errors.New("MUST FAILED"))
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err == nil {
		t.Error("FAILED")
	}

	retryTimes := 0
	result, err = retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(500 * time.Millisecond))
		select {
		case <-timer.C:
			if retryTimes == 0 {
				retryTimes++
				cb.Retry(errors.New("Let's RETRY"))
			} else {
				cb.OnSuccess("HAHA, FINALLY DONE AFTER RETRY")
			}
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err != nil {
		t.Error("FAILED")
	}

	time.Sleep(10 * time.Millisecond)
	if runtime.NumGoroutine() != numGoroutine {
		t.Errorf("FAILED, GORoutine = %d, expect = %d", runtime.NumGoroutine(), numGoroutine)
	}
}

func Test_ConcurrencyRetry(t *testing.T) {
	numGoroutine := runtime.NumGoroutine()
	retryController := ConcurrencySimpleRetryController(3, time.Duration(50*time.Millisecond), time.Duration(2*time.Second))
	result, err := retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(1 * time.Second))
		select {
		case <-timer.C:
			cb.OnSuccess("TIMEOUT, BUT SUCCESSFUL FINALLY")
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err != nil {
		t.Error("FAILED")
	}

	result, err = retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(500 * time.Millisecond))
		select {
		case <-timer.C:
			cb.OnSuccess("THIS IS RESULT")
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err != nil {
		t.Error("FAILED")
	}

	result, err = retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(500 * time.Millisecond))
		select {
		case <-timer.C:
			cb.OnError(errors.New("MUST FAILED"))
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err == nil {
		t.Error("FAILED")
	}

	retryTimes := 0
	result, err = retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
		timer := time.NewTimer(time.Duration(500 * time.Millisecond))
		select {
		case <-timer.C:
			if retryTimes == 0 {
				retryTimes++
				cb.Retry(errors.New("Let's RETRY"))
			} else {
				cb.OnSuccess("HAHA, FINALLY DONE AFTER RETRY")
			}
		case <-ctx.Done():
			fmt.Println("Task exceed deadline")
		}
	})

	fmt.Printf("result = %v, err = %v\n", result, err)
	if err != nil {
		t.Error("FAILED")
	}

	time.Sleep(10 * time.Millisecond)
	if runtime.NumGoroutine() != numGoroutine {
		t.Errorf("FAILED, GORoutine = %d, expect = %d", runtime.NumGoroutine(), numGoroutine)
	}
}

func Test_TaskId(t *testing.T) {
	retryController := NewRetryController(
		WithTimeoutPolicy(FixedTimeoutPolicy(50*time.Millisecond)),
		WithRetryIntervalPolicy(FixedRetryIntervalPolicy(50*time.Millisecond)),
		WithRetryPolicy(FixedRetryCountPolicy(3)),
		WithConcurrencyPolicy(NeverConcurrencyPolicy()),
		WithCancelError(errors.New("WAHAHAHAHA")),
	)

	ch := make(chan struct{})
	go func() {
		result, err := retryController.Invoke(context.Background(), func(ctx context.Context, cb TaskCallback) {
			timer := time.NewTimer(time.Duration(1 * time.Second))
			select {
			case <-timer.C:
				cb.OnSuccess("TIMEOUT, NOT SUCCESS")
			case <-ctx.Done():
				fmt.Println("Task exceed deadline")
			}
		}, InvokeWithTaskId("TTT"))

		ch <- struct{}{}
		fmt.Println("TEST DONE")
		fmt.Printf("result = %v, err = %v\n", result, err)

		if err == nil {
			t.Error("FAILED")
		}

		if retryController.HasTask("TTT") {
			t.Error("Task should gone")
		}
	}()

	time.Sleep(50 * time.Millisecond)
	go func() {
		if !retryController.HasTask("TTT") {
			t.Error("No task found")
		}
	}()
	time.Sleep(50 * time.Millisecond)
	retryController.CancelTask("TTT")
	fmt.Println("CANCEL")

	<-ch

	if retryController.HasTask("TTT") {
		t.Error("Task should gone")
	}
	fmt.Println("DONE")
}
