package service

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *GRPC) ListConfigs(ctx context.Context, _ *pb.ListConfigRequest) (*pb.ListConfigResponse, error) {
	cfgs, err := g.Store.ListConfigs(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query all configs: %v", err)
	}

	var out []*spec.Config
	for _, cfg := range cfgs {
		grpc, err := store.ConvertToGRPC(cfg)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", cfg.Name, err)
		}
		out = append(out, grpc)
	}

	return &pb.ListConfigResponse{Configs: out}, nil
}
