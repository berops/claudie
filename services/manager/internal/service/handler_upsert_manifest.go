package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"
)

func (s *Service) UpsertManifest(ctx context.Context, request *pb.UpsertManifestRequest) (*pb.UpsertManifestResponse, error) {
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

	request.Manifest.Checksum = hash.Digest(request.Manifest.Raw)

	dbConfig, err := s.store.GetConfig(ctx, request.Name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q: %v", request.Name, err)
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

		if err := s.store.CreateConfig(ctx, &newConfig); err != nil {
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

	c, err := s.store.ListConfigs(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error validating config %q: %v", request.Name, err)
	}

	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(request.Manifest.Raw), &m); err != nil {
		return nil, status.Errorf(codes.Internal, "error validating config %q: %v", request.Name, err)
	}

	if err := validateStaticNodePools(&m, c); err != nil {
		return nil, status.Errorf(codes.Internal, "error validating config %q: %v", request.Name, err)
	}

	if err := s.store.UpdateConfig(ctx, dbConfig); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "error while saving config %q in database: %v", request.Name, err)
	}

	log.Info().Msgf("Config %q successfully saved", request.Name)

	return &pb.UpsertManifestResponse{Name: dbConfig.Name, Version: dbConfig.Version}, nil
}

// Check if the new config has reference to already existing static nodes.
func validateStaticNodePools(m *manifest.Manifest, allConfigs []*store.Config) error {
	manifestStaticIPs := collectIPs(m)

	for _, cfg := range allConfigs {
		if cfg.Name == m.Name {
			continue
		}

		var other manifest.Manifest
		if err := yaml.Unmarshal([]byte(cfg.Manifest.Raw), &other); err != nil {
			return fmt.Errorf("config %q failed to unmarshal manifest: %w", cfg.Name, err)
		}
		otherStaticIPs := collectIPs(&other)

		if match := ipMatch(manifestStaticIPs, otherStaticIPs); match != "" {
			return fmt.Errorf("reference to the same static node with IP %q referenced in the newly added config %q is already in use by config %q, reference to the same static node across different clusters/configs is discouraged as it can lead to corrupt state of the cluster", match, m.Name, cfg.Name)
		}
	}

	return nil
}

func ipMatch(first, second map[string]struct{}) string {
	for checkIP := range first {
		if _, ok := second[checkIP]; ok {
			return checkIP
		}
	}
	return ""
}

func collectIPs(m *manifest.Manifest) map[string]struct{} {
	nodepools := make(map[string]struct{})

	for _, snp := range m.NodePools.Static {
		for _, node := range snp.Nodes {
			nodepools[node.Endpoint] = struct{}{}
		}
	}

	return nodepools
}
