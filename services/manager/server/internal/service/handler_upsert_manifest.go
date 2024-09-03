package service

import (
	"bytes"
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/berops/claudie/internal/checksum"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/server/internal/store"
	"github.com/rs/zerolog/log"
)

func (g *GRPC) UpsertManifest(ctx context.Context, request *pb.UpsertManifestRequest) (*pb.UpsertManifestResponse, error) {
	log.Debug().Msgf("Received Config to store: %v", request.Name)

	if request.Manifest == nil {
		return nil, status.Errorf(codes.InvalidArgument, "no supplied manifest to build")
	}
	if request.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if request.Manifest.Raw == "" {
		return nil, status.Errorf(codes.InvalidArgument, "cannot update manifest with empty string")
	}

	request.Manifest.Checksum = checksum.Digest(request.Manifest.Raw)

	dbConfig, err := g.Store.GetConfig(ctx, request.Name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existance for config %q: %v", request.Name, err)
		}

		newConfig := store.Config{
			Version: 0,
			Name:    request.Name,
			K8SCtx: store.KubernetesContext{
				Name:      request.GetK8SCtx().GetName(),
				Namespace: request.GetK8SCtx().GetNamespace(),
			},
			Manifest: store.Manifest{
				Raw:      request.Manifest.Raw,
				Checksum: request.Manifest.Checksum,
				State:    request.Manifest.State.String(),
			},
		}

		if err := g.Store.CreateConfig(ctx, &newConfig); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create document for config %q: %v", newConfig.Name, err)
		}

		return &pb.UpsertManifestResponse{Name: newConfig.Name, Version: newConfig.Version}, nil
	}

	if !bytes.Equal(dbConfig.Manifest.Checksum, request.Manifest.Checksum) {
		dbConfig.Manifest.Raw = request.Manifest.Raw
		dbConfig.Manifest.Checksum = request.Manifest.Checksum
		dbConfig.K8SCtx.Name = request.GetK8SCtx().GetName()
		dbConfig.K8SCtx.Namespace = request.GetK8SCtx().GetNamespace()
	}

	if err := g.Store.UpdateConfig(ctx, dbConfig); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "error while saving config %q in database: %v", request.Name, err)
	}

	log.Info().Msgf("Config %q sucessfully saved", request.Name)

	return &pb.UpsertManifestResponse{Name: dbConfig.Name, Version: dbConfig.Version}, nil
}
