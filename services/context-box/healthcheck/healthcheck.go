package healthcheck

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc/health/grpc_health_v1"
)

type HealthChecker struct{}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{}
}

func (s *HealthChecker) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {

	fmt.Println("Serving the Check request for health check")

	// Check if app is ready -> if true, return SERVING
	timeout := 5 * time.Second
	contextboxPort := os.Getenv("CONTEXT_BOX_PORT")

	if contextboxPort == "" {
		contextboxPort = "50055" // Default value
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", contextboxPort), timeout)
	if err != nil {
		fmt.Println("Readiness probe check: ERROR")
		fmt.Println(err)
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
	if conn != nil {
		defer conn.Close()
		fmt.Println("Readiness probe check: OK")
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	}

	// Return answer NOT_SERVING -> app is live but not ready
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	}, nil
}

func (s *HealthChecker) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {

	fmt.Println("Serving the Watch request for health check")

	// Check if app is ready
	timeout := 5 * time.Second
	contextboxPort := os.Getenv("CONTEXT_BOX_PORT")

	if contextboxPort == "" {
		contextboxPort = "50055" // Default value
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", contextboxPort), timeout)
	if err != nil {
		fmt.Println("Readiness probe check: ERROR")
		fmt.Println(err)
		return err
	}
	if conn != nil {
		defer conn.Close()
		fmt.Println("Readiness probe check: OK")
		// No error, everything is okay
		return nil
	}

	return nil //TODO: should reflect that service is live but not ready
}
