package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

const (
	MethodLabel = "method"
)

var (
	RequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "claudie_grpc_request_in_flight",
			Help: "Number of grpc requests currently handled",
		},
	)

	RequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claudie_grpc_request_count",
			Help: "Total number of gRPC API calls.",
		},
		[]string{MethodLabel},
	)

	Latency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "claudie__grpc_call_latency_seconds",
			Help: "Latency of gRPC API calls in seconds.",
			Buckets: []float64{
				.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, // up to 10sec
				20, 30, 60, 300, 600, // up to 10 min
				1200, 1800, 2700, 3600, // up to 1hour
			},
		},
		[]string{MethodLabel},
	)

	ErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_error_count",
			Help: "Total number of gRPC API call errors.",
		},
		[]string{MethodLabel},
	)
)

func MetricsMiddleware(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	RequestsInFlight.Inc()

	now := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(now)

	RequestCount.WithLabelValues(info.FullMethod).Inc()
	Latency.WithLabelValues(info.FullMethod).Observe(duration.Seconds())

	if err != nil {
		ErrorCount.WithLabelValues(info.FullMethod).Inc()
	}

	RequestsInFlight.Dec()
	return resp, err
}

func MustRegisterCounters() {
	prometheus.MustRegister(RequestCount)
	prometheus.MustRegister(Latency)
	prometheus.MustRegister(ErrorCount)
	prometheus.MustRegister(RequestsInFlight)
}
