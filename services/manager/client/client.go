package managerclient

import (
	"context"
	"errors"
	"fmt"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ ManagerClient = (*Client)(nil)

type Client struct {
	conn   *grpc.ClientConn
	client pb.ManagerServiceClient
	logger *zerolog.Logger
}

func New(logger *zerolog.Logger) (*Client, error) {
	conn, err := utils.GrpcDialWithRetryAndBackoff("manager", envs.ManagerURL)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: pb.NewManagerServiceClient(conn), logger: logger}, nil
}

func (t *Client) Close() error { return t.conn.Close() }

func (t *Client) HealthCheck() error {
	if err := utils.IsConnectionReady(t.conn); err == nil {
		return nil
	} else {
		t.conn.Connect()
		return err
	}
}

func (t *Client) NextTask(ctx context.Context) (*NextTaskResponse, error) {
	resp, err := t.client.NextTask(ctx, new(pb.NextTaskRequest))
	if err == nil {
		return &NextTaskResponse{
			State:   resp.State,
			Config:  resp.Config,
			Cluster: resp.Cluster,
			TTL:     resp.Ttl,
			Current: resp.Current,
			Event:   resp.Event,
		}, nil
	}

	if e, ok := status.FromError(err); ok {
		switch e.Code() {
		case codes.NotFound:
			return nil, nil
		case codes.Aborted:
			return nil, ErrVersionMismatch
		}
	}

	t.logger.Debug().Msgf("Received error %v while calling NextTask", err.Error())
	return nil, err
}

func (t *Client) MarkForDeletion(ctx context.Context, request *MarkForDeletionRequest) error {
	// fetch latest document version before marking for deletion.
	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: request.Name})
	if err == nil {
		_, err := t.client.MarkForDeletion(ctx, &pb.MarkForDeletionRequest{Name: request.Name, Version: current.Config.Version})
		if err == nil {
			return nil
		}
		if e, ok := status.FromError(err); ok && e.Code() == codes.Aborted {
			err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
		}
		t.logger.Debug().Msgf("Received error %v while calling MarkForDeletion", err.Error())
		return err
	}

	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", request.Name)
		return fmt.Errorf("no config with name %q found", request.Name)
	}

	t.logger.Debug().Msgf("Received error %v while calling MarkForDeletion", err.Error())
	return err
}

func (t *Client) UpsertManifest(ctx context.Context, request *UpsertManifestRequest) error {
	req := &pb.UpsertManifestRequest{Name: request.Name}
	if request.K8sCtx != nil {
		req.K8SCtx = &spec.KubernetesContext{Name: request.K8sCtx.Name, Namespace: request.K8sCtx.Namespace}
	}
	if request.Manifest != nil {
		req.Manifest = &spec.Manifest{Raw: request.Manifest.Raw}
	}

	_, err := t.client.UpsertManifest(ctx, req)
	if err == nil {
		return nil
	}

	if e, ok := status.FromError(err); ok && e.Code() == codes.Aborted {
		err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
	}
	t.logger.Debug().Msgf("Received error %v while calling UpsertManifest", err.Error())
	return err
}

func (t *Client) GetConfig(ctx context.Context, request *GetConfigRequest) (*GetConfigResponse, error) {
	resp, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: request.Name})
	if err == nil {
		return &GetConfigResponse{Config: resp.Config}, nil
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", request.Name)
		return nil, fmt.Errorf("no config with name %q found", request.Name)
	}
	t.logger.Debug().Msgf("Received error %v while calling UpsertManifest", err.Error())
	return nil, err
}

func (t *Client) TaskUpdate(ctx context.Context, req *TaskUpdateRequest) error {
	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: req.Config})
	if err == nil {
		_, err := t.client.TaskUpdate(ctx, &pb.TaskUpdateRequest{
			Config:  req.Config,
			Cluster: req.Cluster,
			TaskId:  req.TaskId,
			Version: current.Config.Version,
			State:   req.State,
		})
		if err == nil {
			return nil
		}
		if e, ok := status.FromError(err); ok && e.Code() == codes.Aborted {
			err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
		}
		t.logger.Debug().Msgf("Received error %v while calling TaskUpdate", err.Error())
		return err
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", req.Config)
		return fmt.Errorf("no config with name %q found", req.Config)
	}
	return err
}

func (t *Client) UpdateCurrentState(ctx context.Context, req *UpdateCurrentStateRequest) error {
	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: req.Config})
	if err == nil {
		_, err := t.client.UpdateCurrentState(ctx, &pb.UpdateCurrentStateRequest{
			Name:    req.Config,
			Cluster: req.Cluster,
			Version: current.Config.Version,
			State:   req.Clusters,
		})
		if err == nil {
			return nil
		}
		if e, ok := status.FromError(err); ok && e.Code() == codes.Aborted {
			err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
		}
		t.logger.Debug().Msgf("Received error %v while calling UpdateCurrentState", err.Error())
		return err
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", req.Config)
		return fmt.Errorf("no config with name %q found", req.Config)
	}
	return err
}
