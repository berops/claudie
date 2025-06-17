package usecases

import (
	"context"
	"os"
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

var _ managerclient.ClientAPI = fakeManager{}

type fakeManager struct{}

func (m fakeManager) Close() error                                                       { return nil }
func (m fakeManager) HealthCheck() error                                                 { return nil }
func (m fakeManager) TaskUpdate(context.Context, *managerclient.TaskUpdateRequest) error { return nil }

func (m fakeManager) TaskComplete(context.Context, *managerclient.TaskCompleteRequest) error {
	return nil
}

func (m fakeManager) NextTask(context.Context) (*managerclient.NextTaskResponse, error) {
	return nil, nil
}

func (m fakeManager) UpsertManifest(context.Context, *managerclient.UpsertManifestRequest) error {
	return nil
}

func (m fakeManager) MarkForDeletion(context.Context, *managerclient.MarkForDeletionRequest) error {
	return nil
}

func (m fakeManager) GetConfig(context.Context, *managerclient.GetConfigRequest) (*managerclient.GetConfigResponse, error) {
	return nil, nil
}

func (m fakeManager) ListConfigs(context.Context, *managerclient.ListConfigRequest) (*managerclient.ListConfigResponse, error) {
	return nil, nil
}

func (m fakeManager) UpdateNodePool(context.Context, *managerclient.UpdateNodePoolRequest) error {
	return nil
}

func TestTaskCondition(t *testing.T) {
	currentTask := ""
	tasks := []Task{
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				currentTask = "current kubernetes cluster"
				return nil
			},
			stage:           0,
			description:     "current kubernetes cluster",
			continueOnError: false,
			condition:       func(work *builder.Context) bool { return work.CurrentCluster != nil },
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				currentTask = "loadbalancers cluster"
				return nil
			},
			stage:           0,
			description:     "loadbalancers cluster",
			continueOnError: false,
			condition:       nil,
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				currentTask = "ansible playbooks"
				return nil
			},
			stage:           0,
			description:     "ansible playbooks",
			continueOnError: false,
			condition:       nil,
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				currentTask = "setup autoscaler desired"
				return nil
			},
			stage:           0,
			description:     "setup autoscaler desired",
			continueOnError: false,
			condition:       func(work *builder.Context) bool { return work.DesiredCluster.AnyAutoscaledNodePools() },
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				currentTask = "patch nodes"
				return nil
			},
			stage:           0,
			description:     "patch nodes",
			continueOnError: false,
			condition:       nil,
		},
	}

	executeAll := &builder.Context{
		ProjectName:    "test",
		TaskId:         "1",
		Workflow:       new(spec.Workflow),
		CurrentCluster: new(spec.K8Scluster),
		DesiredCluster: &spec.K8Scluster{
			ClusterInfo: &spec.ClusterInfo{
				NodePools: []*spec.NodePool{
					{
						Type: &spec.NodePool_DynamicNodePool{
							DynamicNodePool: &spec.DynamicNodePool{
								AutoscalerConfig: &spec.AutoscalerConf{
									Min: 0,
									Max: 5,
								},
							},
						},
						Name: "dyn",
					},
				},
			},
		},
	}

	var descriptions []string
	for _, t := range tasks {
		descriptions = append(descriptions, t.description)
	}

	// test that all tasks are executed if all of the conditions hold.
	logger := zerolog.New(os.Stdout)
	u := Usecases{Manager: fakeManager{}}
	for i, tt := range tasks {
		err := u.tryProcessTask(context.Background(), executeAll, &logger, tt)
		assert.NoError(t, err)
		assert.Equal(t, descriptions[i], currentTask)
	}

	// test that the task with the failed condition is skipped.
	descriptions[0], currentTask = "", "" // first condition will fail here.
	firstFails := executeAll
	firstFails.CurrentCluster = nil
	for i, tt := range tasks {
		err := u.tryProcessTask(context.Background(), executeAll, &logger, tt)
		assert.NoError(t, err)
		assert.Equal(t, descriptions[i], currentTask)
	}

	// test that the second to last task is skipped.
	descriptions[0] = tasks[0].description
	secondToLastFails := firstFails
	secondToLastFails.CurrentCluster = new(spec.K8Scluster)
	secondToLastFails.DesiredCluster.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig = nil
	descriptions[len(descriptions)-2] = ""
	for i, tt := range tasks {
		if i == len(descriptions)-2 {
			currentTask = ""
		}
		err := u.tryProcessTask(context.Background(), executeAll, &logger, tt)
		assert.NoError(t, err)
		assert.Equal(t, descriptions[i], currentTask)
	}
}
