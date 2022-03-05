package authentication

import "google.golang.org/grpc"

type noopAuthentication struct {
}

func (s noopAuthentication) UnaryAuthInterceptor() grpc.UnaryServerInterceptor {
	return nil
}

func (s noopAuthentication) StreamAuthInterceptor() grpc.StreamServerInterceptor {
	return nil
}
