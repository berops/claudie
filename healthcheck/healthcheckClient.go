package healthcheck

import (
	"fmt"
	"net/http"

	"github.com/Berops/platform/proto/pb"
)

type ClientHealthChecker struct{}
type functionToCheck func(config *pb.Config) *pb.Config

var fnToCheck functionToCheck
var portForProbes string

func NewClientHealthChecker(port string) *ClientHealthChecker {
	portForProbes = port
	return &ClientHealthChecker{}
}

func (s *ClientHealthChecker) StartProbes(fn functionToCheck) {
	fnToCheck = fn
	http.HandleFunc("/live", live)
	http.HandleFunc("/ready", ready)
	// Port close to other services
	go http.ListenAndServe("0.0.0.0"+portForProbes, nil)
}

func live(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "200 OK \n")
}

func ready(w http.ResponseWriter, req *http.Request) {
	result := fnToCheck(nil)
	if result == nil {
		fmt.Fprintf(w, "200 OK \n")
		return
	}
	fmt.Fprintf(w, "300 NOT READY \n")
}
