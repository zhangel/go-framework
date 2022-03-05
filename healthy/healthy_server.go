package healthy

import (
	"context"
	"sync"
	"time"

	"github.com/zhangel/go-framework.git/log"

	"google.golang.org/grpc/health/grpc_health_v1"
)

type HealthService interface {
	grpc_health_v1.HealthServer
}

type DefaultHealthService interface {
	HealthService
	ChangeState(service string, state grpc_health_v1.HealthCheckResponse_ServingStatus)
}

type healthServiceImpl struct {
	mu     sync.RWMutex
	states map[string]grpc_health_v1.HealthCheckResponse_ServingStatus
}

func NewDefaultHealthService() DefaultHealthService {
	return &healthServiceImpl{
		states: make(map[string]grpc_health_v1.HealthCheckResponse_ServingStatus),
	}
}

func (s *healthServiceImpl) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if state, ok := s.states[req.GetService()]; ok {
		return &grpc_health_v1.HealthCheckResponse{
			Status: state,
		}, nil
	} else {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	}
}

func (s *healthServiceImpl) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if resp, err := s.Check(stream.Context(), req); err != nil {
				log.Infof("health service: check service %q health failed, err = %v", req.GetService(), err)
			} else if err := stream.Send(resp); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (s *healthServiceImpl) ChangeState(service string, state grpc_health_v1.HealthCheckResponse_ServingStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[service] = state
}
