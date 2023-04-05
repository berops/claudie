package healthcheck

import (
	"net"
	"net/http"

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

// StartProbes will initilize http endpoints for liviness (/live) and readiness (/ready) checks
func (s *ClientHealthChecker) StartProbes() {
	http.HandleFunc("/live", live)
	http.HandleFunc("/ready", s.ready)
	// Port close to other services
	go func() {
		if err := http.ListenAndServe(net.JoinHostPort("0.0.0.0", s.portForProbes), nil); err != nil {
			log.Debug().Msgf("Error in health probe : %v", err)
		}
	}()
}

func writeMsg(w http.ResponseWriter, msg string) {
	if _, err := w.Write([]byte(msg)); err != nil {
		log.Debug().Msgf("HealthCheckClient write error: ", err)
	}
}

// live function is testing liviness state of the microservice
// always return 200 -> if microservice is able to respond, it is live
func live(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	writeMsg(w, "ok")
}

// ready function is testing readiness state of the microservice
// uses checkFunction provided in ClientHealthChecker -> if no error thrown, microservice is ready
func (s *ClientHealthChecker) ready(w http.ResponseWriter, req *http.Request) {
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
