package service

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/server/internal/store"
	"github.com/rs/zerolog/log"
)

func (g *GRPC) MarkForDeletion(ctx context.Context, request *pb.MarkForDeletionRequest) (*pb.MarkForDeletionResponse, error) {
	log.Debug().Msgf("Marking config %q with version %v for deletion", request.Name, request.Version)

	if err := g.Store.MarkForDeletion(ctx, request.Name, request.Version); err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to mark config %q with version %v for deletion: %s", request.Name, request.Version, err.Error())
		}

		if _, err := g.Store.GetConfig(ctx, request.Name); err != nil {
			if !errors.Is(err, store.ErrNotFoundOrDirty) {
				return nil, status.Errorf(codes.Internal, "failed to check existence of config %q: %v", request.Name, err)
			}
			return nil, status.Errorf(codes.NotFound, "no config with name %q exists", request.Name)
		}

		log.Warn().Msgf("Couldn't mark config %q with version %v for deletion, dirty write", request.Name, request.Version)

		return nil, status.Errorf(codes.Aborted, "config %q with version %v was not found", request.Name, request.Version)
	}

	log.Info().Msgf("Config %q with version %v successfully marked for deletion", request.Name, request.Version)
	return &pb.MarkForDeletionResponse{Name: request.Name, Version: request.Version}, nil
}
