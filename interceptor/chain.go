package interceptor

import (
	"context"

	"google.golang.org/grpc"
)

func ChainUnaryClient(interceptors ...grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		for i := len(interceptors) - 1; i >= 0; i-- {
			func(unaryInt grpc.UnaryClientInterceptor) {
				prevInvoker := invoker
				invoker = func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
					return unaryInt(ctx, method, req, reply, cc, prevInvoker, opts...)
				}
			}(interceptors[i])
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func ChainStreamClient(interceptors ...grpc.StreamClientInterceptor) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (stream grpc.ClientStream, err error) {
		for i := len(interceptors) - 1; i >= 0; i-- {
			func(streamInt grpc.StreamClientInterceptor) {
				prevStreamer := streamer
				streamer = func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
					return streamInt(ctx, desc, cc, method, prevStreamer, opts...)
				}
			}(interceptors[i])
		}

		return streamer(ctx, desc, cc, method, opts...)
	}
}

func ChainUnaryServer(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		for i := len(interceptors) - 1; i >= 0; i-- {
			func(unaryInt grpc.UnaryServerInterceptor) {
				prevHandler := handler
				handler = func(ctx context.Context, req interface{}) (resp interface{}, err error) {
					return unaryInt(ctx, req, info, prevHandler)
				}
			}(interceptors[i])
		}

		return handler(ctx, req)
	}
}

func ChainStreamServer(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		for i := len(interceptors) - 1; i >= 0; i-- {
			func(streamInt grpc.StreamServerInterceptor) {
				prevHandler := handler
				handler = func(srv interface{}, stream grpc.ServerStream) error {
					return streamInt(srv, stream, info, prevHandler)
				}
			}(interceptors[i])
		}

		return handler(srv, stream)
	}
}
