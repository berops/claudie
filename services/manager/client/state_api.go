package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type StateAPI interface {
	// UpdateCurrentState will update the current state of the cluster within the specified config and version.
	// If the requested config version is not found the ErrVersionMismatch error is returned indicating a Dirty write.
	// On a dirty write the application code should execute the Read/Update/Write cycle again.
	// If either the cluster of Config is not found the ErrNotFound error is returned.
	UpdateCurrentState(ctx context.Context, request *UpdateCurrentStateRequest) error

	// UpdateNodePool will update the nodepool of a cluster within a config. If during an update a dirty
	// write occurs the ErrVersionMismatch error is returned. On a dirty write the application code should execute
	// the Read/Update/Write cycle again. If either one of nodepool, cluster, config is not found the ErrNotFound
	// error is returned.
	UpdateNodePool(ctx context.Context, request *UpdateNodePoolRequest) error
}

type UpdateCurrentStateRequest struct {
	Config   string
	Cluster  string
	Clusters *spec.Clusters
}

type UpdateNodePoolRequest struct {
	Config   string
	Cluster  string
	NodePool *spec.NodePool
}
