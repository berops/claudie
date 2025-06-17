package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/berops/claudie/proto/pb"
	builder "github.com/berops/claudie/services/builder/internal"
	"github.com/rs/zerolog"
)

var (
	// ErrFailedToBuildInfrastructure is returned when the infra fails to build in terraformer
	// including any partial failures.
	ErrFailedToBuildInfrastructure = errors.New("failed to successfully build desired state")
)

// reconcileInfrastructure reconciles the desired infrastructure via terraformer.
func (u *Usecases) reconcileInfrastructure(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	res, err := u.Terraformer.BuildInfrastructure(work, u.Terraformer.GetClient())
	if err != nil {
		return fmt.Errorf("error while reconciling infrastructure for cluster %s project %s : %w", work.GetClusterName(), work.ProjectName, err)
	}

	switch resp := res.Response.(type) {
	case *pb.BuildInfrastructureResponse_Fail:
		work.DesiredCluster = resp.Fail.Desired
		work.DesiredLoadbalancers = resp.Fail.DesiredLbs
		return ErrFailedToBuildInfrastructure
	case *pb.BuildInfrastructureResponse_Ok:
		work.DesiredCluster = resp.Ok.Desired
		work.DesiredLoadbalancers = resp.Ok.DesiredLbs
	}

	return nil
}

// destroyInfrastructure destroys the current infrastructure via terraformer.
func (u *Usecases) destroyInfrastructure(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	if _, err := u.Terraformer.DestroyInfrastructure(work, u.Terraformer.GetClient()); err != nil {
		return fmt.Errorf("error while destroying infrastructure for cluster %s project %s : %w", work.GetClusterName(), work.ProjectName, err)
	}
	return nil
}
