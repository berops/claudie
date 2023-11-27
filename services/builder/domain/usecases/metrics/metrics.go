package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	InputManifestLabel = "claudie_input_manifest"
	K8sClusterLabel    = "claudie_k8s_cluster"
	LBClusterLabel     = "claudie_lb_cluster"
)

var (
	InputManifestsProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_processed",
		Help: "Counter for processed input manifests",
	})
	ClusterProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_clusters_processed",
		Help: "Counter for processed clusters",
	})
	LoadBalancersProcessedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_lb_clusters_processed",
		Help: "Counter for processed LB clusters",
	})
	InputManifestsErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_err",
		Help: "Counter for the errors occurred during processing of Input Manifests",
	})

	InputManifestsDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_input_manifests_deleted",
		Help: "Input Manifests deleted",
	})
	ClustersDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_clusters_deleted",
		Help: "Clusters deleted",
	})
	LBClustersDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "claudie_lb_clusters_deleted",
		Help: "Loadbalancer clusters deleted",
	})

	InputManifestBuildError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "claudie_cluster_error",
		Help: "Number of build errors per input manifest",
	}, []string{InputManifestLabel})

	InputManifestInDeletion = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_input_manifests_in_deletion",
		Help: "Input Manifests in deletion",
	})
	ClustersInDeletion = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_in_deletion",
		Help: "Clusters in deletion",
	})
	LBClustersInDeletion = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_lb_clusters_in_deletion",
		Help: "Loadbalancers clusters in deletion",
	})

	InputManifestsInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_input_manifests_in_progress",
		Help: "Input Manifests in progress",
	})
	ClustersInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_clusters_in_progress",
		Help: "Clusters in progress",
	})
	LoadBalancersInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "claudie_loadbalancers_clusters_in_progress",
		Help: "LoadBalancer clusters in progress",
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

func MustRegisterCounters() {
	prometheus.MustRegister(ClusterProcessedCounter)
	prometheus.MustRegister(InputManifestsProcessedCounter)
	prometheus.MustRegister(InputManifestsErrorCounter)
	prometheus.MustRegister(LoadBalancersProcessedCounter)

	prometheus.MustRegister(InputManifestInDeletion)
	prometheus.MustRegister(ClustersInDeletion)
	prometheus.MustRegister(LBClustersInDeletion)
	prometheus.MustRegister(InputManifestsDeleted)
	prometheus.MustRegister(ClustersDeleted)
	prometheus.MustRegister(LBClustersDeleted)

	prometheus.MustRegister(InputManifestBuildError)

	prometheus.MustRegister(InputManifestsInProgress)
	prometheus.MustRegister(ClustersInProgress)
	prometheus.MustRegister(LoadBalancersInProgress)

	prometheus.MustRegister(K8sAddingNodesInProgress)
	prometheus.MustRegister(K8sDeletingNodesInProgress)
	prometheus.MustRegister(LbAddingNodesInProgress)
	prometheus.MustRegister(LbDeletingNodesInProgress)
}
