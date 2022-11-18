package healthcheck

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// ServerHealthChecker struct
type ServerHealthChecker struct {
	checkFunc checkFunction
}

var defaultServicePort, envKey string

// NewServerHealthChecker function generates a ServerHealthChecker struct
// You can specify a custom readiness function in checkFunc parameter
func NewServerHealthChecker(port string, key string, checkFunc checkFunction) *ServerHealthChecker {
	defaultServicePort = port
	envKey = key
	return &ServerHealthChecker{checkFunc: checkFunc}
}

// Check is a method function on ServerHealthChecker struct
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
		// no connection created -> not ready
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
	if conn != nil {
		defer func() {
			if err := conn.Close(); err != nil {
				log.Printf("error closing connection: %s", err.Error())
			}
		}()
		// execute custom readiness function if provided
		if s.checkFunc != nil {
			err := s.checkFunc()
			if err != nil {
				// error -> not ready
				return &grpc_health_v1.HealthCheckResponse{
					Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
				}, nil
			}
			// no error -> ready
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_SERVING,
			}, nil
		}
		// if custom readiness function not provided, consider server ready
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

// Watch is a method function on ServerHealthChecker struct
// Method is currently not implemented - throws grpc status error
func (s *ServerHealthChecker) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	// Not implemented
	// The method only returns error/nil - can not reflect ready and liviness respectively
	return status.Error(codes.Unimplemented, "unimplemented")
}
