package control

import (
	"context"
	"sync"
	"time"

	"github.com/zhangel/go-framework.git/declare"
	"github.com/zhangel/go-framework.git/lifecycle"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/plugin"
)

var (
	Plugin = declare.PluginType{Name: "control"}

	mu                  sync.RWMutex
	defaultControlPlane Factory
)

type Policy struct {
	Enabled bool
	Version int64
}

type RetryPolicy struct {
	Policy
	NumOfRetries         uint
	PerTryTimeout        time.Duration
	RetriableStatusCodes []int
	Overlap              bool
	BackoffDuration      time.Duration
	BackoffJitter        float64
	BackoffExponential   bool
}

type TimeoutPolicy struct {
	Policy
	Timeout time.Duration
}

type ClientTimeoutPolicy struct {
	TimeoutPolicy
	ConnTimeout time.Duration
}

type ServerTimeoutPolicy struct {
	TimeoutPolicy
}

type CircuitBreakerPolicy struct {
	Policy
	Timeout                time.Duration
	MaxConcurrentRequests  int
	RequestVolumeThreshold int
	SleepWindow            time.Duration
	ErrorPercentThreshold  int
	FailureStatusCode      []int
	RollingDurationInSec   int
}

type RateLimitPolicy struct {
	Policy
	MaxRequestPerUnit int
	MaxSendMsgPerUnit int
	MaxRecvMsgPerUnit int
	Unit              time.Duration
}

type ClientControlPlane interface {
	TimeoutPolicy(serviceName, methodName string) *ClientTimeoutPolicy
	RetryPolicy(serviceName, methodName string) *RetryPolicy
}

type ServerControlPlane interface {
	TimeoutPolicy(serviceName, methodName string) *ServerTimeoutPolicy
	CircuitBreakerPolicy(serviceName, methodName string) *CircuitBreakerPolicy
	RateLimitPolicy(serviceName, methodName string) *RateLimitPolicy
}

type Factory interface {
	ClientControlPlane(service string) (ClientControlPlane, error)
	ServerControlPlane(srv interface{}, service string) (ServerControlPlane, error)
}

type FactoryReleaser interface {
	Release()
}

func init() {
	lifecycle.LifeCycle().HookFinalize(func(context.Context) {
		mu.RLock()
		defer mu.RUnlock()

		if releaser, ok := defaultControlPlane.(FactoryReleaser); ok && releaser != nil {
			releaser.Release()
		}
	}, lifecycle.WithName("Close default logger"), lifecycle.WithPriority(lifecycle.PriorityLowest))
}

func DefaultControlPlane() Factory {
	mu.RLock()
	if defaultControlPlane != nil {
		mu.RUnlock()
		return defaultControlPlane
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if defaultControlPlane != nil {
		return defaultControlPlane
	}

	err := plugin.CreatePlugin(Plugin, &defaultControlPlane)
	if err != nil {
		log.Fatalf("[ERROR] Create control plane plugin failed, err = %s.", err)
	}

	defaultControlPlane = &wrapper{defaultControlPlane}

	return defaultControlPlane
}
