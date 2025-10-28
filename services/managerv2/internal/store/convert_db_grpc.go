package store

import (
	"fmt"
	"time"

	"github.com/berops/claudie/proto/pb/spec"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ConvertToGRPCStages(stages []Stage) ([]*spec.Stage, error) {
	var out []*spec.Stage

	for _, dbStage := range stages {
		var stage *spec.Stage

		switch dbStage.Kind {
		case Ansibler:
			subpasses := make([]*spec.StageAnsibler_SubPass, 0, len(dbStage.SubPasses))
			for _, p := range dbStage.SubPasses {
				subpasses = append(subpasses, &spec.StageAnsibler_SubPass{
					Kind: spec.StageAnsibler_SubPassKind(spec.StageAnsibler_SubPassKind_value[p.Kind]),
					Description: &spec.StageDescription{
						About:      p.Description.About,
						ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[p.Description.ErrorLevel]),
					},
				})
			}

			stage = &spec.Stage{
				StageKind: &spec.Stage_Ansibler{
					Ansibler: &spec.StageAnsibler{
						Description: &spec.StageDescription{
							About:      dbStage.Description.About,
							ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[dbStage.Description.ErrorLevel]),
						},
						SubPasses: subpasses,
					},
				},
			}
		case KubeEleven:
			subpasses := make([]*spec.StageKubeEleven_SubPass, 0, len(dbStage.SubPasses))
			for _, p := range dbStage.SubPasses {
				subpasses = append(subpasses, &spec.StageKubeEleven_SubPass{
					Kind: spec.StageKubeEleven_SubPassKind(spec.StageKubeEleven_SubPassKind_value[p.Kind]),
					Description: &spec.StageDescription{
						About:      p.Description.About,
						ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[p.Description.ErrorLevel]),
					},
				})
			}

			stage = &spec.Stage{
				StageKind: &spec.Stage_KubeEleven{
					KubeEleven: &spec.StageKubeEleven{
						Description: &spec.StageDescription{
							About:      dbStage.Description.About,
							ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[dbStage.Description.ErrorLevel]),
						},
						SubPasses: subpasses,
					},
				},
			}
		case Kuber:
			subpasses := make([]*spec.StageKuber_SubPass, 0, len(dbStage.SubPasses))
			for _, p := range dbStage.SubPasses {
				subpasses = append(subpasses, &spec.StageKuber_SubPass{
					Kind: spec.StageKuber_SubPassKind(spec.StageKuber_SubPassKind_value[p.Kind]),
					Description: &spec.StageDescription{
						About:      p.Description.About,
						ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[p.Description.ErrorLevel]),
					},
				})
			}

			stage = &spec.Stage{
				StageKind: &spec.Stage_Kuber{
					Kuber: &spec.StageKuber{
						Description: &spec.StageDescription{
							About:      dbStage.Description.About,
							ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[dbStage.Description.ErrorLevel]),
						},
						SubPasses: subpasses,
					},
				},
			}
		case Terraformer:
			subpasses := make([]*spec.StageTerraformer_SubPass, 0, len(dbStage.SubPasses))
			for _, p := range dbStage.SubPasses {
				subpasses = append(subpasses, &spec.StageTerraformer_SubPass{
					Kind: spec.StageTerraformer_SubPassKind(spec.StageTerraformer_SubPassKind_value[p.Kind]),
					Description: &spec.StageDescription{
						About:      p.Description.About,
						ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[p.Description.ErrorLevel]),
					},
				})
			}

			stage = &spec.Stage{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      dbStage.Description.About,
							ErrorLevel: spec.ErrorLevel(spec.ErrorLevel_value[dbStage.Description.ErrorLevel]),
						},
						SubPasses: subpasses,
					},
				},
			}
		case Unknown:
			fallthrough
		default:
			return nil, fmt.Errorf("unrecognized stage kind %s", dbStage.Kind)
		}

		out = append(out, stage)
	}

	return out, nil
}

