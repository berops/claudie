package healthcheck

import (
	"fmt"
	"net/http"
)

// Function to check the readiness
type checkFunction func() error

type ClientHealthChecker struct {
	portForProbes string
	checkFunc     checkFunction
}

// NewClientHealthChecker fucntion will return new struct with
func NewClientHealthChecker(port string, f checkFunction) *ClientHealthChecker {
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
	go http.ListenAndServe("0.0.0.0:"+s.portForProbes, nil)
}

// live function is testing liviness state of the microservice
// always return 200 -> if microservice is able to respond, it is live
func live(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Serving the Check request for liviness check")
	fmt.Println("Liviness probe check: OK")
	w.WriteHeader(500)
	w.Write([]byte("ok"))
}

// ready function is testing readiness state of the microservice
// uses checkFunction provided in ClientHealthChecker -> if no error thrown, microservice is ready
func (s *ClientHealthChecker) ready(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Serving the Check request for readiness check")
	result := s.checkFunc()
	if result != nil {
		fmt.Println(result)
		fmt.Println("Readiness probe check: ERROR")
		w.WriteHeader(500)
		w.Write([]byte("not ready"))
		return
	}
	fmt.Println("Readiness probe check: OK")
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}
