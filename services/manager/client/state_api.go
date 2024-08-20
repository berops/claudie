package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type StateAPI interface {
	// UpdateCurrentState will update the current state of the cluster within the specified config and version.
	// If the requested config version is not found the ErrVersionMismatch error is returned indicating a Dirty write.
	// On a dirty write the application code should execute the Read/Update/Write cycle again.
	UpdateCurrentState(ctx context.Context, request *UpdateCurrentStateRequest) error
}

type UpdateCurrentStateRequest struct {
	Config   string
	Cluster  string
	Clusters *spec.Clusters
}