func ConvertFromGRPCStages(stages []*spec.Stage) ([]Stage, error) {
	var out []Stage

	for _, stage := range stages {
		var (
			dbStage Stage

			stageDescription      *spec.StageDescription
			subpassesDescriptions []*spec.StageDescription
			subpassesKinds        []string
		)

		switch typ := stage.StageKind.(type) {
		case *spec.Stage_Ansibler:
			dbStage.Kind, stageDescription = Ansibler, typ.Ansibler.GetDescription()
			for _, p := range typ.Ansibler.GetSubPasses() {
				subpassesDescriptions = append(subpassesDescriptions, p.GetDescription())
				subpassesKinds = append(subpassesKinds, p.GetKind().String())
			}
		case *spec.Stage_KubeEleven:
			dbStage.Kind, stageDescription = KubeEleven, typ.KubeEleven.GetDescription()
			for _, p := range typ.KubeEleven.GetSubPasses() {
				subpassesDescriptions = append(subpassesDescriptions, p.GetDescription())
				subpassesKinds = append(subpassesKinds, p.GetKind().String())
			}
		case *spec.Stage_Kuber:
			dbStage.Kind, stageDescription = Kuber, typ.Kuber.GetDescription()
			for _, p := range typ.Kuber.GetSubPasses() {
				subpassesDescriptions = append(subpassesDescriptions, p.GetDescription())
				subpassesKinds = append(subpassesKinds, p.GetKind().String())
			}
		case *spec.Stage_Terraformer:
			dbStage.Kind, stageDescription = Terraformer, typ.Terraformer.GetDescription()
			for _, p := range typ.Terraformer.GetSubPasses() {
				subpassesDescriptions = append(subpassesDescriptions, p.GetDescription())
				subpassesKinds = append(subpassesKinds, p.GetKind().String())
			}
		default:
			// left as unknown
		}

		if dbStage.Kind == Unknown {
			return nil, fmt.Errorf("task contains pipeline stage with unknown kind: %T", stage.GetStageKind())
		}

		dbStage.Description.About = stageDescription.GetAbout()
		dbStage.Description.ErrorLevel = stageDescription.GetErrorLevel().String()

		for i := range len(subpassesKinds) {
			dbStage.SubPasses = append(dbStage.SubPasses, SubPass{
				Kind: subpassesKinds[i],
				Description: StageDescription{
					About:      subpassesDescriptions[i].GetAbout(),
					ErrorLevel: subpassesDescriptions[i].GetErrorLevel().String(),
				},
			})
		}

		out = append(out, dbStage)
	}

	return out, nil
}

func ConvertFromGRPCTask(te *spec.TaskEventV2) (*TaskEvent, error) {
	if te == nil {
		return nil, nil
	}

	task, err := proto.Marshal(te.Task)
	if err != nil {
		return nil, err
	}

	retry, err := proto.Marshal(te.OnError)
	if err != nil {
		return nil, err
	}

	var e TaskEvent
	{
		e.Id = te.Id
		e.Timestamp = te.Timestamp.AsTime().Format(time.RFC3339)
		e.Type = te.Event.String()
		e.Task = task
		e.Description = te.Description
		e.OnError = retry
		e.CurrentStage = te.CurrentStage

		e.Pipeline, err = ConvertFromGRPCStages(te.Pipeline)
		if err != nil {
			return nil, err
		}
	}

	return &e, nil
}

func ConvertToGRPCTask(te *TaskEvent) (*spec.TaskEventV2, error) {
	if te == nil {
		return nil, nil
	}

	var task spec.TaskV2
	if err := proto.Unmarshal(te.Task, &task); err != nil {
		return nil, err
	}

	t, err := time.Parse(time.RFC3339, te.Timestamp)
	if err != nil {
		return nil, err
	}

	var strategy spec.RetryV2
	if err := proto.Unmarshal(te.OnError, &strategy); err != nil {
		return nil, err
	}

	e := &spec.TaskEventV2{
		Id:           te.Id,
		Timestamp:    timestamppb.New(t),
		Event:        spec.EventV2(spec.EventV2_value[te.Type]),
		Task:         &task,
		Description:  te.Description,
		OnError:      &strategy,
		Pipeline:     nil,
		CurrentStage: te.CurrentStage,
	}

	e.Pipeline, err = ConvertToGRPCStages(te.Pipeline)
	if err != nil {
		return nil, err
	}

	return e, nil
}

