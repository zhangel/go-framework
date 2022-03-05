package connection

import (
	"github.com/zhangel/go-framework.git/hooks"

	"google.golang.org/grpc"
)

func MakeConnection(target string, unaryInt grpc.UnaryClientInterceptor, streamInt grpc.StreamClientInterceptor) (cc *grpc.ClientConn, err error) {
	return hooks.MakeConnection(target, unaryInt, streamInt)
}

func IsMemoryConnection(cc *grpc.ClientConn) bool {
	return hooks.IsMemoryConnection(cc)
}
