package store

import (
	"fmt"
	"time"

	"github.com/berops/claudie/proto/pb/spec"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConvertToGRPCEvents converts the database representation of events to GRPC.
func ConvertToGRPCEvents(w Events) (*spec.Events, error) {
	var te []*spec.TaskEvent

	for i := range w.TaskEvents {
		g, err := ConvertToGRPCTaskEvent(w.TaskEvents[i])
		if err != nil {
			return nil, err
		}
		te = append(te, g)
	}

	return &spec.Events{Events: te, Ttl: w.TTL, Autoscaled: w.Autoscaled}, nil
}

// ConvertFromGRPCEvents converts the events data from GRPC to the database representation.
func ConvertFromGRPCEvents(w *spec.Events) (Events, error) {
	var te []TaskEvent
	for _, e := range w.Events {
		t, err := proto.Marshal(e.Task)
		if err != nil {
			return Events{}, err
		}

		r, err := proto.Marshal(e.OnError)
		if err != nil {
			return Events{}, err
		}

		te = append(te, TaskEvent{
			Id:          e.Id,
			Timestamp:   e.Timestamp.AsTime().Format(time.RFC3339),
			Event:       e.Event.String(),
			Task:        t,
			Description: e.Description,
			OnError:     r,
		})
	}

	return Events{TaskEvents: te, TTL: w.Ttl, Autoscaled: w.Autoscaled}, nil
}

func ConvertToGRPCTaskEvent(te TaskEvent) (*spec.TaskEvent, error) {
	var task spec.Task
	if err := proto.Unmarshal(te.Task, &task); err != nil {
		return nil, err
	}

	t, err := time.Parse(time.RFC3339, te.Timestamp)
	if err != nil {
		return nil, err
	}

	var strategy spec.Retry
	if err := proto.Unmarshal(te.OnError, &strategy); err != nil {
		return nil, err
	}

	return &spec.TaskEvent{
		Id:          te.Id,
		Timestamp:   timestamppb.New(t),
		Event:       spec.Event(spec.Event_value[te.Event]),
		Task:        &task,
		Description: te.Description,
		OnError:     &strategy,
	}, nil
}

// ConvertFromGRPCWorkflow converts the workflow state data from GRPC to the database representation.
func ConvertFromGRPCWorkflow(w *spec.Workflow) Workflow {
	return Workflow{
		Status:      w.GetStatus().String(),
		Stage:       w.GetStage().String(),
		Description: w.GetDescription(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

// ConvertToGRPCWorkflow converts the database representation of the workflow state to GRPC.
func ConvertToGRPCWorkflow(w Workflow) *spec.Workflow {
	return &spec.Workflow{
		Stage:       spec.Workflow_Stage(spec.Workflow_Stage_value[w.Stage]),
		Status:      spec.Workflow_Status(spec.Workflow_Status_value[w.Status]),
		Description: w.Description,
	}
}

// ConvertFromGRPC converts the grpc representation to the database representation.
func ConvertFromGRPC(cfg *spec.Config) (*Config, error) {
	db := Config{
		Version: cfg.GetVersion(),
		Name:    cfg.GetName(),
		K8SCtx: KubernetesContext{
			Name:      cfg.GetK8SCtx().GetName(),
			Namespace: cfg.GetK8SCtx().GetNamespace(),
		},
		Manifest: Manifest{
			Raw:                 cfg.GetManifest().GetRaw(),
			Checksum:            cfg.GetManifest().GetChecksum(),
			LastAppliedChecksum: cfg.GetManifest().GetLastAppliedChecksum(),
			State:               cfg.GetManifest().GetState().String(),
		},
		Clusters: nil,
	}

	clusters := make(map[string]*ClusterState, len(cfg.GetClusters()))

	for k8sName, cluster := range cfg.GetClusters() {
		marshaller := proto.MarshalOptions{Deterministic: true}
		currentK8s, err := marshaller.Marshal(cluster.GetCurrent().GetK8S())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal current k8s cluster: %w", err)
		}
		currentLbs, err := marshaller.Marshal(cluster.GetCurrent().GetLoadBalancers())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal current load balancer clusters: %w", err)
		}

		desiredK8s, err := marshaller.Marshal(cluster.GetDesired().GetK8S())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal desired k8s cluster: %w", err)
		}
		desiredLbs, err := marshaller.Marshal(cluster.GetDesired().GetLoadBalancers())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal desired load balancer clusters: %w", err)
		}

		events, err := ConvertFromGRPCEvents(cluster.GetEvents())
		if err != nil {
			return nil, fmt.Errorf("failed to convert events to database representation: %w", err)
		}

		clusters[k8sName] = &ClusterState{
			Current: Clusters{
				K8s:           currentK8s,
				LoadBalancers: currentLbs,
			},
			Desired: Clusters{
				K8s:           desiredK8s,
				LoadBalancers: desiredLbs,
			},
			Events: events,
			State:  ConvertFromGRPCWorkflow(cluster.State),
		}
	}

	if len(clusters) > 0 {
		db.Clusters = clusters
	}

	return &db, nil
}

