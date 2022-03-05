package grpc_server

import (
	"github.com/zhangel/go-framework/server/internal/grpc_reflection"
	"google.golang.org/grpc"

	"github.com/zhangel/go-framework/credentials"
	"github.com/zhangel/go-framework/deadline"
	"github.com/zhangel/go-framework/interceptor"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/prometheus"
	"github.com/zhangel/go-framework/recovery"
	"github.com/zhangel/go-framework/server/internal/option"
	"github.com/zhangel/go-framework/tracing"
)

func PrepareUnaryInterceptors(options *option.Options, ignoreInternalInterceptors bool) grpc.UnaryServerInterceptor {
	result := options.UnaryInterceptors

	if options.Tracer != nil {
		result = append(result, tracing.OpenTracingServerInterceptor(options.Tracer))
	}

	if options.Recovery {
		result = append(result, recovery.UnaryServerInterceptor())
	}

	if options.AuthProvider != nil {
		if unaryInt := options.AuthProvider.UnaryAuthInterceptor(); unaryInt != nil {
			result = append(result, unaryInt)
		}
	}

	if unaryInt := prometheus.UnaryServerInterceptor(); unaryInt != nil {
		result = append(result, unaryInt)
	}

	if !ignoreInternalInterceptors {
		result = append(result, deadline.DeadlineCheckerUnaryServerInterceptor)
		result = append(result, tracing.RequestIdTracingUnaryServerInterceptor)
	}

	if len(result) > 0 {
		return interceptor.ChainUnaryServer(result...)
	} else {
		return nil
	}
}

func PrepareStreamInterceptors(options *option.Options, ignoreInternalInterceptors bool) grpc.StreamServerInterceptor {
	result := options.StreamInterceptors

	if options.Tracer != nil {
		result = append(result, tracing.OpenTracingStreamServerInterceptor(options.Tracer))
	}

	if options.Recovery {
		result = append(result, recovery.StreamServerInterceptor())
	}

	if options.AuthProvider != nil {
		if streamInt := options.AuthProvider.StreamAuthInterceptor(); streamInt != nil {
			result = append(result, streamInt)
		}
	}

	if options.GrpcServerReflection && options.GrpcServerReflectionAuth != nil {
		result = append(result, grpc_reflection.GrpcReflectionAuthInterceptor(options.GrpcServerReflectionAuth))
	}

	if streamInt := prometheus.StreamServerInterceptor(); streamInt != nil {
		result = append(result, streamInt)
	}

	if !ignoreInternalInterceptors {
		result = append(result, tracing.RequestIdTracingStreamingServerInterceptor)
	}
	if len(result) > 0 {
		return interceptor.ChainStreamServer(result...)
	} else {
		return nil
	}
}

func PrepareGrpcOptions(opts *option.Options) []grpc.ServerOption {
	unaryInt := PrepareUnaryInterceptors(opts, opts.IgnoreInternalInterceptors)
	streamInt := PrepareStreamInterceptors(opts, opts.IgnoreInternalInterceptors)
	grpcOpts := append([]grpc.ServerOption{}, grpc.UnaryInterceptor(unaryInt), grpc.StreamInterceptor(streamInt))

	if credentialOpt, err := credentials.ServerOption(opts.CredentialOpts...); err != nil {
		log.Fatal("[ERROR]", err)
	} else if credentialOpt != nil {
		opts.WithTls = true
		grpcOpts = append(grpcOpts, credentialOpt)
	} else {
		opts.WithTls = false
	}

	if opts.MaxConcurrentStreams != 0 {
		grpcOpts = append(grpcOpts, grpc.MaxConcurrentStreams(opts.MaxConcurrentStreams))
	}

	if opts.MaxRecvMsgSize != 0 {
		grpcOpts = append(grpcOpts, grpc.MaxRecvMsgSize(opts.MaxRecvMsgSize))
	}

	if opts.MaxSendMsgSize != 0 {
		grpcOpts = append(grpcOpts, grpc.MaxSendMsgSize(opts.MaxSendMsgSize))
	}

	if opts.KeepAliveParams != nil {
		grpcOpts = append(grpcOpts, grpc.KeepaliveParams(*opts.KeepAliveParams))
	}

	if opts.KeepAliveEnforcementPolicy != nil {
		grpcOpts = append(grpcOpts, grpc.KeepaliveEnforcementPolicy(*opts.KeepAliveEnforcementPolicy))
	}

	return grpcOpts
}
