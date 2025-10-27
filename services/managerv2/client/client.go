package managerclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ ClientAPI = (*Client)(nil)

type Client struct {
	conn   *grpc.ClientConn
	client pb.ManagerV2ServiceClient
	logger *zerolog.Logger
}

func New(logger *zerolog.Logger) (*Client, error) {
	conn, err := grpcutils.GrpcDialWithRetryAndBackoff("manager", envs.ManagerURL)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: pb.NewManagerV2ServiceClient(conn), logger: logger}, nil
}

func (t *Client) Close() error { return t.conn.Close() }

func (t *Client) HealthCheck() error {
	if err := grpcutils.IsConnectionReady(t.conn); err == nil {
		return nil
	} else {
		t.conn.Connect()
		return err
	}
}

func (t *Client) MarkForDeletion(ctx context.Context, request *MarkForDeletionRequest) error {
	// fetch latest document version before marking for deletion.
	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequestV2{Name: request.Name})
	if err == nil {
		_, err := t.client.MarkForDeletion(ctx, &pb.MarkForDeletionRequestV2{Name: request.Name, Version: current.Config.Version})
		if err == nil {
			return nil
		}
		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.NotFound:
				err = errors.Join(err, fmt.Errorf("config %q: %w", request.Name, ErrNotFound))
			case codes.Aborted:
				err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
			}
		}

		t.logger.Debug().Msgf("Received error %v while calling MarkForDeletion", err)
		return err
	}

	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", request.Name)
		return fmt.Errorf("config with name %q: %w", request.Name, ErrNotFound)
	}

	t.logger.Debug().Msgf("Received error %v while calling MarkForDeletion", err)
	return err
}

func (t *Client) UpsertManifest(ctx context.Context, request *UpsertManifestRequest) error {
	req := &pb.UpsertManifestRequestV2{Name: request.Name}
	if request.K8sCtx != nil {
		req.K8SCtx = &spec.KubernetesContextV2{Name: request.K8sCtx.Name, Namespace: request.K8sCtx.Namespace}
	}
	if request.Manifest != nil {
		req.Manifest = &spec.ManifestV2{Raw: request.Manifest.Raw}
	}

	_, err := t.client.UpsertManifest(ctx, req)
	if err == nil {
		return nil
	}

	if e, ok := status.FromError(err); ok && e.Code() == codes.Aborted {
		err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
	}
	t.logger.Debug().Msgf("Received error %v while calling UpsertManifest", err)
	return err
}

func (t *Client) ListConfigs(ctx context.Context, _ *ListConfigRequest) (*ListConfigResponse, error) {
	resp, err := t.client.ListConfigs(ctx, new(pb.ListConfigRequestV2))
	if err == nil {
		return &ListConfigResponse{Config: resp.Configs}, nil
	}
	t.logger.Debug().Msgf("Received error %v while calling ListConfigs", err)
	return nil, err
}

func (t *Client) GetConfig(ctx context.Context, request *GetConfigRequest) (*GetConfigResponse, error) {
	resp, err := t.client.GetConfig(ctx, &pb.GetConfigRequestV2{Name: request.Name})
	if err == nil {
		return &GetConfigResponse{Config: resp.Config}, nil
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", request.Name)
		return nil, fmt.Errorf("config with name %q: %w", request.Name, ErrNotFound)
	}
	t.logger.Debug().Msgf("Received error %v while calling GetConfig", err)
	return nil, err
}
