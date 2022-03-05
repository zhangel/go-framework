package retry

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/zhangel/go-framework/control"
	"github.com/zhangel/go-framework/internal/retry"
	"github.com/zhangel/go-framework/log"

	"google.golang.org/grpc/status"
)

var (
	once    sync.Once
	manager *_RetryManager
)

type backoffPolicy struct {
	backoffDuration    time.Duration
	backoffJitter      float64
	backoffExponential bool
}

func NewBackoffPolicy(backoffDuration time.Duration, backoffJitter float64, backoffExponential bool) retry.TimePolicy {
	return &backoffPolicy{backoffDuration, backoffJitter, backoffExponential}
}

func (s *backoffPolicy) Calc(_ context.Context, taskContext *retry.TaskContext) time.Duration {
	if taskContext.RetryNum == 0 {
		return 0
	} else {
		return s.jitterUp(s.exponentBase2(s.backoffDuration, taskContext.RetryNum))
	}
}

func (s *backoffPolicy) exponentBase2(duration time.Duration, attemp uint) time.Duration {
	if !s.backoffExponential {
		return duration
	}

	return duration * time.Duration((1<<attemp)>>1)
}

func (s *backoffPolicy) jitterUp(duration time.Duration) time.Duration {
	if s.backoffJitter == 0.0 {
		return duration
	}

	rand.Seed(time.Now().UnixNano())
	multiplier := s.backoffJitter * (rand.Float64()*2 - 1)
	return time.Duration(float64(duration) * (1 + multiplier))
}

type fixedTimeoutTimePolicy struct {
	timeout time.Duration
}

func FixedTimeoutPolicy(timeout time.Duration) retry.TimePolicy {
	return &fixedTimeoutTimePolicy{timeout}
}

func (s *fixedTimeoutTimePolicy) Calc(ctx context.Context, taskContext *retry.TaskContext) time.Duration {
	return s.timeout
}

type fixedRetryCountPolicy struct {
	retryTimes uint
}

func FixedRetryCountPolicy(retryTimes uint) *fixedRetryCountPolicy {
	return &fixedRetryCountPolicy{retryTimes}
}

func (s *fixedRetryCountPolicy) Calc(ctx context.Context, taskContext *retry.TaskContext) bool {
	return taskContext.RetryNum <= s.retryTimes
}

type _RetryManager struct {
	retryControllersMu sync.RWMutex
	retryControllers   map[string]*retryControllerWrapper
}

type retryControllerWrapper struct {
	mu   sync.RWMutex
	name string
	retry.RetryController
	policy *control.RetryPolicy
}

func retryManager() *_RetryManager {
	once.Do(func() {
		manager = &_RetryManager{
			retryControllers: map[string]*retryControllerWrapper{},
		}
	})

	return manager
}

func (s *_RetryManager) RetryControlByPolicy(name string, policy *control.RetryPolicy) *retryControllerWrapper {
	s.retryControllersMu.RLock()
	if b, ok := s.retryControllers[name]; ok {
		s.retryControllersMu.RUnlock()
		b.UpdatePolicy(policy)
		return b
	}
	s.retryControllersMu.RUnlock()

	s.retryControllersMu.Lock()
	if b, ok := s.retryControllers[name]; ok {
		b.UpdatePolicy(policy)
		s.retryControllersMu.Unlock()
		return b
	}
	defer s.retryControllersMu.Unlock()

	wrapper := newRetryControllerWrapper(name, policy)
	s.retryControllers[name] = wrapper
	return wrapper
}

func newRetryController(policy *control.RetryPolicy) retry.RetryController {
	retryOpts := []func(*retry.Options){
		retry.WithTimeoutPolicy(FixedTimeoutPolicy(policy.PerTryTimeout)),
		retry.WithRetryPolicy(FixedRetryCountPolicy(policy.NumOfRetries)),
		retry.WithRetryIntervalPolicy(NewBackoffPolicy(policy.BackoffDuration, policy.BackoffJitter, policy.BackoffExponential)),
	}
	if policy.Overlap {
		retryOpts = append(retryOpts, retry.WithConcurrencyPolicy(retry.AlwaysConcurrencyPolicy()))
	} else {
		retryOpts = append(retryOpts, retry.WithConcurrencyPolicy(retry.NeverConcurrencyPolicy()))
	}

	return retry.NewRetryController(retryOpts...)
}

func newRetryControllerWrapper(name string, policy *control.RetryPolicy) *retryControllerWrapper {
	log.Infof("retry: initialize retry policy for %q, policy = %+v", name, policy)

	return &retryControllerWrapper{
		name:            name,
		RetryController: newRetryController(policy),
		policy:          policy,
	}
}

func (s *retryControllerWrapper) UpdatePolicy(policy *control.RetryPolicy) {
	s.mu.RLock()
	if policy.Version == s.policy.Version {
		s.mu.RUnlock()
		return
	}

	if policy.Enabled == s.policy.Enabled &&
		policy.NumOfRetries == s.policy.NumOfRetries &&
		policy.PerTryTimeout == s.policy.PerTryTimeout &&
		policy.Overlap == s.policy.Overlap &&
		policy.BackoffDuration == s.policy.BackoffDuration &&
		policy.BackoffJitter == s.policy.BackoffJitter &&
		policy.BackoffExponential == s.policy.BackoffExponential {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = policy
	s.RetryController = newRetryController(policy)
	log.Infof("retry: update retry for %q, policy = %+v", s.name, policy)
}

func (s *retryControllerWrapper) Invoke(ctx context.Context, retriableCodes []int, invoker func(ctx context.Context) error) (err error) {
	logger := log.WithContext(ctx).WithField("tag", "retry::RetryHandler")
	_, err = s.RetryController.Invoke(ctx, func(ctx context.Context, cb retry.TaskCallback) {
		if cb.TaskContext().RetryNum != 0 {
			logger.Tracef("retrying, attempts = %d, reason = %+v", cb.TaskContext().RetryNum, cb.TaskContext().LastError)
		}

		if err = invoker(ctx); err == nil {
			cb.OnSuccess(nil)
		} else if s, ok := status.FromError(err); ok {
			for _, code := range retriableCodes {
				if uint32(code) == uint32(s.Code()) {
					cb.Retry(err)
					return
				}
			}
			cb.OnError(err)
		} else {
			cb.OnError(err)
		}
	})
	return err
}
