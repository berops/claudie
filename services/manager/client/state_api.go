package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type StateAPI interface {
	// UpdateNodePool will update the nodepool of a cluster within a config. If during an update a dirty
	// write occurs the ErrVersionMismatch error is returned. On a dirty write the application code should execute
	// the Read/Update/Write cycle again. If either one of nodepool, cluster, config is not found the ErrNotFound
	// error is returned.
	UpdateNodePool(ctx context.Context, request *UpdateNodePoolRequest) error
}

type UpdateNodePoolRequest struct {
	Config   string
	Cluster  string
	NodePool *spec.NodePool
}