// ConvertToGRPC converts from database representation to GRPC representation.
// For clusters, it mimics the GRPC unmarshalling style where if a field was
// not set within a message it will be nil instead of a zero value for that type.
func ConvertToGRPC(cfg *Config) (*spec.Config, error) {
	grpc := spec.Config{
		Version: cfg.Version,
		Name:    cfg.Name,
		K8SCtx: &spec.KubernetesContext{
			Name:      cfg.K8SCtx.Name,
			Namespace: cfg.K8SCtx.Namespace,
		},
		Manifest: &spec.Manifest{
			Raw:                 cfg.Manifest.Raw,
			Checksum:            cfg.Manifest.Checksum,
			LastAppliedChecksum: cfg.Manifest.LastAppliedChecksum,
			State:               spec.Manifest_State(spec.Manifest_State_value[cfg.Manifest.State]),
		},
		Clusters: nil,
	}

	clusters := make(map[string]*spec.ClusterState)

	for k8sName, cluster := range cfg.Clusters {
		events, err := ConvertToGRPCEvents(cluster.Events)
		if err != nil {
			return nil, fmt.Errorf("failed to convert db events back to grpc representation: %w", err)
		}

		current, err := convertClusters(cluster.Current)
		if err != nil {
			return nil, fmt.Errorf("failed to convert db clusters back to grpc representation: %w", err)
		}

		desired, err := convertClusters(cluster.Desired)
		if err != nil {
			return nil, fmt.Errorf("failed to convert db clusters back to grpc representation: %w", err)
		}

		clusters[k8sName] = &spec.ClusterState{
			Current: current,
			Desired: desired,
			Events:  events,
			State:   ConvertToGRPCWorkflow(cluster.State),
		}
	}

	if len(clusters) > 0 {
		grpc.Clusters = clusters
	}

	return &grpc, nil
}

// convertClusters converts the database representation to the GRPC representation.
// If no error is returned, the result can still be nil. This is so that the GRPC
// representation will have a nil (essentially mimicking what the GRPC unmarshall does
// if the respective value is not set) value as well when converted which simplifies
// checking absence of a specific state (i.e. current, desired).
func convertClusters(cluster Clusters) (*spec.Clusters, error) {
	var out *spec.Clusters

	if len(cluster.K8s) > 0 {
		var k8s spec.K8Scluster
		if err := proto.Unmarshal(cluster.K8s, &k8s); err != nil {
			return nil, fmt.Errorf("failed to unmarshall current k8s cluster: %w", err)
		}
		out = &spec.Clusters{K8S: &k8s}

		if len(cluster.LoadBalancers) > 0 {
			var lbs spec.LoadBalancers
			if err := proto.Unmarshal(cluster.LoadBalancers, &lbs); err != nil {
				return nil, fmt.Errorf("failed to unmarshall current load balancers cluster: %w", err)
			}
			out.LoadBalancers = &lbs
		}
	}

	return out, nil
}
