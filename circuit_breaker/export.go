package circuit_breaker

import (
	"context"
	"fmt"

	"github.com/zhangel/go-framework.git/control"
	"github.com/zhangel/go-framework.git/hooks"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/utils"

	"github.com/cep21/circuit/v3"
	cmap "github.com/orcaman/concurrent-map"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var lastStatus = cmap.New()

func circuitBreakerHandler(ctx context.Context, srv interface{}, service, method string, handler func(ctx context.Context) (err error)) (err error) {
	cp, err := control.DefaultControlPlane().ServerControlPlane(srv, service)
	if err != nil {
		return handler(ctx)
	}

	policy := cp.CircuitBreakerPolicy(service, method)
	key := service + "/" + method
	if st, ok := lastStatus.Get(key); !ok || st.(bool) != policy.Enabled {
		if policy.Enabled {
			log.Infof("circuit_breaker: circuit breaker of %q enabled.", key)
		} else if ok {
			log.Infof("circuit_breaker: circuit breaker of %q disabled.", key)
		}
		lastStatus.Set(key, policy.Enabled)
	}

	if !policy.Enabled {
		return handler(ctx)
	}

	err = circuitBreakerManager().CircuitWithPolicy(key, policy).Execute(ctx, func(ctx context.Context) (err error) {
		if err = handler(ctx); err == nil {
			return err
		} else if s, ok := status.FromError(err); ok {
			for _, code := range policy.FailureStatusCode {
				if uint32(code) == uint32(s.Code()) {
					return err
				}
			}

			return circuit.SimpleBadRequest{Err: err}
		} else {
			return err
		}
	}, func(ctx context.Context, err error) error {
		if _, ok := err.(circuit.Error); ok {
			return status.New(codes.ResourceExhausted, fmt.Sprintf("circuit_breaker: circuit breaker of %q is triggered, policy = %+v, err = %v", key, policy, err.Error())).Err()
		} else {
			return err
		}
	})

	if badRequestErr, ok := err.(circuit.SimpleBadRequest); ok {
		return badRequestErr.Err
	} else {
		return err
	}
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		service, method := utils.ServerServiceMethodName(ctx, info.FullMethod)
		err = circuitBreakerHandler(ctx, nil, service, method, func(ctx context.Context) (err error) {
			resp, err = handler(ctx, req)
			return err
		})
		return resp, err
	}
}

func UnaryServerHookInterceptor() hooks.UnaryHookInterceptor {
	return func(srv interface{}, ctx context.Context, service, method string, handler hooks.UnaryHookHandler) (resp interface{}, err error) {
		err = circuitBreakerHandler(ctx, srv, service, method, func(ctx context.Context) (err error) {
			resp, err = handler(srv, ctx)
			return err
		})
		return resp, err
	}
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		service, method := utils.ServerServiceMethodName(ss.Context(), info.FullMethod)
		return circuitBreakerHandler(ss.Context(), nil, service, method, func(ctx context.Context) (err error) {
			return handler(srv, ss)
		})
	}
}

func StreamServerHookInterceptor() hooks.StreamHookInterceptor {
	return func(srv interface{}, service, method string, stream grpc.ServerStream, handler hooks.StreamHookHandler) error {
		return circuitBreakerHandler(stream.Context(), srv, service, method, func(ctx context.Context) (err error) {
			return handler(srv, stream)
		})
	}
}
