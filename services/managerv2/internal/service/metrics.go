package service

import (
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: adjust metrics for the NATS queue also.
var (
	TasksScheduled = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_tasks_scheduled",
		Help: "Total number of tasks scheduled for builder service to work on",
	})

	TasksFinishedOk = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_tasks_completed",
		Help: "Total number of tasks completed by the builder service",
	})

	TasksFinishedErr = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_tasks_errored",
		Help: "Total number of tasks errored while processing by the builder service",
	})
)

func MustRegisterCounters() {
	prometheus.MustRegister(TasksScheduled)
	prometheus.MustRegister(TasksFinishedOk)
	prometheus.MustRegister(TasksFinishedErr)
}
