package service

import (
	"time"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Schedules a [spec.TaskEvent] task for creating the clusters in the passed in desired [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleCreateCluster(desired *spec.Clusters) *spec.TaskEvent {
	// Stages
	var (
		tf = spec.Stage_Terraformer{
			Terraformer: &spec.StageTerraformer{
				Description: &spec.StageDescription{
					About:      "Creating infrastructure for the new cluster",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageTerraformer_SubPass{
					{
						Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE,
						Description: &spec.StageDescription{
							About:      "Building desired state infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		ansProxy = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring newly spawned cluster infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables to be used by the package manager",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS,
						Description: &spec.StageDescription{
							About:      "Installing pre-requisites on all of the nodes of the cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_TEE_OVERRIDE,
						Description: &spec.StageDescription{
							About:      "Installing Tee override for newly added nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Setting up VPN across the nodes of the kuberentes and loadbalancer clusters",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables, after populating private addresses on nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
						Description: &spec.StageDescription{
							About:      "Reconciling Envoy service across the loadbalancer nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					// NOTE: there is no need to Commit Proxy envs as this is a create task
				},
			},
		}

		ansNoProxy = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring newly spawned cluster infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS,
						Description: &spec.StageDescription{
							About:      "Installing pre-requisites on all of the nodes of the cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_TEE_OVERRIDE,
						Description: &spec.StageDescription{
							About:      "Installing Tee override for newly added nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Setting up VPN across the nodes of the kuberentes and loadbalancer clusters",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
						Description: &spec.StageDescription{
							About:      "Reconciling Envoy service across the loadbalancer nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		kubeeleven = spec.Stage_KubeEleven{
			KubeEleven: &spec.StageKubeEleven{
				Description: &spec.StageDescription{
					About:      "Building kubernetes cluster out of the spawned infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageKubeEleven_SubPass{
					{
						Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
						Description: &spec.StageDescription{
							About:      "Creating kubernetes cluster from the set up infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		kuber = spec.Stage_Kuber{
			Kuber: &spec.StageKuber{
				Description: &spec.StageDescription{
					About:      "Configuring cluster",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageKuber_SubPass{
					{
						Kind: spec.StageKuber_DEPLOY_KUBELET_CSR_APPROVER,
						Description: &spec.StageDescription{
							About:      "Deploying kubelet csr-approver",
							ErrorLevel: spec.ErrorLevel_ERROR_WARN,
						},
					},
					{
						Kind: spec.StageKuber_PATCH_NODES,
						Description: &spec.StageDescription{
							About:      "Patching nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageKuber_DEPLOY_LONGHORN,
						Description: &spec.StageDescription{
							About:      "Deploying longhorn for storage",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageKuber_RECONCILE_LONGHORN_STORAGE_CLASSES,
						Description: &spec.StageDescription{
							About:      "Reconciling longhorn claudie storage classes",
							ErrorLevel: spec.ErrorLevel_ERROR_WARN,
						},
					},
				},
			},
		}
	)

	var (
		createK8s = proto.Clone(desired.GetK8S()).(*spec.K8Scluster)
		createLbs = proto.Clone(desired.GetLoadBalancers()).(*spec.LoadBalancers)
		createOp  = spec.Create{
			K8S:           createK8s,
			LoadBalancers: createLbs.GetClusters(),
		}
	)

	pipeline := []*spec.Stage{
		{StageKind: &tf},
		{StageKind: nil},
		{StageKind: &kubeeleven},
		{StageKind: &kuber},
	}

	if UsesProxy(desired.K8S) {
		pipeline[1].StageKind = &ansProxy
	} else {
		pipeline[1].StageKind = &ansNoProxy
	}

	if len(nodepools.Autoscaled(createK8s.ClusterInfo.NodePools)) > 0 {
		kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
			Kind: spec.StageKuber_ENABLE_LONGHORN_CA,
			Description: &spec.StageDescription{
				About:      "Enabling cluster-autoscaler support in longhorn",
				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
			},
		})
	}

	if len(createLbs.Clusters) > 0 {
		kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
			Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
			Description: &spec.StageDescription{
				About:      "Storing scrape config for loadbalancers",
				ErrorLevel: spec.ErrorLevel_ERROR_WARN,
			},
		})
	}

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_CREATE,
		Task: &spec.Task{
			Do: &spec.Task_Create{
				Create: &createOp,
			},
		},
		Description: "creating cluster",
		Pipeline:    pipeline,
	}
}

// Schedules a [spec.TaskEvent] task for deleting the clusters in the passed in current [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleDeleteCluster(current *spec.Clusters) *spec.TaskEvent {
	var pipeline []*spec.Stage

	if static := nodepools.Static(current.K8S.ClusterInfo.NodePools); len(static) > 0 {
		// The idea is to continue during the destruction of these two stages even if the
		// kube-eleven stage fails. The static nodes could already be unreachable, for
		// example when credits on a provider expired and there is no way to reach those
		// VMs anymore.
		ke := &spec.Stage{
			StageKind: &spec.Stage_KubeEleven{
				KubeEleven: &spec.StageKubeEleven{
					Description: &spec.StageDescription{
						About:      "Destroying kubernetes cluster and related binaries",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
					SubPasses: []*spec.StageKubeEleven_SubPass{
						{
							Kind: spec.StageKubeEleven_DESTROY_CLUSTER,
							Description: &spec.StageDescription{
								About:      "Tearing down kuberentes cluster",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		}

		ans := &spec.Stage{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Removing claudie installed utilities across nodes",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES,
							Description: &spec.StageDescription{
								About:      "Removing claudie installed utilities",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
						},
					},
				},
			},
		}

		pipeline = append(pipeline, ke)
		pipeline = append(pipeline, ans)
	}

	if dyn := nodepools.Dynamic(current.K8S.ClusterInfo.NodePools); len(dyn) > 0 {
		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Destroying infrastructure of the cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Destroying current state",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})
	}

	var (
		deleteK8s = proto.Clone(current.GetK8S()).(*spec.K8Scluster)
		deleteLbs = proto.Clone(current.GetLoadBalancers()).(*spec.LoadBalancers)
		deleteOp  = spec.Delete{
			K8S:           deleteK8s,
			LoadBalancers: deleteLbs.GetClusters(),
		}
	)

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_DELETE,
		Task: &spec.Task{
			Do: &spec.Task_Delete{
				Delete: &deleteOp,
			},
		},
		Description: "deleting cluster and its attached loadbalancers",
		Pipeline:    pipeline,
	}
}

// Schedules a [spec.TaskEvent] task for reconciling the VPN across the nodes of the clusters in the
// passed in [spec.Clusters]. If proxy is used within the cluster the proxy Envs will also be refreshed.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleRefreshVPN(usesProxy bool, current *spec.Clusters) *spec.TaskEvent {
	var (
		ans = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring infrastructure, after drift detection",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Fixing drift in VPN across nodes of the kuberentes and loadbalancer clusters",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		ansProxy = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring infrastructure, after drift detection",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables to be used by the package manager",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Fixing drift in VPN across nodes of the kuberentes and loadbalancer clusters",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables, after populating private addresses on nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
						Description: &spec.StageDescription{
							About:      "Committing proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}
	)

	pipeline := []*spec.Stage{
		{StageKind: nil},
	}

	if usesProxy {
		pipeline[0].StageKind = &ansProxy
	} else {
		pipeline[0].StageKind = &ans
	}

	inFlight := proto.Clone(current).(*spec.Clusters)
	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
				Update: &spec.Update{
					State: &spec.Update_State{
						K8S:           inFlight.K8S,
						LoadBalancers: inFlight.LoadBalancers.Clusters,
					},
					Delta: new(spec.Update_None_),
				},
			},
		},
		Description: "Refreshing VPN",
		Pipeline:    pipeline,
	}
}
