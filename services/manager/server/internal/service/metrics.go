package service

import (
	"github.com/prometheus/client_golang/prometheus"
)

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

	TasksInQueue = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_input_manifests_tasks_enqueued",
		Help: "Number of input manifests tasks currently enqueued for builder service",
	})
)

func MustRegisterCounters() {
	prometheus.MustRegister(TasksScheduled)
	prometheus.MustRegister(TasksInQueue)
	prometheus.MustRegister(TasksFinishedOk)
	prometheus.MustRegister(TasksFinishedErr)
}
