package service

import (
	"time"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ScheduleResult describes what has happened during the
// scheduling of the tasks.
type ScheduleResult uint8

// TODO: endless reconciliation...
const (
	// NoReschedule describes the case where the manifest should not be rescheduled again
	// after either error-ing or completing.
	NoReschedule ScheduleResult = iota
	// Reschedule describes the case where the manifest should be rescheduled again
	// after either error-ing or completing.
	Reschedule
	// NotReady describes the case where the manifest is not ready to be scheduled yet,
	// this is mostly related to the retry policies which can vary. For example if
	// an exponential retry policy is used the manifest will not be ready to be scheduled
	// until the specified number of Tick pass.
	NotReady
	// FinalRetry describes the case where a manifest had a retry policy to retry
	// rescheduling the manifest N times before giving up. FinalRetry states that
	// the manifest should be retried one last time before giving up.
	FinalRetry
)

// Schedules tasks based on the difference between the current and desired state.
// No changes to the passed in values are done.
func scheduleTasks(pending *spec.ConfigV2, desired map[string]*spec.ClustersV2) (ScheduleResult, error) {
	var result ScheduleResult

	fillEntriesForNewClusters(&pending.Clusters, desired)

	for cluster, state := range pending.Clusters {
		// It is guaranteed by validation, that within a single InputManifest
		// no two clusters (including LB) can share the same name.
		current := proto.Clone(state.Current).(*spec.ClustersV2)
		desired := proto.Clone(desired[cluster]).(*spec.ClustersV2)

		switch {
		case current == nil && desired == nil:
			// nothing to do (desired state was not build).
		case current == nil && desired != nil:
			state.Task = createCluster(desired)
		case desired == nil && current != nil:
			state.Task = deleteCluster(current)
		// update
		default:
			panic("Implement update")
		}

		switch result {
		case Reschedule, NoReschedule, FinalRetry:
			// Events are going to be worked on, thus clear the Error state, if any.
			state.State = &spec.WorkflowV2{
				Status: spec.WorkflowV2_WAIT_FOR_PICKUP,
			}
		case NotReady:
		}
	}

	return result, nil
}

func fillEntriesForNewClusters(
	current *map[string]*spec.ClusterStateV2,
	desired map[string]*spec.ClustersV2,
) {
	if *current == nil {
		*current = make(map[string]*spec.ClusterStateV2)
	}

	for desired := range desired {
		if current := (*current)[desired]; current != nil {
			continue
		}
		// create an entry in the map but without any state at all.
		(*current)[desired] = &spec.ClusterStateV2{}
	}
}

func createCluster(desired *spec.ClustersV2) *spec.TaskEventV2 {
	// Choose initial api endpoint.
	var ep bool
	for _, lb := range desired.GetLoadBalancers().GetClusters() {
		if lb.HasApiRole() {
			lb.UsedApiEndpoint = true
			ep = true
			break
		}
	}
	if !ep {
		nps := desired.K8S.ClusterInfo.NodePools
		nodepools.FirstControlNode(nps).NodeType = spec.NodeType_apiEndpoint
	}

	pipeline := []*spec.TaskEventV2_Stage{
		{
			Stage:       spec.TaskEventV2_Stage_TERRAFORMER,
			Description: "Creating infrastructure for the new cluster",
			ErrorLevel:  spec.TaskEventV2_Stage_ErrorFatal,
		},
		{
			Stage:       spec.TaskEventV2_Stage_ANSIBLER,
			Description: "Configuring newly spawned cluster infrastructure",
			ErrorLevel:  spec.TaskEventV2_Stage_ErrorFatal,
		},
		{
			Stage:       spec.TaskEventV2_Stage_KUBE_ELEVEN,
			Description: "Building kubernetes cluster out of the spawned infrastructure",
			ErrorLevel:  spec.TaskEventV2_Stage_ErrorFatal,
		},
		{
			Stage:       spec.TaskEventV2_Stage_KUBER,
			Description: "Finalizing cluster configuration",
			ErrorLevel:  spec.TaskEventV2_Stage_ErrorFatal,
		},
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_CREATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Create{
				Create: &spec.CreateV2{Clusters: desired},
			},
		},
		Description: "creating cluster",
		Pipeline:    pipeline,
	}
}

func deleteCluster(current *spec.ClustersV2) *spec.TaskEventV2 {
	var pipeline []*spec.TaskEventV2_Stage

	if static := nodepools.Static(current.K8S.ClusterInfo.NodePools); len(static) > 0 {
		// We want to continue during the destruction of these two stages even if the
		// kube-eleven stage fails. The static nodes could already be unreachable, for
		// example when credits on a provider expired and there is no way to reach those
		// VMs anymore.
		pipeline = append(pipeline, &spec.TaskEventV2_Stage{
			Stage:       spec.TaskEventV2_Stage_KUBE_ELEVEN,
			Description: "Destroying kubernetes cluster in related binaries",
			ErrorLevel:  spec.TaskEventV2_Stage_ErrorWarn,
		})
		pipeline = append(pipeline, &spec.TaskEventV2_Stage{
			Stage:       spec.TaskEventV2_Stage_ANSIBLER,
			Description: "Removing claudie installed utilities on across the nodes",
			ErrorLevel:  spec.TaskEventV2_Stage_ErrorWarn,
		})
	}

	pipeline = append(pipeline, &spec.TaskEventV2_Stage{
		Stage:       spec.TaskEventV2_Stage_TERRAFORMER,
		Description: "Destroying infrastructure of the cluster",
		ErrorLevel:  spec.TaskEventV2_Stage_ErrorFatal,
	})

	pipeline = append(pipeline, &spec.TaskEventV2_Stage{
		Stage:       spec.TaskEventV2_Stage_KUBER,
		Description: "Cleanup cluster resources in the Claudie Management Cluster",
		ErrorLevel:  spec.TaskEventV2_Stage_ErrorFatal,
	})

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_DELETE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Delete{
				Delete: &spec.DeleteV2{
					Op: &spec.DeleteV2_Clusters_{
						Clusters: &spec.DeleteV2_Clusters{Clusters: current},
					},
				},
			},
		},
		Description: "deleting cluster and its attached loadbalancers",
		Pipeline:    pipeline,
	}
}
