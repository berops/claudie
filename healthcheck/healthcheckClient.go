package healthcheck

import (
	"fmt"
	"net/http"
)

type checkFunction func() error
type ClientHealthChecker struct{}

var portForProbes string
var checkFunc checkFunction

// Will initilize new healthchecks
func NewClientHealthChecker(port string, f checkFunction) *ClientHealthChecker {
	portForProbes = port
	checkFunc = f
	return &ClientHealthChecker{}
}

// StartProbes will initilize http endpoints for liviness (/live) and readiness (/ready) checks
func (s *ClientHealthChecker) StartProbes() {
	http.HandleFunc("/live", live)
	http.HandleFunc("/ready", ready)
	// Port close to other services
	fmt.Println("0.0.0.0:" + portForProbes)
	go http.ListenAndServe("0.0.0.0:"+portForProbes, nil)
	//fmt.Println(id)
}

// live function is testing liviness state of the microservice
// always return 200 -> if microservice is able to respond, it is live
func live(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "200 OK \n")
}

// ready function is testing readiness state of the microservice
// uses checkFunction provided in NewClientHealthChecker -> if no error thrown, microservice is ready
func ready(w http.ResponseWriter, req *http.Request) {
	result := checkFunc()
	if result != nil {
		fmt.Fprintf(w, "200 OK \n")
		return
	}
	fmt.Println(result)
	fmt.Fprintf(w, "300 NOT READY \n")
}
