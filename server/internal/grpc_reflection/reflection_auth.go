package grpc_reflection

import (
	"bytes"
	"context"
	"encoding/base64"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func GrpcReflectionAuthInterceptor(authHandler []func(ctx context.Context) error) func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" && len(authHandler) != 0 {
			var err error
			for _, auth := range authHandler {
				if auth == nil {
					continue
				}

				if err = auth(ss.Context()); err == nil {
					return handler(srv, ss)
				}
			}

			return err
		} else {
			return handler(srv, ss)
		}
	}
}

func GrpcReflectionTokenAuth(token string) func (ctx context.Context) error {
	tokenByte := []byte(token)
	return func(ctx context.Context) error {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "no authorization header")
		}

		authorization := md.Get("Authorization")
		if len(authorization) == 0 {
			return status.Error(codes.Unauthenticated, "no authorization header")
		}

		if authHeader, err := base64.StdEncoding.DecodeString(authorization[0]); err !=nil || !bytes.Equal(authHeader, tokenByte) {
			return status.Error(codes.Unauthenticated, "invalid authorization token")
		}

		return nil
	}
}
