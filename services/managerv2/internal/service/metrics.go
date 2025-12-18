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

	// TODO: handle the below metrics from builder here ?
	prometheus.MustRegister(TasksProcessedCounter)
	prometheus.MustRegister(TasksProcessedOkCounter)
	prometheus.MustRegister(TasksProcessedErrCounter)

	prometheus.MustRegister(TasksProcessedCreateCounter)
	prometheus.MustRegister(TasksProcessedUpdateCounter)
	prometheus.MustRegister(TasksProcessedDeleteCounter)

	prometheus.MustRegister(LoadBalancersProcessedCounter)
	prometheus.MustRegister(ClusterProcessedCounter)

	prometheus.MustRegister(ClustersInProgress)
	prometheus.MustRegister(LoadBalancersInProgress)

	prometheus.MustRegister(ClustersInCreate)
	prometheus.MustRegister(ClustersCreated)

	prometheus.MustRegister(ClustersInUpdate)
	prometheus.MustRegister(ClustersUpdated)

	prometheus.MustRegister(ClustersInDelete)
	prometheus.MustRegister(ClustersDeleted)

	prometheus.MustRegister(LBClustersInDeletion)
	prometheus.MustRegister(LBClustersDeleted)

	prometheus.MustRegister(K8sAddingNodesInProgress)
	prometheus.MustRegister(K8sDeletingNodesInProgress)
	prometheus.MustRegister(LbAddingNodesInProgress)
	prometheus.MustRegister(LbDeletingNodesInProgress)
}

const (
	InputManifestLabel = "claudie_input_manifest"
	K8sClusterLabel    = "claudie_k8s_cluster"
	LBClusterLabel     = "claudie_lb_cluster"
)

var (
	ClustersInDelete = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_in_delete",
		Help: "Clusters being deleted",
	})
	ClustersDeleted = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_delete",
		Help: "Clusters deleted",
	})

	ClustersInUpdate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_in_update",
		Help: "Clusters being updated",
	})
	ClustersUpdated = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_update",
		Help: "Clusters updated",
	})

	ClustersInCreate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_in_create",
		Help: "Clusters being created",
	})
	ClustersCreated = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_created",
		Help: "Clusters created",
	})

	ClustersInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_in_progress",
		Help: "Clusters currently being build",
	})
	LoadBalancersInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_lb_clusters_in_progress",
		Help: "Loadbalancers currently being build",
	})

	ClusterProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_clusters_processed",
		Help: "Counter for processed clusters",
	})

	LoadBalancersProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_lb_clusters_processed",
		Help: "Counter for processed LB clusters",
	})

	TasksProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_scheduled_tasks_processed",
		Help: "Scheduled tasks processed",
	})
	TasksProcessedOkCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_scheduled_tasks_processed_ok",
		Help: "Scheduled tasks processed ok",
	})
	TasksProcessedErrCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_scheduled_tasks_processed_err",
		Help: "Scheduled tasks processed error",
	})

	TasksProcessedCreateCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_scheduled_tasks_create_processed",
		Help: "Scheduled create tasks processed",
	})
	TasksProcessedUpdateCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_scheduled_tasks_update_processed",
		Help: "Scheduled update tasks processed",
	})
	TasksProcessedDeleteCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_scheduled_tasks_delete_processed",
		Help: "Scheduled delete tasks processed error",
	})

	LBClustersDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_lb_clusters_deleted",
		Help: "Loadbalancer clusters deleted",
	})

	LBClustersInDeletion = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_lb_clusters_in_deletion",
		Help: "Loadbalancers clusters in deletion",
	})

	K8sAddingNodesInProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "claudie_k8s_cluster_nodes_adding",
			Help: "Nodes currently added to the cluster",
		},
		[]string{K8sClusterLabel, InputManifestLabel},
	)

	K8sDeletingNodesInProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "claudie_k8s_cluster_nodes_deleting",
			Help: "Nodes currently deleted from the cluster",
		},
		[]string{K8sClusterLabel, InputManifestLabel},
	)

	LbAddingNodesInProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "claudie_lb_cluster_nodes_adding",
			Help: "Nodes currently added to the lb cluster",
		},
		[]string{LBClusterLabel, InputManifestLabel, K8sClusterLabel},
	)

	LbDeletingNodesInProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "claudie_lb_cluster_nodes_deleting",
			Help: "Nodes currently deleted from the lb cluster",
		},
		[]string{LBClusterLabel, InputManifestLabel, K8sClusterLabel},
	)
)