// ConvertFromGRPCWorkflow converts the workflow state data from GRPC to the database representation.
func ConvertFromGRPCWorkflow(w *spec.WorkflowV2) Workflow {
	return Workflow{
		Status:      w.GetStatus().String(),
		Description: w.GetDescription(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

// ConvertToGRPCWorkflow converts the database representation of the workflow state to GRPC.
func ConvertToGRPCWorkflow(w Workflow) *spec.WorkflowV2 {
	return &spec.WorkflowV2{
		Status:      spec.WorkflowV2_Status(spec.WorkflowV2_Status_value[w.Status]),
		Description: w.Description,
	}
}

// ConvertFromGRPC converts the grpc representation to the database representation.
func ConvertFromGRPC(cfg *spec.ConfigV2) (*Config, error) {
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

		task, err := ConvertFromGRPCTask(cluster.GetTask())
		if err != nil {
			return nil, fmt.Errorf("failed to convert task to database representation: %w", err)
		}

		clusters[k8sName] = &ClusterState{
			Current: Clusters{
				K8s:           currentK8s,
				LoadBalancers: currentLbs,
			},
			Task:  task,
			State: ConvertFromGRPCWorkflow(cluster.State),
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
func ConvertToGRPC(cfg *Config) (*spec.ConfigV2, error) {
	grpc := spec.ConfigV2{
		Version: cfg.Version,
		Name:    cfg.Name,
		K8SCtx: &spec.KubernetesContextV2{
			Name:      cfg.K8SCtx.Name,
			Namespace: cfg.K8SCtx.Namespace,
		},
		Manifest: &spec.ManifestV2{
			Raw:                 cfg.Manifest.Raw,
			Checksum:            cfg.Manifest.Checksum,
			LastAppliedChecksum: cfg.Manifest.LastAppliedChecksum,
			State:               spec.ManifestV2_State(spec.ManifestV2_State_value[cfg.Manifest.State]),
		},
		Clusters: nil,
	}

	clusters := make(map[string]*spec.ClusterStateV2)

	for k8sName, cluster := range cfg.Clusters {
		task, err := ConvertToGRPCTask(cluster.Task)
		if err != nil {
			return nil, fmt.Errorf("failed to convert db events back to grpc representation: %w", err)
		}

		// WARN:
		// If making changes to .proto files in the /spec directory
		// we need to always consider backwards compabitlity with the
		// version stored in the database. The database is the proto message
		// in the past and if we update the /spec directory by modifying fields
		// or changing their order we need to consider these changes when reading it from
		// the database aswell.
		current, err := convertClusters(cluster.Current)
		if err != nil {
			return nil, fmt.Errorf("failed to convert db clusters back to grpc representation: %w", err)
		}

		clusters[k8sName] = &spec.ClusterStateV2{
			Current: current,
			State:   ConvertToGRPCWorkflow(cluster.State),
			Task:    task,
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
func convertClusters(cluster Clusters) (*spec.ClustersV2, error) {
	var out *spec.ClustersV2

	if len(cluster.K8s) > 0 {
		var k8s spec.K8SclusterV2
		if err := proto.Unmarshal(cluster.K8s, &k8s); err != nil {
			return nil, fmt.Errorf("failed to unmarshall current k8s cluster: %w", err)
		}
		out = &spec.ClustersV2{K8S: &k8s}

		if len(cluster.LoadBalancers) > 0 {
			var lbs spec.LoadBalancersV2
			if err := proto.Unmarshal(cluster.LoadBalancers, &lbs); err != nil {
				return nil, fmt.Errorf("failed to unmarshall current load balancers cluster: %w", err)
			}
			out.LoadBalancers = &lbs
		}
	}

	return out, nil
}
