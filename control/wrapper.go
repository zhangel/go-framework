package control

import (
	"fmt"
	"sort"
	"time"

	"google.golang.org/grpc/codes"
)

type wrapper struct {
	factory Factory
}

type clientControlPlaneWrapper struct {
	clientControlPlane ClientControlPlane
}

type serverControlPlaneWrapper struct {
	serverControlPlane ServerControlPlane
}

func (s *wrapper) ClientControlPlane(service string) (ClientControlPlane, error) {
	if s.factory == nil {
		return nil, fmt.Errorf("no client control plane of %q found", service)
	}

	if cp, err := s.factory.ClientControlPlane(service); err != nil {
		return nil, err
	} else {
		return &clientControlPlaneWrapper{cp}, nil
	}
}

func (s *wrapper) ServerControlPlane(srv interface{}, service string) (ServerControlPlane, error) {
	if s.factory == nil {
		return nil, fmt.Errorf("no server control plane of %q found", service)
	}

	if cp, err := s.factory.ServerControlPlane(srv, service); err != nil {
		return nil, err
	} else {
		return &serverControlPlaneWrapper{cp}, nil
	}
}

func (s *wrapper) Release() {
	if releaser, ok := s.factory.(FactoryReleaser); ok && releaser != nil {
		releaser.Release()
	}
}

func (s *clientControlPlaneWrapper) TimeoutPolicy(serviceName, methodName string) *ClientTimeoutPolicy {
	if s.clientControlPlane == nil {
		return &ClientTimeoutPolicy{}
	}

	policy := s.clientControlPlane.TimeoutPolicy(serviceName, methodName)

	if policy == nil {
		return &ClientTimeoutPolicy{}
	} else if !policy.Enabled {
		return policy
	}

	return policy
}

func (s *clientControlPlaneWrapper) RetryPolicy(serviceName, methodName string) *RetryPolicy {
	if s.clientControlPlane == nil {
		return &RetryPolicy{}
	}

	policy := s.clientControlPlane.RetryPolicy(serviceName, methodName)

	if policy == nil {
		return &RetryPolicy{}
	} else if !policy.Enabled {
		return policy
	}

	if policy.NumOfRetries <= 0 {
		policy.Enabled = false
		return policy
	}

	if len(policy.RetriableStatusCodes) == 0 {
		policy.RetriableStatusCodes = []int{
			int(codes.DeadlineExceeded),
		}
	}

	if policy.BackoffDuration == 0 {
		policy.BackoffDuration = 500 * time.Millisecond
	}

	return policy
}

func (s *serverControlPlaneWrapper) TimeoutPolicy(serviceName, methodName string) *ServerTimeoutPolicy {
	if s.serverControlPlane == nil {
		return &ServerTimeoutPolicy{}
	}

	policy := s.serverControlPlane.TimeoutPolicy(serviceName, methodName)

	if policy == nil {
		return &ServerTimeoutPolicy{}
	} else if !policy.Enabled {
		return policy
	}

	return policy
}

func (s *serverControlPlaneWrapper) CircuitBreakerPolicy(serviceName, methodName string) *CircuitBreakerPolicy {
	if s.serverControlPlane == nil {
		return &CircuitBreakerPolicy{}
	}

	policy := s.serverControlPlane.CircuitBreakerPolicy(serviceName, methodName)

	if policy == nil {
		return &CircuitBreakerPolicy{}
	} else if !policy.Enabled {
		return policy
	}

	if policy.ErrorPercentThreshold <= 0 {
		policy.ErrorPercentThreshold = 50
	}

	if policy.RequestVolumeThreshold <= 0 {
		policy.RequestVolumeThreshold = 20
	}

	if policy.SleepWindow <= 0 {
		policy.SleepWindow = 5 * time.Second
	}

	if policy.MaxConcurrentRequests == 0 {
		policy.MaxConcurrentRequests = -1
	}

	if policy.RollingDurationInSec <= 0 {
		policy.RollingDurationInSec = 10
	}

	if len(policy.FailureStatusCode) == 0 {
		policy.FailureStatusCode = []int{
			int(codes.Unknown),
			int(codes.DeadlineExceeded),
			int(codes.ResourceExhausted),
			int(codes.Internal),
			int(codes.Unavailable),
		}
	}
	sort.Ints(policy.FailureStatusCode)

	return policy
}

func (s *serverControlPlaneWrapper) RateLimitPolicy(serviceName, methodName string) *RateLimitPolicy {
	if s.serverControlPlane == nil {
		return &RateLimitPolicy{}
	}

	policy := s.serverControlPlane.RateLimitPolicy(serviceName, methodName)

	if policy == nil {
		return &RateLimitPolicy{}
	} else if !policy.Enabled {
		return policy
	}

	if policy.MaxRequestPerUnit == 0 {
		policy.MaxRequestPerUnit = -1
	}

	if policy.MaxSendMsgPerUnit == 0 {
		policy.MaxSendMsgPerUnit = -1
	}

	if policy.MaxRecvMsgPerUnit == 0 {
		policy.MaxRecvMsgPerUnit = -1
	}

	if policy.Unit == 0 {
		policy.Unit = time.Second
	}

	return policy
}
