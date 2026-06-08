package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type CrudAPI interface {
	// GetConfig will query the config with the specified name. If the requested
	// config is not found the [ErrNotFound] error is returned.
	GetConfig(ctx context.Context, request *GetConfigRequest) (*GetConfigResponse, error)

	// ListConfigs will query all the configs the manager handles.
	ListConfigs(ctx context.Context, request *ListConfigRequest) (*ListConfigResponse, error)

	// Marks the node for deletion along with updating the 'count' of the nodepool itself
	// if requested.
	//
	// If the requested nodepool/node/config tuple is not found the [ErrNotFound] error is returned.
	//
	// If the change couldn't be handled by the Manager the [ErrVersionMismatch] error is returned
	// in which case the caller should either retry the operation or abort.
	MarkNodeForDeletion(ctx context.Context, request *MarkNodeForDeletionRequest) (*MarkNodeForDeletionResponse, error)

	// NodePoolUpdateTargetSize updates the target size of the nodepool.
	NodePoolUpdateTargetSize(ctx context.Context, request *NodePoolUpdateTargetSizeRequest) (*NodePoolUpdateTargetSizeResponse, error)
}

type GetConfigRequest struct{ Name string }
type GetConfigResponse struct{ Config *spec.Config }

type ListConfigRequest struct{}
type ListConfigResponse struct{ Config []*spec.Config }

type MarkNodeForDeletionRequest struct {
	Config   string
	Cluster  string
	NodePool string
	Node     string

	// If set the node will be searched for in this
	// specified loadbalanacer. The passed in [Cluster]
	// specifies the kuberentes cluster, but loadbalancers
	// are attached to the kubernetes cluster thus the search
	// requires the kubernetes cluster id but makes the loadbalancer
	// id optional.
	LoadBalancer *string

	// This flag only has effect with autoscaled dynamic nodepools,
	// if any other nodepool is detected and this parameter is set
	// it will be ignored.
	//
	// Why only for autoscaled nodepools ?
	//
	// Dynamic and Static nodepools have a fixed desired capacity that
	// does not change until explicitly changed in the InputManifest.
	// So even if this Api endpoint would allow to decrease the capacity
	// for these nodepools on the next iteration of the reconciliation
	// loop it would be overwritten by the desired count and effectively
	// result in a Noop.
	//
	// Autoscaled nodepools work within a range of nodes [Min, Max] and
	// works with two counters for nodes, 'count' which is the current number
	// of nodes within the nodepool and 'targetSize' which is the Target Size
	// within [Min, Max] that the NodePool should have. This parameter is
	// externally managed, i.e. the desired state does not come from the
	// InputManifest, for autoscaled nodepools and thus is also made
	// available via this parameter.
	//
	// This parameter adjusts the 'targetSize' which will result of downscaling.
	ShouldDecrementDesiredCapacity *bool
}

type MarkNodeForDeletionResponse struct {
	// The new desirec capacity of the nodepool.
	TargetSize int64
}

type NodePoolUpdateTargetSizeRequest struct {
	Config   string
	Cluster  string
	NodePool string

	// If set the node will be searched for in this
	// specified loadbalanacer. The passed in [Cluster]
	// specifies the kuberentes cluster, but loadbalancers
	// are attached to the kubernetes cluster thus the search
	// requires the kubernetes cluster id but makes the loadbalancer
	// id optional.
	LoadBalancer *string

	// Specifies the target size of the nodepool.
	//
	// For Dynamic and static nodepools this request or value
	// will be ignored
	//
	// Why ?
	//
	// If this request would change the targetSize it would be
	// overwritten on the next reconciliation loop by the desired
	// state from the InputManifest which would result in a Noop.
	//
	// Only for autoscaled nodepools this will have an effect, which
	// effectively sets the TargetSize within the [Min, Max] range of
	// the autoscaled nodepool.
	TargetSize int32
}

type NodePoolUpdateTargetSizeResponse struct {
	// The new update targetSize
	TargetSize int32
}
