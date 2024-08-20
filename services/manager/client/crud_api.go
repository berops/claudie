package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type CrudAPI interface {
	// GetConfig will query the config with the specified name.
	GetConfig(ctx context.Context, request *GetConfigRequest) (*GetConfigResponse, error)
}

type GetConfigRequest struct{ Name string }
type GetConfigResponse struct{ Config *spec.Config }
