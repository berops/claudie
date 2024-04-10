package healthcheck

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ClientHealthChecker contains the port and check function callback
type ClientHealthChecker struct {
	portForProbes string
	checkFunc     func() error
}

// NewClientHealthChecker function will return new ClientHealthChecker struct with specified port and checkFunction
func NewClientHealthChecker(port string, f func() error) *ClientHealthChecker {
	return &ClientHealthChecker{
		portForProbes: port,
		checkFunc:     f,
	}
}

// StartProbes will initilize http endpoint for(/health) check
func (s *ClientHealthChecker) StartProbes() {
	http.HandleFunc("/health", s.health)
	// Port close to other services
	go func() {
		if err := http.ListenAndServe(net.JoinHostPort("0.0.0.0", s.portForProbes), nil); err != nil {
			log.Debug().Msgf("Error in health probe : %v", err)
		}
	}()
}

func writeMsg(w http.ResponseWriter, msg string) {
	if _, err := w.Write([]byte(msg)); err != nil {
		log.Debug().Msgf("HealthCheckClient write error: %v", err)
	}
}

// ready function is testing readiness state of the microservice
// uses checkFunction provided in ClientHealthChecker -> if no error thrown, microservice is ready
func (s *ClientHealthChecker) health(w http.ResponseWriter, req *http.Request) {
	err := s.checkFunc()
	if err != nil {
		log.Debug().Msgf("Error in health probe: %v", err)
		w.WriteHeader(500)
		writeMsg(w, "not ready")
		return
	}
	w.WriteHeader(200)
	writeMsg(w, "ok")
}

type HealthCheck struct {
	Ping             func() error
	ServiceName      string
	timeSinceFailure *time.Time
}

type HealthChecker struct {
	services []HealthCheck
	logger   *zerolog.Logger
	lock     sync.Mutex
}

func NewHealthCheck(logger *zerolog.Logger, interval time.Duration, services []HealthCheck) *HealthChecker {
	hc := &HealthChecker{
		services: services,
		logger:   logger,
	}

	hc.check() // perform initial check

	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			hc.check()
		}
	}()

	return hc
}

func (c *HealthChecker) check() {
	c.lock.Lock()
	defer c.lock.Unlock()

	updateTimeSinceFailure := func(n string, now *time.Time, t **time.Time, err error) {
		if err == nil {
			if *t != nil {
				c.logger.Debug().Msgf("service:%v is healthy again", n)
				*t = nil
			}
			return
		}
		if *t == nil {
			c.logger.Debug().Msgf("service:%v return error in ping: %v", n, err)
			*t = now
		}
	}

	now := time.Now()
	for i := range c.services {
		updateTimeSinceFailure(c.services[i].ServiceName, &now, &c.services[i].timeSinceFailure, c.services[i].Ping())
	}
}

func (c *HealthChecker) CheckForFailures() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error
	for _, svc := range c.services {
		err = c.checkFailure(svc.timeSinceFailure, svc.ServiceName, err)
	}
	return err
}

func (c *HealthChecker) checkFailure(t *time.Time, service string, perr error) error {
	if t != nil && time.Since(*t) >= 4*time.Minute {
		if perr != nil {
			return fmt.Errorf("%w; %s is unhealthy", perr, service)
		}
		return fmt.Errorf("%s is unhealthy", service)
	}
	return perr
}

func (c *HealthChecker) AnyServiceUnhealthy() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	ok := false
	for _, svc := range c.services {
		ok = ok || (svc.timeSinceFailure != nil)
	}

	return ok
}
