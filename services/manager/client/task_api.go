package managerclient

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
)

type TaskAPI interface {
	// NextTask will fetch the next task from the queue of tasks available at the Manager service.
	// If no tasks are available a nil response and a nil error is returned. If at any point an error
	// occurred an error is returned describing the cause. The API on server performs validation before
	// and initialization before returning the task, thus the error ErrVersionMismatch will be returned in
	// case a dirty write occurred on the server side. On the Client it is simply enough to call the
	// NextTask function again. Once new tasks are scheduled a non-nil response is returned.
	// If no tasks are scheduled or the Config for which the task was scheduled was deleted in the
	// meantime, the ErrNotFound error is returned.
	NextTask(ctx context.Context) (*NextTaskResponse, error)

	// TaskUpdate will update the state of the cluster within the specified config and version. If the requested config version is not
	// found the ErrVersionMismatch error is returned indicating a Dirty write. On a dirty write the application code
	// should execute the Read/Update/Write cycle again. If either the config or the cluster within the config or
	// the task for which the state should be updated is not found the ErrNotFound error is returned.
	TaskUpdate(ctx context.Context, request *TaskUpdateRequest) error
}

type NextTaskResponse struct {
	Config  string
	Cluster string
	TTL     int32
	Current *spec.Clusters
	Event   *spec.TaskEvent
	State   *spec.Workflow
}

type TaskUpdateRequest struct {
	Config  string
	Cluster string
	TaskId  string
	State   *spec.Workflow
}
