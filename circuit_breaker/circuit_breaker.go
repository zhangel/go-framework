package circuit_breaker

import (
	"sync"
	"time"

	"github.com/zhangel/go-framework/control"
	"github.com/zhangel/go-framework/log"

	"github.com/cep21/circuit/v3"
	"github.com/cep21/circuit/v3/closers/hystrix"
)

var (
	once    sync.Once
	manager *_CircuitBreakerManager
)

type _CircuitBreakerManager struct {
	circuitBreakerMu sync.RWMutex
	circuitBreakers  map[string]*circuitBreakerWrapper
}

type circuitBreakerWrapper struct {
	mu sync.RWMutex
	*circuit.Circuit
	policy *control.CircuitBreakerPolicy
}

func circuitBreakerManager() *_CircuitBreakerManager {
	once.Do(func() {
		manager = &_CircuitBreakerManager{
			circuitBreakers: map[string]*circuitBreakerWrapper{},
		}
	})

	return manager
}

func newCircuitBreakerWrapper(name string, policy *control.CircuitBreakerPolicy) *circuitBreakerWrapper {
	log.Infof("circuit_breaker: initialize circuit breaker policy for %q, policy = %+v", name, policy)

	return &circuitBreakerWrapper{
		Circuit: circuit.NewCircuitFromConfig(name, circuit.Config{
			General: circuit.GeneralConfig{
				ClosedToOpenFactory: hystrix.OpenerFactory(hystrix.ConfigureOpener{
					RequestVolumeThreshold:   int64(policy.RequestVolumeThreshold),
					ErrorThresholdPercentage: int64(policy.ErrorPercentThreshold),
					RollingDuration:          time.Duration(policy.RollingDurationInSec) * time.Second,
					NumBuckets:               policy.RollingDurationInSec,
				}),
				OpenToClosedFactory: hystrix.CloserFactory(hystrix.ConfigureCloser{
					SleepWindow: policy.SleepWindow,
				}),
			},
			Execution: circuit.ExecutionConfig{
				Timeout:               policy.Timeout,
				MaxConcurrentRequests: int64(policy.MaxConcurrentRequests),
			},
			Fallback: circuit.FallbackConfig{
				MaxConcurrentRequests: -1,
			},
		}),
		policy: policy,
	}
}

func (s *circuitBreakerWrapper) UpdatePolicy(policy *control.CircuitBreakerPolicy) {
	s.mu.RLock()
	if policy.Version == s.policy.Version {
		s.mu.RUnlock()
		return
	}

	if policy.Enabled == s.policy.Enabled &&
		policy.Timeout == s.policy.Timeout &&
		policy.MaxConcurrentRequests == s.policy.MaxConcurrentRequests &&
		policy.RequestVolumeThreshold == s.policy.RequestVolumeThreshold &&
		policy.SleepWindow == s.policy.SleepWindow &&
		policy.ErrorPercentThreshold == s.policy.ErrorPercentThreshold &&
		policy.RollingDurationInSec == s.policy.RollingDurationInSec {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = policy

	s.Circuit.SetConfigThreadSafe(circuit.Config{
		Execution: circuit.ExecutionConfig{
			Timeout:               policy.Timeout,
			MaxConcurrentRequests: int64(policy.MaxConcurrentRequests),
		},
		Fallback: circuit.FallbackConfig{
			MaxConcurrentRequests: -1,
		},
	})
	s.Circuit.OpenToClose.(*hystrix.Closer).SetConfigThreadSafe(hystrix.ConfigureCloser{
		SleepWindow: policy.SleepWindow,
	})
	s.Circuit.ClosedToOpen.(*hystrix.Opener).SetConfigThreadSafe(hystrix.ConfigureOpener{
		RequestVolumeThreshold:   int64(policy.RequestVolumeThreshold),
		ErrorThresholdPercentage: int64(policy.ErrorPercentThreshold),
		RollingDuration:          time.Duration(policy.RollingDurationInSec) * time.Second,
		NumBuckets:               policy.RollingDurationInSec,
	})

	log.Infof("circuit_breaker: update circuit breaker policy for %q, policy = %+v", s.Circuit.Name(), policy)
}

func (s *_CircuitBreakerManager) CircuitWithPolicy(name string, policy *control.CircuitBreakerPolicy) *circuit.Circuit {
	s.circuitBreakerMu.RLock()
	if b, ok := s.circuitBreakers[name]; ok {
		s.circuitBreakerMu.RUnlock()
		b.UpdatePolicy(policy)
		return b.Circuit
	}
	s.circuitBreakerMu.RUnlock()

	s.circuitBreakerMu.Lock()
	if b, ok := s.circuitBreakers[name]; ok {
		b.UpdatePolicy(policy)
		s.circuitBreakerMu.Unlock()
		return b.Circuit
	}
	defer s.circuitBreakerMu.Unlock()

	wrapper := newCircuitBreakerWrapper(name, policy)
	s.circuitBreakers[name] = wrapper
	return wrapper.Circuit
}
