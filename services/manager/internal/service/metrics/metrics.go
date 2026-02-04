package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: metrics on nats.
var (
	NatsDuplicateMessagesCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_nats_messages_duplicate",
		Help: "Total number of duplicate messages that were scheduled",
	})
	NatsMsgsAcknowledged = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_nats_messages_acknowledged",
		Help: "Total number of acknowledged messages from the NATS queue",
	})
	NatsMsgsUnAcknowledged = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_nats_messages_unacknowledged",
		Help: "Total number of un-acknowledged messages from the NATS queue",
	})
	TaskClearResultCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_clear_result",
		Help: "Total number of task results processed that are clearing state",
	})
	TaskUpdateResultCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_update_result",
		Help: "Total number of task results processed that are updating state",
	})
	TaskNoneResultCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_none_result",
		Help: "Total number of task results processed that have no side effect",
	})
	TaskUnknownResultCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_unknown_result",
		Help: "Total number of unknown task results processed",
	})
	TasksScheduled = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_scheduled",
		Help: "Total number of tasks scheduled for NATS queue to work on",
	})
	TasksFinishedOk = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_completed",
		Help: "Total number of tasks completed by processing the result and updating the state",
	})
	TasksFinishedErr = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_errored",
		Help: "Total number of tasks errored while processing",
	})
	TasksProcessedTypeUnknownCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_type_unknown",
		Help: "Total number of tasks processed with event type 'unknown' or that are unrecognized by claudie",
	})
	TasksProcessedTypeCreateCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_type_create",
		Help: "Total number of tasks processed with event type 'create'",
	})
	TasksProcessedTypeUpdateCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_type_update",
		Help: "Total number of tasks processed with event type 'update'",
	})
	TasksProcessedTypeDeleteCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_tasks_type_delete",
		Help: "Total number of tasks processed with event type 'delete'",
	})
)

func MustRegisterCounters() {
	prometheus.MustRegister(NatsDuplicateMessagesCounter)
	prometheus.MustRegister(NatsMsgsAcknowledged)
	prometheus.MustRegister(NatsMsgsUnAcknowledged)
	prometheus.MustRegister(TaskClearResultCounter)
	prometheus.MustRegister(TaskUpdateResultCounter)
	prometheus.MustRegister(TaskNoneResultCounter)
	prometheus.MustRegister(TaskUnknownResultCounter)
	prometheus.MustRegister(TasksScheduled)
	prometheus.MustRegister(TasksFinishedOk)
	prometheus.MustRegister(TasksFinishedErr)
	prometheus.MustRegister(TasksProcessedTypeUnknownCounter)
	prometheus.MustRegister(TasksProcessedTypeCreateCounter)
	prometheus.MustRegister(TasksProcessedTypeUpdateCounter)
	prometheus.MustRegister(TasksProcessedTypeDeleteCounter)
}
