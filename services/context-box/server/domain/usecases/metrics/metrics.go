package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	InputManifestsEnqueuedScheduler = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_enqueued_scheduler",
		Help: "Number of input manifests enqueued for scheduler service",
	})

	InputManifestsEnqueuedBuilder = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_enqueued_builder",
		Help: "Number of input manifests enqueued for builder service",
	})
)

func MustRegisterCounters() {
	prometheus.MustRegister(InputManifestsEnqueuedScheduler)
	prometheus.MustRegister(InputManifestsEnqueuedBuilder)
}
