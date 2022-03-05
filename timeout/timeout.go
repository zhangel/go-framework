package timeout

import (
	"context"
	"fmt"

	"github.com/zhangel/go-framework/control"
	"github.com/zhangel/go-framework/hooks"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/registry"
	"github.com/zhangel/go-framework/utils"

	cmap "github.com/orcaman/concurrent-map"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var lastStatus = cmap.New()

const (
	typeClient = "client"
	typeServer = "server"
)

type timeoutPolicy interface {
	TimeoutPolicy() *control.TimeoutPolicy
}

type timeoutPolicyWrapper struct {
	policy *control.TimeoutPolicy
}

func (s timeoutPolicyWrapper) TimeoutPolicy() *control.TimeoutPolicy {
	return s.policy
}

func timeoutHandler(typ string, ctx context.Context, service, method string, controlPlane func() (timeoutPolicy, error), handler func(ctx context.Context) (err error)) (err error) {
	cp, err := controlPlane()
	if err != nil {
		return handler(ctx)
	}

	policy := cp.TimeoutPolicy()
	key := typ + ":" + service + "/" + method
	if st, ok := lastStatus.Get(key); !ok || st.(bool) != policy.Enabled {
		if policy.Enabled {
			log.Infof("timeout_handler: timeout handler of %q enabled, timeout = %v.", key, policy.Timeout)
		} else if ok {
			log.Infof("timeout_handler: timeout handler of %q disabled.", key)
		}
		lastStatus.Set(key, policy.Enabled)
	}

	if !policy.Enabled || policy.Timeout == 0 {
		return handler(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, policy.Timeout)
	defer cancel()

	ch := make(chan error)
	go func() {
		ch <- handler(ctx)
	}()

	select {
	case <-ctx.Done():
		return status.New(codes.DeadlineExceeded, fmt.Sprintf("timeout_handler: deadline of %q[%v] is exceeded", key, policy.Timeout)).Err()
	case err := <-ch:
		return err
	}
}

func UnaryClientInterceptor(targetTransformer registry.TargetTransformer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, fullMethod string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		service, method := utils.ClientServiceMethodName(targetTransformer(cc.Target()), fullMethod)
		err := timeoutHandler(typeClient, ctx, service, method, func() (timeoutPolicy, error) {
			cp, err := control.DefaultControlPlane().ClientControlPlane(service)
			if err != nil {
				return nil, err
			} else {
				return &timeoutPolicyWrapper{policy: &cp.TimeoutPolicy(service, method).TimeoutPolicy}, nil
			}
		}, func(ctx context.Context) (err error) {
			err = invoker(ctx, fullMethod, req, reply, cc, opts...)
			return err
		})
		return err
	}
}

func StreamClientInterceptor(targetTransformer registry.TargetTransformer) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, fullMethod string, streamer grpc.Streamer, opts ...grpc.CallOption) (stream grpc.ClientStream, err error) {
		service, method := utils.ClientServiceMethodName(targetTransformer(cc.Target()), fullMethod)
		err = timeoutHandler(typeClient, ctx, service, method, func() (timeoutPolicy, error) {
			cp, err := control.DefaultControlPlane().ClientControlPlane(service)
			if err != nil {
				return nil, err
			} else {
				return &timeoutPolicyWrapper{policy: &cp.TimeoutPolicy(service, method).TimeoutPolicy}, nil
			}
		}, func(ctx context.Context) (err error) {
			stream, err = streamer(ctx, desc, cc, fullMethod, opts...)
			return err
		})
		return stream, err
	}
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		service, method := utils.ServerServiceMethodName(ctx, info.FullMethod)
		err = timeoutHandler(typeServer, ctx, service, method, func() (timeoutPolicy, error) {
			cp, err := control.DefaultControlPlane().ServerControlPlane(nil, service)
			if err != nil {
				return nil, err
			} else {
				return &timeoutPolicyWrapper{policy: &cp.TimeoutPolicy(service, method).TimeoutPolicy}, nil
			}
		}, func(ctx context.Context) (err error) {
			resp, err = handler(ctx, req)
			return err
		})
		return resp, err
	}
}

func UnaryServerHookInterceptor() hooks.UnaryHookInterceptor {
	return func(srv interface{}, ctx context.Context, service, method string, handler hooks.UnaryHookHandler) (resp interface{}, err error) {
		err = timeoutHandler(typeServer, ctx, service, method, func() (timeoutPolicy, error) {
			cp, err := control.DefaultControlPlane().ServerControlPlane(srv, service)
			if err != nil {
				return nil, err
			} else {
				return &timeoutPolicyWrapper{policy: &cp.TimeoutPolicy(service, method).TimeoutPolicy}, nil
			}
		}, func(ctx context.Context) (err error) {
			resp, err = handler(srv, ctx)
			return err
		})
		return resp, err
	}
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		service, method := utils.ServerServiceMethodName(ss.Context(), info.FullMethod)
		err := timeoutHandler(typeServer, ss.Context(), service, method, func() (timeoutPolicy, error) {
			cp, err := control.DefaultControlPlane().ServerControlPlane(nil, service)
			if err != nil {
				return nil, err
			} else {
				return &timeoutPolicyWrapper{policy: &cp.TimeoutPolicy(service, method).TimeoutPolicy}, nil
			}
		}, func(ctx context.Context) (err error) {
			return handler(srv, &streamWrapper{ctx, ss})
		})
		return err
	}
}

func StreamServerHookInterceptor() hooks.StreamHookInterceptor {
	return func(srv interface{}, service, method string, stream grpc.ServerStream, handler hooks.StreamHookHandler) error {
		err := timeoutHandler(typeServer, stream.Context(), service, method, func() (timeoutPolicy, error) {
			cp, err := control.DefaultControlPlane().ServerControlPlane(srv, service)
			if err != nil {
				return nil, err
			} else {
				return &timeoutPolicyWrapper{policy: &cp.TimeoutPolicy(service, method).TimeoutPolicy}, nil
			}
		}, func(ctx context.Context) (err error) {
			return handler(srv, &streamWrapper{ctx, stream})
		})
		return err
	}
}

type streamWrapper struct {
	ctx context.Context
	ss  grpc.ServerStream
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
	return s.ctx
}

func (s *streamWrapper) SendMsg(m interface{}) error {
	return s.ss.SendMsg(m)
}

func (s *streamWrapper) RecvMsg(m interface{}) error {
	return s.ss.RecvMsg(m)
}
