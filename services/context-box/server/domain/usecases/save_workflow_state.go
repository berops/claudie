package usecases

import "github.com/berops/claudie/proto/pb"

// SaveWorkflowState updates the workflow for a single cluster
func (u *Usecases) SaveWorkflowState(request *pb.SaveWorkflowStateRequest) (*pb.SaveWorkflowStateResponse, error) {
	if request.Workflow == nil {
		return &pb.SaveWorkflowStateResponse{}, nil
	}

	err := u.DB.UpdateWorkflowState(request.ConfigName, request.ClusterName, request.Workflow)
	return &pb.SaveWorkflowStateResponse{}, err
}
