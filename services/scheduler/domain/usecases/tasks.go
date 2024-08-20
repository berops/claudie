package usecases

import (
	"time"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func CreateTasks(config *spec.Config) {
	clusterView := utils.NewClusterView(config)

	clusterEvents := make(map[string]*spec.Events)

	for _, k8sCluster := range clusterView.AllClusters() {
		var events []*spec.TaskEvent

		switch {
		case clusterView.CurrentClusters[k8sCluster] == nil:
			events = append(events, &spec.TaskEvent{
				Id:         uuid.New().String(),
				ConfigName: config.Name,
				Timestamp:  timestamppb.New(time.Now().UTC()),
				Event:      spec.Event_CREATE,
				Task: &spec.Task{
					CreateState: &spec.CreateState{
						K8S: clusterView.DesiredClusters[k8sCluster],
						Lbs: clusterView.DesiredLoadbalancers[k8sCluster],
					},
				},
			})
		case clusterView.DesiredClusters[k8sCluster] == nil:
			events = append(events, &spec.TaskEvent{
				Id:         uuid.New().String(),
				ConfigName: config.Name,
				Timestamp:  timestamppb.New(time.Now().UTC()),
				Event:      spec.Event_DELETE,
				Task: &spec.Task{
					DeleteState: &spec.DeleteState{
						K8S: clusterView.CurrentClusters[k8sCluster],
						Lbs: clusterView.Loadbalancers[k8sCluster],
					},
				},
			})
		default:
			// Diff handles the events for edge cases such as EndpointChange, Addition && Deletion
			// of nodes etc. The Config needs to go through an "intermediate representation" before
			// executing the workflow.
			events = append(events, Diff(
				config.Name,
				clusterView.CurrentClusters[k8sCluster],
				clusterView.DesiredClusters[k8sCluster],
				clusterView.Loadbalancers[k8sCluster],
				clusterView.DesiredLoadbalancers[k8sCluster],
			)...)

			events = append(events, &spec.TaskEvent{
				Id:         uuid.New().String(),
				ConfigName: config.Name,
				Timestamp:  timestamppb.New(time.Now().UTC()),
				Event:      spec.Event_UPDATE,
				Task: &spec.Task{
					UpdateState: &spec.UpdateState{
						K8S: clusterView.DesiredClusters[k8sCluster],
						Lbs: clusterView.DesiredLoadbalancers[k8sCluster],
					},
					DeleteState: &spec.DeleteState{
						Lbs: clusterView.DeletedLoadbalancers[k8sCluster],
					},
				},
			})

			events = append(events, &spec.TaskEvent{
				Id:         uuid.New().String(),
				ConfigName: config.Name,
				Timestamp:  timestamppb.New(time.Now().UTC()),
				Event:      spec.Event_DELETE,
				Task: &spec.Task{
					DeleteState: &spec.DeleteState{
						Lbs: clusterView.DeletedLoadbalancers[k8sCluster],
					},
				},
			})
		}

		clusterEvents[k8sCluster] = &spec.Events{
			Events: events,
		}
	}

	config.Events = clusterEvents
}
