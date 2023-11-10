package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	InputManifestsProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_processed",
		Help: "Counter for processed input manifests",
	})
	InputManifestsErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_err",
		Help: "Counter for the errors occurred during processing of Input Manifests",
	})
	InputManifestsInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_input_manifests_in_progress",
		Help: "Number of input manifests currently processed by scheduler",
	})
)

func MustRegisterCounters() {
	prometheus.MustRegister(InputManifestsProcessedCounter)
	prometheus.MustRegister(InputManifestsErrorCounter)
	prometheus.MustRegister(InputManifestsInProgress)
}
