package service

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"
)

func (s *Service) GetConfig(ctx context.Context, request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Debug().Msgf("Received request for config: %q", request.Name)

	cfg, err := s.store.GetConfig(ctx, request.Name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q: %v", request.Name, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q found", request.Name)
	}

	resp, err := store.ConvertToGRPC(cfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", request.Name, err)
	}

	return &pb.GetConfigResponse{Config: resp}, nil
}
