package service

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) ListConfigs(ctx context.Context, _ *pb.ListConfigRequestV2) (*pb.ListConfigResponseV2, error) {
	cfgs, err := s.store.ListConfigs(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query all configs: %v", err)
	}

	var out []*spec.ConfigV2
	for _, cfg := range cfgs {
		grpc, err := store.ConvertToGRPC(cfg)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", cfg.Name, err)
		}
		out = append(out, grpc)
	}

	return &pb.ListConfigResponseV2{Configs: out}, nil
}
