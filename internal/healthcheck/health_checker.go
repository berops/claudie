package healthcheck

type HealthChecker interface {
	// HealthCheck checks whether the underlying connection is still ongoing.
	HealthCheck() error
}
