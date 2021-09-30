package healthcheck

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type ServerHealthChecker struct{}

var defaultServicePort, envKey string

func NewServerHealthChecker(port, key string) *ServerHealthChecker {
	defaultServicePort = port
	envKey = key
	return &ServerHealthChecker{}
}

func (s *ServerHealthChecker) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {

	// Check if app is ready -> if true, return SERVING
	timeout := 5 * time.Second
	servicePort := os.Getenv(envKey)

	if servicePort == "" {
		servicePort = defaultServicePort // Default value
	}

	// Check if port is open -> use 0.0.0.0 since we are in a same pod
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("0.0.0.0", servicePort), timeout)
	if err != nil {
		fmt.Println("gRPC server status: NOT_SERVING")
		fmt.Println(err)
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
	if conn != nil {
		defer conn.Close()
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	}

	// Return answer NOT_SERVING -> app is live but not ready
	fmt.Println("gRPC server status: NOT_SERVING")
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	}, nil
}

func (s *ServerHealthChecker) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	// Unimplemented
	// The method only returns error/nil - can not reflect ready and liviness respectively
	return status.Error(codes.Unimplemented, "unimplemented")
}
