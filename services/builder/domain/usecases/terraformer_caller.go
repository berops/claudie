package usecases

import (
	"errors"
	"fmt"
	"strings"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

var (
	// ErrFailedToBuildInfrastructure is returned when the infra fails to build in terraformer
	// including any partial failures.
	ErrFailedToBuildInfrastructure = errors.New("failed to successfully build desired state")
)

// reconcileInfrastructure reconciles the desired infrastructure via terraformer.
func (u *Usecases) reconcileInfrastructure(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	// Set workflow state.
	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_TERRAFORMER
	ctx.Workflow.Description = strings.TrimSpace(fmt.Sprintf("%s building infrastructure", description))
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling BuildInfrastructure on Terraformer")
	res, err := u.Terraformer.BuildInfrastructure(ctx, u.Terraformer.GetClient())
	if err != nil {
		return fmt.Errorf("error while reconciling infrastructure for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	switch resp := res.GetResponse().(type) {
	case *pb.BuildInfrastructureResponse_Fail:
		logger.Error().Msgf("failed to build %s", resp.Fail.Failed)
		ctx.CurrentCluster = resp.Fail.Current
		ctx.DesiredCluster = resp.Fail.Desired
		ctx.CurrentLoadbalancers = resp.Fail.CurrentLbs
		ctx.DesiredLoadbalancers = resp.Fail.DesiredLbs

		return ErrFailedToBuildInfrastructure
	case *pb.BuildInfrastructureResponse_Ok:
		ctx.CurrentCluster = resp.Ok.Current
		ctx.DesiredCluster = resp.Ok.Desired
		ctx.CurrentLoadbalancers = resp.Ok.CurrentLbs
		ctx.DesiredLoadbalancers = resp.Ok.DesiredLbs
	}

	// Set description to original string.
	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("BuildInfrastructure on Terraformer finished successfully")
	return nil
}

// destroyInfrastructure destroys the current infrastructure via terraformer.
func (u *Usecases) destroyInfrastructure(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	// Set workflow state.
	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_DESTROY_TERRAFORMER
	ctx.Workflow.Description = fmt.Sprintf("%s destroying infrastructure", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling DestroyInfrastructure on Terraformer")

	if _, err := u.Terraformer.DestroyInfrastructure(ctx, u.Terraformer.GetClient()); err != nil {
		return fmt.Errorf("error while destroying infrastructure for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Set description to original string.
	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	logger.Info().Msg("DestroyInfrastructure on Terraformer finished successfully")
	return nil
}
