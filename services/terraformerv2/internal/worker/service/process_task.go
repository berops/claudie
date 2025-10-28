package service

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type Work struct {
	Task   *spec.TaskV2
	Passes []*spec.StageTerraformer_SubPass
}

func ProcessTask(ctx context.Context, task Work) *spec.TaskResult {
	// TODO: implement the domain and move it here and consider processing as subpasses
	// i.e the recieved message would have subpsasses which would be worked on
	// individually and thus we could cancel them really easily on service kill.

	return &spec.TaskResult{
		Result: &spec.TaskResult_None_{
			None: new(spec.TaskResult_None),
		},
	}
}
