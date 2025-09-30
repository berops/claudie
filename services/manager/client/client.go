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
	client pb.ManagerServiceClient
	logger *zerolog.Logger
}

func New(logger *zerolog.Logger) (*Client, error) {
	conn, err := grpcutils.GrpcDialWithRetryAndBackoff("manager", envs.ManagerURL)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: pb.NewManagerServiceClient(conn), logger: logger}, nil
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

func (t *Client) NextTask(ctx context.Context) (*NextTaskResponse, error) {
	resp, err := t.client.NextTask(ctx, new(pb.NextTaskRequest))
	if err == nil {
		return &NextTaskResponse{
			State:   resp.State,
			Config:  resp.Name,
			Cluster: resp.Cluster,
			Current: resp.Current,
			Event:   resp.Event,
			Lease: Lease{
				// The Lease structure will always be present
				// see handler_next_task.go:[pb.ManagerServiceServer.NextTask] function.
				TaskLeaseTime: resp.Lease.TaskLeaseTime,
			},
		}, nil
	}

	if e, ok := status.FromError(err); ok {
		switch e.Code() {
		case codes.NotFound:
			return nil, fmt.Errorf("%w: no task scheduled or config deleted in the meanwhile", ErrNotFound)
		case codes.Aborted:
			return nil, fmt.Errorf("%w: %w", ErrVersionMismatch, err)
		}
	}

	t.logger.Debug().Msgf("Received error %v while calling NextTask", err)
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
	t.logger.Debug().Msgf("Received error %v while calling UpsertManifest", err)
	return err
}

func (t *Client) ListConfigs(ctx context.Context, _ *ListConfigRequest) (*ListConfigResponse, error) {
	resp, err := t.client.ListConfigs(ctx, new(pb.ListConfigRequest))
	if err == nil {
		return &ListConfigResponse{Config: resp.Configs}, nil
	}
	t.logger.Debug().Msgf("Received error %v while calling ListConfigs", err)
	return nil, err
}

func (t *Client) GetConfig(ctx context.Context, request *GetConfigRequest) (*GetConfigResponse, error) {
	resp, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: request.Name})
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

func (t *Client) TaskUpdate(ctx context.Context, req *TaskUpdateRequest) error {
	if req.Action.Refresh != nil && req.Action.State != nil {
		return fmt.Errorf("only one action at a time can be specified")
	}
	if req.Action == taskUpdateNoAction {
		return fmt.Errorf("no action specified, required one of Refresh or State Update")
	}

	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: req.Config})
	if err == nil {
		payload := &pb.TaskUpdateRequest{
			Name:    req.Config,
			Cluster: req.Cluster,
			TaskId:  req.TaskId,
			Version: current.Config.Version,
			Action:  nil,
		}

		if req.Action.Refresh != nil {
			payload.Action = &pb.TaskUpdateRequest_Refresh_{Refresh: new(pb.TaskUpdateRequest_Refresh)}
		}
		if req.Action.State != nil {
			payload.Action = &pb.TaskUpdateRequest_State{State: req.Action.State}
		}

		if payload.Action == nil {
			return fmt.Errorf("no action specified, required one of Refresh or State Update")
		}

		_, err := t.client.TaskUpdate(ctx, payload)
		if err == nil {
			return nil
		}

		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.NotFound:
				err = errors.Join(err, fmt.Errorf("combination config %q cluster %q task %q: %w", req.Config, req.Cluster, req.TaskId, ErrNotFound))
			case codes.Aborted:
				err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
			}
		}

		t.logger.Debug().Msgf("Received error %v while calling TaskUpdate", err)
		return err
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", req.Config)
		return fmt.Errorf("config with name %q: %w", req.Config, ErrNotFound)
	}
	return err
}

func (t *Client) TaskComplete(ctx context.Context, req *TaskCompleteRequest) error {
	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: req.Config})
	if err == nil {
		_, err := t.client.TaskComplete(ctx, &pb.TaskCompleteRequest{
			Name:     req.Config,
			Cluster:  req.Cluster,
			TaskId:   req.TaskId,
			Version:  current.Config.Version,
			Workflow: req.Workflow,
			State:    req.State,
		})
		if err == nil {
			return nil
		}

		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.NotFound:
				err = errors.Join(err, fmt.Errorf("combination config %q cluster %q task %q: %w", req.Config, req.Cluster, req.TaskId, ErrNotFound))
			case codes.Aborted:
				err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
			}
		}

		t.logger.Debug().Msgf("Received error %v while calling TaskComplete", err)
		return err
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", req.Config)
		return fmt.Errorf("config with name %q: %w", req.Config, ErrNotFound)
	}
	return err
}

func (t *Client) UpdateNodePool(ctx context.Context, req *UpdateNodePoolRequest) error {
	current, err := t.client.GetConfig(ctx, &pb.GetConfigRequest{Name: req.Config})
	if err == nil {
		_, err := t.client.UpdateNodePool(ctx, &pb.UpdateNodePoolRequest{
			Name:     req.Config,
			Cluster:  req.Cluster,
			Version:  current.Config.Version,
			Nodepool: req.NodePool,
		})
		if err == nil {
			return nil
		}

		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.NotFound:
				err = errors.Join(err, fmt.Errorf("combination config %q cluster %q nodepool %q: %w", req.Config, req.Cluster, req.NodePool.GetName(), ErrNotFound))
			case codes.Aborted:
				err = errors.Join(err, fmt.Errorf("%w", ErrVersionMismatch))
			}
		}

		t.logger.Debug().Msgf("Received error %v while calling UpdateNodePool", err)
		return err
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
		t.logger.Debug().Msgf("GetConfig(): no config with name %q found", req.Config)
		return fmt.Errorf("config with name %q: %w", req.Config, ErrNotFound)
	}
	return err
}
