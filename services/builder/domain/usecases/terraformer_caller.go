package usecases

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
)

var (
	// ErrFailedToBuildInfrastructure is returned when the infra fails to build in terraformer
	// including any partial failures.
	ErrFailedToBuildInfrastructure = errors.New("failed to successfully build desired state")
)

// reconcileInfrastructure reconciles the desired infrastructure via terraformer.
func (u *Usecases) reconcileInfrastructure(ctx *builder.Context) error {
	logger := utils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	// Set workflow state.
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_TERRAFORMER, fmt.Sprintf("%s building infrastructure", description))

	logger.Info().Msgf("Calling BuildInfrastructure on Terraformer")
	res, err := u.Terraformer.BuildInfrastructure(ctx, u.Terraformer.GetClient())
	if err != nil {
		return fmt.Errorf("error while reconciling infrastructure for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}
	logger.Info().Msgf("BuildInfrastructure on Terraformer finished successfully")

	switch resp := res.Response.(type) {
	case *pb.BuildInfrastructureResponse_Fail:
		logger.Error().Msgf("failed to build %s", resp.Fail.Failed)
		ctx.DesiredCluster = resp.Fail.Desired
		ctx.DesiredLoadbalancers = resp.Fail.DesiredLbs
		return ErrFailedToBuildInfrastructure
	case *pb.BuildInfrastructureResponse_Ok:
		ctx.DesiredCluster = resp.Ok.Desired
		ctx.DesiredLoadbalancers = resp.Ok.DesiredLbs
	}

	u.updateTaskWithDescription(ctx, spec.Workflow_TERRAFORMER, description)
	return nil
}

// destroyInfrastructure destroys the current infrastructure via terraformer.
func (u *Usecases) destroyInfrastructure(ctx *builder.Context) error {
	logger := utils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	// Set workflow state.
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_DESTROY_TERRAFORMER, fmt.Sprintf("%s destroying infrastructure", description))

	logger.Info().Msg("Calling DestroyInfrastructure on Terraformer")
	if _, err := u.Terraformer.DestroyInfrastructure(ctx, u.Terraformer.GetClient()); err != nil {
		return fmt.Errorf("error while destroying infrastructure for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}
	logger.Info().Msg("DestroyInfrastructure on Terraformer finished successfully")

	// Set description to original string.
	u.updateTaskWithDescription(ctx, spec.Workflow_DESTROY_TERRAFORMER, description)
	return nil
}
