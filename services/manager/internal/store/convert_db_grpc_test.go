package store_test

import (
	"testing"
	"time"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConvertFromGRPCWorkflow(t *testing.T) {
	t.Parallel()

	prevTimestamp := timestamppb.New(time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC))

	tests := []struct {
		name  string
		input *spec.Workflow
		want  store.Workflow
	}{
		{
			name: "workflow with previous entries",
			input: &spec.Workflow{
				Status:      spec.Workflow_DONE,
				Description: "completed successfully",
				Previous: []*spec.FinishedWorkflow{
					{
						Status:          spec.Workflow_IN_PROGRESS,
						TaskDescription: "building infrastructure",
						Stage:           string(store.Terraformer),
						Timestamp:       prevTimestamp,
					},
					{
						Status:          spec.Workflow_ERROR,
						TaskDescription: "failed to reconcile",
						Stage:           string(store.Ansibler),
						Timestamp:       prevTimestamp,
					},
				},
			},
			want: store.Workflow{
				Status:      spec.Workflow_DONE.String(),
				Description: "completed successfully",
				Previous: []store.FinishedWorkflow{
					{
						Status:          spec.Workflow_IN_PROGRESS.String(),
						TaskDescription: "building infrastructure",
						Stage:           store.Terraformer,
						Timestamp:       prevTimestamp.AsTime().UTC().Format(time.RFC3339),
					},
					{
						Status:          spec.Workflow_ERROR.String(),
						TaskDescription: "failed to reconcile",
						Stage:           store.Ansibler,
						Timestamp:       prevTimestamp.AsTime().UTC().Format(time.RFC3339),
					},
				},
			},
		},
		{
			name: "workflow with no previous entries",
			input: &spec.Workflow{
				Status:      spec.Workflow_WAIT_FOR_PICKUP,
				Description: "awaiting pickup",
				Previous:    nil,
			},
			want: store.Workflow{
				Status:      spec.Workflow_WAIT_FOR_PICKUP.String(),
				Description: "awaiting pickup",
				Previous:    nil,
			},
		},
		{
			name: "workflow in progress",
			input: &spec.Workflow{
				Status:      spec.Workflow_IN_PROGRESS,
				Description: "working on task",
			},
			want: store.Workflow{
				Status:      spec.Workflow_IN_PROGRESS.String(),
				Description: "working on task",
				Previous:    nil,
			},
		},
		{
			name: "workflow error status",
			input: &spec.Workflow{
				Status:      spec.Workflow_ERROR,
				Description: "something went wrong",
				Previous: []*spec.FinishedWorkflow{
					{
						Status:          spec.Workflow_DONE,
						TaskDescription: "previous task",
						Stage:           string(store.Kuber),
						Timestamp:       prevTimestamp,
					},
				},
			},
			want: store.Workflow{
				Status:      spec.Workflow_ERROR.String(),
				Description: "something went wrong",
				Previous: []store.FinishedWorkflow{
					{
						Status:          spec.Workflow_DONE.String(),
						TaskDescription: "previous task",
						Stage:           store.Kuber,
						Timestamp:       prevTimestamp.AsTime().UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := time.Now().UTC()
			got := store.ConvertFromGRPCWorkflow(tt.input)
			after := time.Now().UTC()

			// Timestamp is set to time.Now() inside the function,
			// so we verify it falls within the expected range and
			// then overwrite it for the rest of the comparison.
			ts, err := time.Parse(time.RFC3339, got.Timestamp)
			assert.NoError(t, err)
			assert.False(t, ts.Before(before.Truncate(time.Second)), "timestamp %v should not be before %v", ts, before)
			assert.False(t, ts.After(after.Add(time.Second)), "timestamp %v should not be after %v", ts, after)

			// Clear the dynamic timestamp for structural comparison.
			got.Timestamp = ""
			tt.want.Timestamp = ""

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertToGRPCWorkflow(t *testing.T) {
	t.Parallel()

	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	tsStr := ts.Format(time.RFC3339)

	tests := []struct {
		name  string
		input store.Workflow
		want  *spec.Workflow
	}{
		{
			name: "workflow with previous entries",
			input: store.Workflow{
				Status:      spec.Workflow_DONE.String(),
				Description: "completed successfully",
				Previous: []store.FinishedWorkflow{
					{
						Status:          spec.Workflow_IN_PROGRESS.String(),
						TaskDescription: "building infrastructure",
						Stage:           store.Terraformer,
						Timestamp:       tsStr,
					},
					{
						Status:          spec.Workflow_ERROR.String(),
						TaskDescription: "failed to reconcile",
						Stage:           store.Ansibler,
						Timestamp:       tsStr,
					},
				},
			},
			want: &spec.Workflow{
				Status:      spec.Workflow_DONE,
				Description: "completed successfully",
				Previous: []*spec.FinishedWorkflow{
					{
						Status:          spec.Workflow_IN_PROGRESS,
						TaskDescription: "building infrastructure",
						Stage:           string(store.Terraformer),
						Timestamp:       timestamppb.New(ts),
					},
					{
						Status:          spec.Workflow_ERROR,
						TaskDescription: "failed to reconcile",
						Stage:           string(store.Ansibler),
						Timestamp:       timestamppb.New(ts),
					},
				},
			},
		},
		{
			name: "workflow with no previous entries",
			input: store.Workflow{
				Status:      spec.Workflow_WAIT_FOR_PICKUP.String(),
				Description: "awaiting pickup",
				Previous:    nil,
			},
			want: &spec.Workflow{
				Status:      spec.Workflow_WAIT_FOR_PICKUP,
				Description: "awaiting pickup",
				Previous:    nil,
			},
		},
		{
			name: "workflow with invalid timestamp falls back to time.Now",
			input: store.Workflow{
				Status:      spec.Workflow_DONE.String(),
				Description: "done",
				Previous: []store.FinishedWorkflow{
					{
						Status:          spec.Workflow_DONE.String(),
						TaskDescription: "previous",
						Stage:           store.KubeEleven,
						Timestamp:       "not-a-valid-timestamp",
					},
				},
			},
		},
		{
			name: "workflow in error state",
			input: store.Workflow{
				Status:      spec.Workflow_ERROR.String(),
				Description: "something went wrong",
				Previous: []store.FinishedWorkflow{
					{
						Status:          spec.Workflow_IN_PROGRESS.String(),
						TaskDescription: "was in progress",
						Stage:           store.Kuber,
						Timestamp:       tsStr,
					},
				},
			},
			want: &spec.Workflow{
				Status:      spec.Workflow_ERROR,
				Description: "something went wrong",
				Previous: []*spec.FinishedWorkflow{
					{
						Status:          spec.Workflow_IN_PROGRESS,
						TaskDescription: "was in progress",
						Stage:           string(store.Kuber),
						Timestamp:       timestamppb.New(ts),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := time.Now().UTC()
			got := store.ConvertToGRPCWorkflow(tt.input)
			after := time.Now().UTC()

			// Special handling for the invalid timestamp case:
			// the function falls back to time.Now(), so we verify
			// the timestamp is approximately now and skip proto diff.
			if tt.want == nil {
				assert.Equal(t, spec.Workflow_DONE, got.Status)
				assert.Equal(t, "done", got.Description)
				assert.Len(t, got.Previous, 1)

				fallbackTs := got.Previous[0].Timestamp.AsTime().UTC()
				assert.False(t, fallbackTs.Before(before.Truncate(time.Second)))
				assert.False(t, fallbackTs.After(after.Add(time.Second)))

				assert.Equal(t, spec.Workflow_DONE, got.Previous[0].Status)
				assert.Equal(t, "previous", got.Previous[0].TaskDescription)
				assert.Equal(t, string(store.KubeEleven), got.Previous[0].Stage)
				return
			}

			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertToGRPCTask(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    *spec.Task
		wantErr bool
	}{
		{
			name: "valid bytes unmarshals correctly",
			input: func() []byte {
				task := &spec.Task{
					Do: &spec.Task_Create{
						Create: &spec.Create{
							K8S: &spec.K8Scluster{
								ClusterInfo: &spec.ClusterInfo{
									Name: "roundtrip-k8s",
									Hash: "pqr678",
								},
								Kubernetes: "v1.28.0",
							},
						},
					},
				}
				b, _ := proto.Marshal(task)
				return b
			}(),
			want: &spec.Task{
				Do: &spec.Task_Create{
					Create: &spec.Create{
						K8S: &spec.K8Scluster{
							ClusterInfo: &spec.ClusterInfo{
								Name: "roundtrip-k8s",
								Hash: "pqr678",
							},
							Kubernetes: "v1.28.0",
						},
					},
				},
			},
		},
		{
			name:    "invalid bytes returns error",
			input:   []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa},
			wantErr: true,
		},
		{
			name: "empty bytes unmarshals to empty task",
			input: func() []byte {
				b, _ := proto.Marshal(&spec.Task{})
				return b
			}(),
			want: &spec.Task{},
		},
		{
			name:    "nil bytes unmarshals to empty task",
			input:   nil,
			want:    &spec.Task{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := store.ConvertToGRPCTask(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertFromGRPCTask(t *testing.T) {
	tests := []struct {
		name string
		task *spec.Task
	}{
		{
			name: "Create task",
			task: &spec.Task{
				Do: &spec.Task_Create{
					Create: &spec.Create{
						K8S: &spec.K8Scluster{
							ClusterInfo: &spec.ClusterInfo{
								Name: "test-k8s",
								Hash: "abc123",
							},
							Network:    "192.168.0.0/16",
							Kubernetes: "v1.28.0",
						},
						LoadBalancers: []*spec.LBcluster{
							{
								ClusterInfo: &spec.ClusterInfo{
									Name: "test-lb",
									Hash: "def456",
								},
								TargetedK8S: "test-k8s",
								Roles: []*spec.Role{
									{
										Name:     "api",
										Protocol: "tcp",
										Port:     6443,
										RoleType: spec.RoleType_ApiServer,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Update task with None delta",
			task: &spec.Task{
				Do: &spec.Task_Update{
					Update: &spec.Update{
						State: &spec.Update_State{
							K8S: &spec.K8Scluster{
								ClusterInfo: &spec.ClusterInfo{
									Name: "update-k8s",
									Hash: "ghi789",
								},
								Kubernetes: "v1.29.0",
							},
						},
						Delta: &spec.Update_None_{
							None: &spec.Update_None{},
						},
					},
				},
			},
		},
		{
			name: "Delete task",
			task: &spec.Task{
				Do: &spec.Task_Delete{
					Delete: &spec.Delete{
						K8S: &spec.K8Scluster{
							ClusterInfo: &spec.ClusterInfo{
								Name: "delete-k8s",
								Hash: "jkl012",
							},
						},
					},
				},
			},
		},
		{
			name: "Update task with TerraformerAddK8sNodes delta",
			task: &spec.Task{
				Do: &spec.Task_Update{
					Update: &spec.Update{
						State: &spec.Update_State{
							K8S: &spec.K8Scluster{
								ClusterInfo: &spec.ClusterInfo{
									Name: "scale-k8s",
									Hash: "mno345",
								},
								Kubernetes: "v1.28.0",
							},
						},
						Delta: &spec.Update_TfAddK8SNodes{
							TfAddK8SNodes: &spec.Update_TerraformerAddK8SNodes{
								Kind: &spec.Update_TerraformerAddK8SNodes_Existing_{
									Existing: &spec.Update_TerraformerAddK8SNodes_Existing{
										Nodepool: "worker-pool",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := store.ConvertFromGRPCTask(tt.task)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			// Verify round-trip: the marshalled bytes should unmarshal back to an equal message.
			got, err := store.ConvertToGRPCTask(data)
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.task, got, protocmp.Transform()); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertFromGRPCStages(t *testing.T) {
	tests := []struct {
		name    string
		input   []*spec.Stage
		want    []store.Stage
		wantErr bool
	}{
		{
			name: "Kuber stage with subpass",
			input: []*spec.Stage{
				{
					StageKind: &spec.Stage_Kuber{
						Kuber: &spec.StageKuber{
							Description: &spec.StageDescription{
								About:      "Kuber about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageKuber_SubPass{
								{
									Kind: spec.StageKuber_DELETE_NODES,
									Description: &spec.StageDescription{
										About:      "Kuber subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_WARN,
									},
								},
							},
						},
					},
				},
			},
			want: []store.Stage{
				{
					Kind: store.Kuber,
					Description: store.StageDescription{
						About:      "Kuber about",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageKuber_DELETE_NODES.String(),
							Description: store.StageDescription{
								About:      "Kuber subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
							},
						},
					},
				},
			},
		},
		{
			name: "Ansibler stage with subpass",
			input: []*spec.Stage{
				{
					StageKind: &spec.Stage_Ansibler{
						Ansibler: &spec.StageAnsibler{
							Description: &spec.StageDescription{
								About:      "Ansibler about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageAnsibler_SubPass{
								{
									Kind: spec.StageAnsibler_INSTALL_VPN,
									Description: &spec.StageDescription{
										About:      "Ansibler subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_WARN,
									},
								},
							},
						},
					},
				},
			},
			want: []store.Stage{
				{
					Kind: store.Ansibler,
					Description: store.StageDescription{
						About:      "Ansibler about",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageAnsibler_INSTALL_VPN.String(),
							Description: store.StageDescription{
								About:      "Ansibler subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
							},
						},
					},
				},
			},
		},
		{
			name: "KubeEleven stage with subpass",
			input: []*spec.Stage{
				{
					StageKind: &spec.Stage_KubeEleven{
						KubeEleven: &spec.StageKubeEleven{
							Description: &spec.StageDescription{
								About:      "KubeEleven about",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
							SubPasses: []*spec.StageKubeEleven_SubPass{
								{
									Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
									Description: &spec.StageDescription{
										About:      "KubeEleven subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
			},
			want: []store.Stage{
				{
					Kind: store.KubeEleven,
					Description: store.StageDescription{
						About:      "KubeEleven about",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageKubeEleven_RECONCILE_CLUSTER.String(),
							Description: store.StageDescription{
								About:      "KubeEleven subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
			},
		},
		{
			name: "Terraformer stage with subpass",
			input: []*spec.Stage{
				{
					StageKind: &spec.Stage_Terraformer{
						Terraformer: &spec.StageTerraformer{
							Description: &spec.StageDescription{
								About:      "Terraformer about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageTerraformer_SubPass{
								{
									Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE,
									Description: &spec.StageDescription{
										About:      "Terraformer subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
			},
			want: []store.Stage{
				{
					Kind: store.Terraformer,
					Description: store.StageDescription{
						About:      "Terraformer about",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE.String(),
							Description: store.StageDescription{
								About:      "Terraformer subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
			},
		},
		{
			name: "multiple stages with multiple subpasses",
			input: []*spec.Stage{
				{
					StageKind: &spec.Stage_Terraformer{
						Terraformer: &spec.StageTerraformer{
							Description: &spec.StageDescription{
								About:      "Terraform infra",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageTerraformer_SubPass{
								{
									Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE,
									Description: &spec.StageDescription{
										About:      "Destroy",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
				{
					StageKind: &spec.Stage_Kuber{
						Kuber: &spec.StageKuber{
							Description: &spec.StageDescription{
								About:      "Kuber ops",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
							SubPasses: []*spec.StageKuber_SubPass{
								{
									Kind: spec.StageKuber_DEPLOY_LONGHORN,
									Description: &spec.StageDescription{
										About:      "Deploy longhorn",
										ErrorLevel: spec.ErrorLevel_ERROR_WARN,
									},
								},
								{
									Kind: spec.StageKuber_CILIUM_RESTART,
									Description: &spec.StageDescription{
										About:      "Restart cilium",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
			},
			want: []store.Stage{
				{
					Kind: store.Terraformer,
					Description: store.StageDescription{
						About:      "Terraform infra",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE.String(),
							Description: store.StageDescription{
								About:      "Destroy",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
				{
					Kind: store.Kuber,
					Description: store.StageDescription{
						About:      "Kuber ops",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageKuber_DEPLOY_LONGHORN.String(),
							Description: store.StageDescription{
								About:      "Deploy longhorn",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
							},
						},
						{
							Kind: spec.StageKuber_CILIUM_RESTART.String(),
							Description: store.StageDescription{
								About:      "Restart cilium",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
			},
		},
		{
			name: "stage with no subpasses",
			input: []*spec.Stage{
				{
					StageKind: &spec.Stage_Ansibler{
						Ansibler: &spec.StageAnsibler{
							Description: &spec.StageDescription{
								About:      "Ansibler no subpasses",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: nil,
						},
					},
				},
			},
			want: []store.Stage{
				{
					Kind: store.Ansibler,
					Description: store.StageDescription{
						About:      "Ansibler no subpasses",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: nil,
				},
			},
		},
		{
			name: "nil StageKind returns error",
			input: []*spec.Stage{
				{
					StageKind: nil,
				},
			},
			wantErr: true,
		},
		{
			name:  "empty input returns nil",
			input: []*spec.Stage{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := store.ConvertFromGRPCStages(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertToGRPCStages(t *testing.T) {
	tests := []struct {
		name    string
		input   []store.Stage
		want    []*spec.Stage
		wantErr bool
	}{
		{
			name: "Kuber stage with subpass",
			input: []store.Stage{
				{
					Kind: store.Kuber,
					Description: store.StageDescription{
						About:      "Kuber about",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageKuber_DELETE_NODES.String(),
							Description: store.StageDescription{
								About:      "Kuber subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
							},
						},
					},
				},
			},
			want: []*spec.Stage{
				{
					StageKind: &spec.Stage_Kuber{
						Kuber: &spec.StageKuber{
							Description: &spec.StageDescription{
								About:      "Kuber about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageKuber_SubPass{
								{
									Kind: spec.StageKuber_DELETE_NODES,
									Description: &spec.StageDescription{
										About:      "Kuber subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_WARN,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Ansibler stage with subpass",
			input: []store.Stage{
				{
					Kind: store.Ansibler,
					Description: store.StageDescription{
						About:      "Ansibler about",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageAnsibler_INSTALL_VPN.String(),
							Description: store.StageDescription{
								About:      "Ansibler subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
							},
						},
					},
				},
			},
			want: []*spec.Stage{
				{
					StageKind: &spec.Stage_Ansibler{
						Ansibler: &spec.StageAnsibler{
							Description: &spec.StageDescription{
								About:      "Ansibler about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageAnsibler_SubPass{
								{
									Kind: spec.StageAnsibler_INSTALL_VPN,
									Description: &spec.StageDescription{
										About:      "Ansibler subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_WARN,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "KubeEleven stage with subpass",
			input: []store.Stage{
				{
					Kind: store.KubeEleven,
					Description: store.StageDescription{
						About:      "KubeEleven about",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageKubeEleven_RECONCILE_CLUSTER.String(),
							Description: store.StageDescription{
								About:      "KubeEleven subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
			},
			want: []*spec.Stage{
				{
					StageKind: &spec.Stage_KubeEleven{
						KubeEleven: &spec.StageKubeEleven{
							Description: &spec.StageDescription{
								About:      "KubeEleven about",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
							SubPasses: []*spec.StageKubeEleven_SubPass{
								{
									Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
									Description: &spec.StageDescription{
										About:      "KubeEleven subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Terraformer stage with subpass",
			input: []store.Stage{
				{
					Kind: store.Terraformer,
					Description: store.StageDescription{
						About:      "Terraformer about",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE.String(),
							Description: store.StageDescription{
								About:      "Terraformer subpass about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
			},
			want: []*spec.Stage{
				{
					StageKind: &spec.Stage_Terraformer{
						Terraformer: &spec.StageTerraformer{
							Description: &spec.StageDescription{
								About:      "Terraformer about",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageTerraformer_SubPass{
								{
									Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE,
									Description: &spec.StageDescription{
										About:      "Terraformer subpass about",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple stages",
			input: []store.Stage{
				{
					Kind: store.Terraformer,
					Description: store.StageDescription{
						About:      "Terraform infra",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE.String(),
							Description: store.StageDescription{
								About:      "Destroy",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
				{
					Kind: store.Kuber,
					Description: store.StageDescription{
						About:      "Kuber ops",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
					},
					SubPasses: []store.SubPass{
						{
							Kind: spec.StageKuber_DEPLOY_LONGHORN.String(),
							Description: store.StageDescription{
								About:      "Deploy longhorn",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN.String(),
							},
						},
						{
							Kind: spec.StageKuber_CILIUM_RESTART.String(),
							Description: store.StageDescription{
								About:      "Restart cilium",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
							},
						},
					},
				},
			},
			want: []*spec.Stage{
				{
					StageKind: &spec.Stage_Terraformer{
						Terraformer: &spec.StageTerraformer{
							Description: &spec.StageDescription{
								About:      "Terraform infra",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageTerraformer_SubPass{
								{
									Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE,
									Description: &spec.StageDescription{
										About:      "Destroy",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
				{
					StageKind: &spec.Stage_Kuber{
						Kuber: &spec.StageKuber{
							Description: &spec.StageDescription{
								About:      "Kuber ops",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
							SubPasses: []*spec.StageKuber_SubPass{
								{
									Kind: spec.StageKuber_DEPLOY_LONGHORN,
									Description: &spec.StageDescription{
										About:      "Deploy longhorn",
										ErrorLevel: spec.ErrorLevel_ERROR_WARN,
									},
								},
								{
									Kind: spec.StageKuber_CILIUM_RESTART,
									Description: &spec.StageDescription{
										About:      "Restart cilium",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty subpasses",
			input: []store.Stage{
				{
					Kind: store.Ansibler,
					Description: store.StageDescription{
						About:      "Ansibler no subpasses",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
					SubPasses: []store.SubPass{},
				},
			},
			want: []*spec.Stage{
				{
					StageKind: &spec.Stage_Ansibler{
						Ansibler: &spec.StageAnsibler{
							Description: &spec.StageDescription{
								About:      "Ansibler no subpasses",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
							SubPasses: []*spec.StageAnsibler_SubPass{},
						},
					},
				},
			},
		},
		{
			name: "unknown stage kind returns error",
			input: []store.Stage{
				{
					Kind: store.Unknown,
					Description: store.StageDescription{
						About:      "Unknown stage",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unrecognized stage kind returns error",
			input: []store.Stage{
				{
					Kind: "nonexistent",
					Description: store.StageDescription{
						About:      "Bad stage",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL.String(),
					},
				},
			},
			wantErr: true,
		},
		{
			name:  "empty input returns nil",
			input: []store.Stage{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := store.ConvertToGRPCStages(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertToDBAndBack(t *testing.T) {
	t.Parallel()

	want := &spec.Config{
		Version: 256,
		Name:    "Test-03",
		K8SCtx: &spec.KubernetesContext{
			Name:      "test-03",
			Namespace: "test-04",
		},
		Manifest: &spec.Manifest{
			Raw:      "random-manifest",
			Checksum: hash.Digest("random-manifest"),
			State:    spec.Manifest_Pending,
		},
		Clusters: map[string]*spec.ClusterState{
			"test-03": {
				Current: &spec.Clusters{
					K8S: &spec.K8Scluster{
						ClusterInfo: &spec.ClusterInfo{
							Name: "Desired-K8s-test-cluster",
							Hash: "abcd",
							NodePools: []*spec.NodePool{
								{
									Type: &spec.NodePool_DynamicNodePool{
										DynamicNodePool: &spec.DynamicNodePool{
											ServerType:      "performance",
											Image:           "latest",
											StorageDiskSize: 50,
											Region:          "local",
											Zone:            "local",
											Count:           3,
											Provider: &spec.Provider{
												SpecName:          "hetzner",
												CloudProviderName: "hetzner-01",
												ProviderType: &spec.Provider_Hetzner{
													Hetzner: &spec.HetznerProvider{
														Token: "test",
													},
												},
												Templates: &spec.TemplateRepository{
													Repository: "/root/",
													Path:       "hetzner",
												},
											},
											PublicKey:  "default",
											PrivateKey: "default",
											Cidr:       "127.0.0.1/24",
										},
									},
									Name: "test-nodepool",
									Nodes: []*spec.Node{
										{
											Name:     "test-node-01",
											Private:  "192.168.0.1",
											Public:   "127.0.0.1",
											NodeType: spec.NodeType_master,
											Username: "root",
										},
									},
									IsControl: true,
								},
							},
						},
						Network:    "127.0.0.1/24",
						Kubeconfig: "test-kubeconfig",
						Kubernetes: "test-kubernetes",
					},
					LoadBalancers: &spec.LoadBalancers{
						Clusters: []*spec.LBcluster{
							{
								ClusterInfo: &spec.ClusterInfo{
									Name: "Desired-lb-test-cluster",
									Hash: "abcd",
									NodePools: []*spec.NodePool{
										{
											Type: &spec.NodePool_DynamicNodePool{
												DynamicNodePool: &spec.DynamicNodePool{
													ServerType:      "performance",
													Image:           "latest",
													StorageDiskSize: 50,
													Region:          "local",
													Zone:            "local",
													Count:           3,
													Provider: &spec.Provider{
														SpecName:          "hetzner",
														CloudProviderName: "hetzner-01",
														ProviderType: &spec.Provider_Hetzner{
															Hetzner: &spec.HetznerProvider{
																Token: "test",
															},
														},
														Templates: &spec.TemplateRepository{
															Repository: "/root/",
															Path:       "hetzner",
														},
													},
													PublicKey:  "default",
													PrivateKey: "default",
													Cidr:       "127.0.0.1/24",
												},
											},
											Name: "test-nodepool",
											Nodes: []*spec.Node{
												{
													Name:     "test-node-01",
													Private:  "192.168.0.1",
													Public:   "127.0.0.1",
													NodeType: spec.NodeType_worker,
													Username: "root",
												},
											},
											IsControl: false,
										},
									},
								},
								Roles:       nil,
								Dns:         nil,
								TargetedK8S: "",
							},
						},
					},
				},
				State: &spec.Workflow{
					Status:      spec.Workflow_DONE,
					Description: "Some description",
					Previous: []*spec.FinishedWorkflow{
						{
							Status:          spec.Workflow_ERROR,
							TaskDescription: "Some task description",
							Stage:           "Ansibler",
							Timestamp:       timestamppb.Now(),
						},
					},
				},
				InFlight: &spec.TaskEvent{
					Id:        "random-id",
					Timestamp: timestamppb.Now(),
					Event:     spec.Event_DELETE,
					Task: &spec.Task{
						Do: &spec.Task_Update{
							Update: &spec.Update{
								State: &spec.Update_State{
									K8S:           &spec.K8Scluster{},
									LoadBalancers: []*spec.LBcluster{},
								},
								Delta: &spec.Update_AddedK8SNodes_{
									AddedK8SNodes: &spec.Update_AddedK8SNodes{
										NewNodePool: false,
										Nodepool:    "test-nodepool",
										Nodes:       []string{"random", "random-1"},
									},
								},
							},
						},
					},
					Description: "Random description",
					Pipeline: []*spec.Stage{
						{
							StageKind: &spec.Stage_Ansibler{
								Ansibler: &spec.StageAnsibler{
									Description: &spec.StageDescription{
										About:      "about",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
									SubPasses: []*spec.StageAnsibler_SubPass{
										{
											Kind: spec.StageAnsibler_CLEAR_PROXY_ENVS_ON_NODES,
											Description: &spec.StageDescription{
												About:      "about inner",
												ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
											},
										},
									},
								},
							},
						},
						{
							StageKind: &spec.Stage_Terraformer{
								Terraformer: &spec.StageTerraformer{
									Description: &spec.StageDescription{
										About:      "about",
										ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
									},
									SubPasses: []*spec.StageTerraformer_SubPass{
										{
											Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE,
											Description: &spec.StageDescription{
												About:      "about inner",
												ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
											},
										},
									},
								},
							},
						},
					},
					CurrentStage: 1,
					LowerPriority: &spec.TaskEvent{
						Id:            "lower-priority",
						Timestamp:     timestamppb.Now(),
						Event:         spec.Event_CREATE,
						Task:          &spec.Task{},
						Description:   "other description",
						Pipeline:      []*spec.Stage{},
						CurrentStage:  0,
						LowerPriority: nil,
					},
				},
			},
		},
	}

	dbrepr, err := store.ConvertFromGRPC(want)
	if err != nil {
		t.Errorf("failed to convert from GRPC to Database representation: %v", err)
	}

	got, err := store.ConvertToGRPC(dbrepr)
	if err != nil {
		t.Errorf("failed to convert from Database to GRPC representation: %v", err)
	}

	if diff := cmp.Diff(
		want,
		got,
		protocmp.Transform(),
		protocmp.IgnoreFields(&timestamppb.Timestamp{}, "nanos"),
	); diff != "" {
		t.Errorf("Conversion GRPC->DB->GRPC failed\ndiff %v", diff)
	}
}

func TestConvertFromGRPCCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *spec.K8Scluster
	}{
		{
			name: "full cluster",
			input: &spec.K8Scluster{
				ClusterInfo: &spec.ClusterInfo{
					Name: "test-k8s",
					Hash: "abc123",
					NodePools: []*spec.NodePool{
						{Name: "control-pool"},
						{Name: "worker-pool"},
					},
				},
				Network:    "192.168.0.0/16",
				Kubeconfig: "apiVersion: v1\nclusters: []",
				Kubernetes: "v1.28.0",
				InstallationProxy: &spec.InstallationProxy{
					Mode:     "on",
					Endpoint: "http://proxy:8080",
					NoProxy:  "10.0.0.0/8",
				},
			},
		},
		{
			name: "minimal cluster",
			input: &spec.K8Scluster{
				ClusterInfo: &spec.ClusterInfo{
					Name: "minimal",
					Hash: "min123",
				},
				Kubernetes: "v1.29.0",
			},
		},
		{
			name:  "empty cluster",
			input: &spec.K8Scluster{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := store.ConvertFromGRPCCluster(tt.input)
			assert.NoError(t, err)
			assert.NotNil(t, data)

			got, err := store.ConvertToGRPCCluster(data)
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.input, got, protocmp.Transform()); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertToGRPCCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		want    *spec.K8Scluster
		wantErr bool
	}{
		{
			name: "valid bytes",
			input: func() []byte {
				c := &spec.K8Scluster{
					ClusterInfo: &spec.ClusterInfo{Name: "test", Hash: "h1"},
					Kubernetes:  "v1.28.0",
				}
				b, _ := proto.Marshal(c)
				return b
			}(),
			want: &spec.K8Scluster{
				ClusterInfo: &spec.ClusterInfo{Name: "test", Hash: "h1"},
				Kubernetes:  "v1.28.0",
			},
		},
		{
			name:    "invalid bytes returns error",
			input:   []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa},
			wantErr: true,
		},
		{
			name:  "nil bytes unmarshals to empty cluster",
			input: nil,
			want:  &spec.K8Scluster{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := store.ConvertToGRPCCluster(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertFromGRPCLoadBalancers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *spec.LoadBalancers
	}{
		{
			name: "multiple load balancers",
			input: &spec.LoadBalancers{
				Clusters: []*spec.LBcluster{
					{
						ClusterInfo: &spec.ClusterInfo{Name: "lb-1", Hash: "lbh1"},
						TargetedK8S: "test-k8s",
						Roles: []*spec.Role{
							{
								Name:     "api",
								Protocol: "tcp",
								Port:     6443,
								RoleType: spec.RoleType_ApiServer,
								Settings: &spec.Role_Settings{
									ProxyProtocol:  true,
									StickySessions: false,
									EnvoyAdminPort: 9901,
								},
							},
							{
								Name:       "ingress",
								Protocol:   "tcp",
								Port:       443,
								TargetPort: 30443,
								RoleType:   spec.RoleType_Ingress,
							},
						},
						Dns:             &spec.DNS{},
						UsedApiEndpoint: true,
					},
					{
						ClusterInfo: &spec.ClusterInfo{Name: "lb-2", Hash: "lbh2"},
						TargetedK8S: "test-k8s",
						Roles: []*spec.Role{
							{
								Name:       "ingress",
								Protocol:   "tcp",
								Port:       80,
								TargetPort: 30080,
								RoleType:   spec.RoleType_Ingress,
							},
						},
					},
				},
			},
		},
		{
			name: "single load balancer",
			input: &spec.LoadBalancers{
				Clusters: []*spec.LBcluster{
					{
						ClusterInfo: &spec.ClusterInfo{Name: "lb-solo", Hash: "solo1"},
						TargetedK8S: "k8s-solo",
					},
				},
			},
		},
		{
			name:  "empty load balancers",
			input: &spec.LoadBalancers{},
		},
		{
			name: "no clusters",
			input: &spec.LoadBalancers{
				Clusters: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := store.ConvertFromGRPCLoadBalancers(tt.input)
			assert.NoError(t, err)
			assert.NotNil(t, data)

			got, err := store.ConvertToGRPCLoadBalancers(data)
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.input, got, protocmp.Transform()); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertToGRPCLoadBalancers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		want    *spec.LoadBalancers
		wantErr bool
	}{
		{
			name: "valid bytes",
			input: func() []byte {
				lbs := &spec.LoadBalancers{
					Clusters: []*spec.LBcluster{
						{
							ClusterInfo: &spec.ClusterInfo{Name: "lb-test", Hash: "lbt1"},
							TargetedK8S: "k8s-test",
							Roles: []*spec.Role{
								{
									Name:     "api",
									Protocol: "tcp",
									Port:     6443,
									RoleType: spec.RoleType_ApiServer,
								},
							},
						},
					},
				}
				b, _ := proto.Marshal(lbs)
				return b
			}(),
			want: &spec.LoadBalancers{
				Clusters: []*spec.LBcluster{
					{
						ClusterInfo: &spec.ClusterInfo{Name: "lb-test", Hash: "lbt1"},
						TargetedK8S: "k8s-test",
						Roles: []*spec.Role{
							{
								Name:     "api",
								Protocol: "tcp",
								Port:     6443,
								RoleType: spec.RoleType_ApiServer,
							},
						},
					},
				},
			},
		},
		{
			name:    "invalid bytes returns error",
			input:   []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa},
			wantErr: true,
		},
		{
			name:  "nil bytes unmarshals to empty load balancers",
			input: nil,
			want:  &spec.LoadBalancers{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := store.ConvertToGRPCLoadBalancers(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
