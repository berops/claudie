package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type CrudAPI interface {
	// GetConfig will query the config with the specified name. If the requested
	// config is not found the ErrNotFound error is returned.
	GetConfig(ctx context.Context, request *GetConfigRequest) (*GetConfigResponse, error)
	// ListConfigs will query all the configs the manager handles.
	ListConfigs(ctx context.Context, request *ListConfigRequest) (*ListConfigResponse, error)
}

type GetConfigRequest struct{ Name string }
type GetConfigResponse struct{ Config *spec.ConfigV2 }

type ListConfigRequest struct{}
type ListConfigResponse struct{ Config []*spec.ConfigV2 }
