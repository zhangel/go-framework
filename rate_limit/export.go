package rate_limit

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/zhangel/go-framework/control"
	"github.com/zhangel/go-framework/hooks"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/utils"

	cmap "github.com/orcaman/concurrent-map"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var lastStatus = cmap.New()

type streamWrapper struct {
	srv     interface{}
	service string
	method  string
	ss      grpc.ServerStream
}

func (s *streamWrapper) SetHeader(md metadata.MD) error {
	return s.ss.SetHeader(md)
}

func (s *streamWrapper) SendHeader(md metadata.MD) error {
	return s.ss.SendHeader(md)
}

func (s *streamWrapper) SetTrailer(md metadata.MD) {
	s.ss.SetTrailer(md)
}

func (s *streamWrapper) Context() context.Context {
	return s.ss.Context()
}

func (s *streamWrapper) SendMsg(m interface{}) error {
	cp, err := control.DefaultControlPlane().ServerControlPlane(s.srv, s.service)
	if err != nil {
		return s.ss.SendMsg(m)
	}

	policy := cp.RateLimitPolicy(s.service, s.method)
	key := s.service + "/" + s.method
	if st, ok := lastStatus.Get(key); !ok || st.(bool) != policy.Enabled {
		if policy.Enabled {
			log.Infof("rate_limit: rate limit of %q enabled.", key)
		} else if ok {
			log.Infof("rate_limit: rate limit of %q disabled.", key)
		}
		lastStatus.Set(key, policy.Enabled)
	}

	if !policy.Enabled || policy.MaxSendMsgPerUnit <= 0 {
		return s.ss.SendMsg(m)
	}

	if rateLimiterManger().RateLimiterWithPolicy(sendMsgRateLimiter, key, policy, func(policy *control.RateLimitPolicy) int {
		return policy.MaxSendMsgPerUnit
	}).TakeAvailable(1) > 0 {
		return s.ss.SendMsg(m)
	} else {
		return status.Errorf(codes.ResourceExhausted, "stream msg send of %q is rejected by rate limiter, max_send_msg_per_unit = %d, unit = %v", key, policy.MaxSendMsgPerUnit, policy.Unit)
	}
}

func (s *streamWrapper) RecvMsg(m interface{}) error {
	cp, err := control.DefaultControlPlane().ServerControlPlane(s.srv, s.service)
	if err != nil {
		return s.ss.RecvMsg(m)
	}

	policy := cp.RateLimitPolicy(s.service, s.method)
	key := s.service + "/" + s.method
	if st, ok := lastStatus.Get(key); !ok || st.(bool) != policy.Enabled {
		if policy.Enabled {
			log.Infof("rate_limit: rate limit of %q enabled.", key)
		} else if ok {
			log.Infof("rate_limit: rate limit of %q disabled.", key)
		}
		lastStatus.Set(key, policy.Enabled)
	}

	if !policy.Enabled || policy.MaxRecvMsgPerUnit <= 0 {
		return s.ss.RecvMsg(m)
	}

	if rateLimiterManger().RateLimiterWithPolicy(recvMsgRateLimiter, key, policy, func(policy *control.RateLimitPolicy) int {
		return policy.MaxRecvMsgPerUnit
	}).TakeAvailable(1) > 0 {
		return s.ss.RecvMsg(m)
	} else {
		return status.Errorf(codes.ResourceExhausted, "stream msg receive of %q is rejected by rate limiter, max_recv_msg_per_unit = %d, unit = %v", key, policy.MaxRecvMsgPerUnit, policy.Unit)
	}
}

func requestRateLimitHandler(ctx context.Context, srv interface{}, service, method string, handler func(ctx context.Context) (err error)) (err error) {
	cp, err := control.DefaultControlPlane().ServerControlPlane(srv, service)
	if err != nil {
		return handler(ctx)
	}

	policy := cp.RateLimitPolicy(service, method)
	key := service + "/" + method
	if st, ok := lastStatus.Get(key); !ok || st.(bool) != policy.Enabled {
		if policy.Enabled {
			log.Infof("rate_limit: rate limit of %q enabled.", key)
		} else if ok {
			log.Infof("rate_limit: rate limit of %q disabled.", key)
		}
		lastStatus.Set(key, policy.Enabled)
	}

	if !policy.Enabled || policy.MaxRequestPerUnit <= 0 {
		return handler(ctx)
	}

	if rateLimiterManger().RateLimiterWithPolicy(requestRateLimiter, key, policy, func(policy *control.RateLimitPolicy) int {
		return policy.MaxRequestPerUnit
	}).TakeAvailable(1) > 0 {
		return handler(ctx)
	} else {
		return status.Errorf(codes.ResourceExhausted, "request of %q is rejected by rate limiter, max_request_per_unit = %d, unit = %v", key, policy.MaxRequestPerUnit, policy.Unit)
	}
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		service, method := utils.ServerServiceMethodName(ctx, info.FullMethod)
		err = requestRateLimitHandler(ctx, nil, service, method, func(ctx context.Context) (err error) {
			resp, err = handler(ctx, req)
			return err
		})
		return resp, err
	}
}

func UnaryServerHookInterceptor() hooks.UnaryHookInterceptor {
	return func(srv interface{}, ctx context.Context, service, method string, handler hooks.UnaryHookHandler) (resp interface{}, err error) {
		err = requestRateLimitHandler(ctx, srv, service, method, func(ctx context.Context) (err error) {
			resp, err = handler(srv, ctx)
			return err
		})
		return resp, err
	}
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		service, method := utils.ServerServiceMethodName(ss.Context(), info.FullMethod)
		return requestRateLimitHandler(ss.Context(), nil, service, method, func(ctx context.Context) (err error) {
			return handler(srv, &streamWrapper{nil, service, method, ss})
		})
	}
}

func StreamServerHookInterceptor() hooks.StreamHookInterceptor {
	return func(srv interface{}, service, method string, stream grpc.ServerStream, handler hooks.StreamHookHandler) error {
		return requestRateLimitHandler(stream.Context(), srv, service, method, func(ctx context.Context) (err error) {
			return handler(srv, &streamWrapper{srv, service, method, stream})
		})
	}
}
