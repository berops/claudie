package usecases

import (
	"errors"
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	terraformer "github.com/berops/claudie/services/terraformer/client"
)

var (
	// ErrFailedToBuildInfrastructure is returned when the infra fails to build in terraformer
	// including any partial failures.
	ErrFailedToBuildInfrastructure = errors.New("failed to successfully build desired state")
)

func (u *Usecases) reconcileInfrastructure(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_TERRAFORMER
	ctx.Workflow.Description = fmt.Sprintf("%s building infrastructure", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling BuildInfrastructure on Terraformer")

	res, err := terraformer.BuildInfrastructure(u.Terraformer.GetClient(),
		&pb.BuildInfrastructureRequest{
			Current:     ctx.CurrentCluster,
			Desired:     ctx.DesiredCluster,
			CurrentLbs:  ctx.CurrentLoadbalancers,
			DesiredLbs:  ctx.DesiredLoadbalancers,
			ProjectName: ctx.ProjectName,
		})
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

	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	logger.Info().Msgf("BuildInfrastructure on Terraformer finished successfully")
	return nil
}

func (u *Usecases) destroyInfrastructure(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_DESTROY_TERRAFORMER
	ctx.Workflow.Description = fmt.Sprintf("%s destroying infrastructure", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling DestroyInfrastructure on Terraformer")

	if _, err := terraformer.DestroyInfrastructure(u.Terraformer.GetClient(),
		&pb.DestroyInfrastructureRequest{
			ProjectName: ctx.ProjectName,
			Current:     ctx.CurrentCluster,
			CurrentLbs:  ctx.CurrentLoadbalancers,
		}); err != nil {
		return fmt.Errorf("error while destroying infrastructure for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}
	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	logger.Info().Msg("DestroyInfrastructure on Terraformer finished successfully")
	return nil
}
