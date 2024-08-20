package utils

//// IntermediateRepresentation represents the intermediate state
//// for a pair of current, desired *pb.K8sClusters>
//type IntermediateRepresentation struct {
//	// IR is the intermediate representation that should be passed through the workflow
//	// before actually building the desired state. If nil there is no in-between step.
//	IR *spec.K8Scluster
//
//	// IRLbs are the intermediate representations of LB clusters that should be passed through the workflow
//	// before actually building the desired state of the LB clusters. If nil there is no in-between step.
//	IRLbs []*spec.LBcluster
//
//	// ToDelete are the nodepools from which nodes needs to be deleted. This may be set
//	// even if IR is nil.
//	ToDelete map[string]int32
//
//	// ControlPlaneWithAPIEndpointReplace if this is set it means that the nodepool in current state
//	// which has an ApiEndpoint is deleted in desired state and needs to be updated
//	// before executing the workflow for the desired state and before deleting the nodes
//	// from the ToDelete.
//	ControlPlaneWithAPIEndpointReplace bool
//}

// Stages returns the number of individual stages. Useful for logging.
//func (ir *IntermediateRepresentation) Stages() int {
//	count := 0
//
//	if ir.IR != nil {
//		count++
//	}
//
//	if len(ir.ToDelete) > 0 {
//		count++
//	}
//
//	if ir.ControlPlaneWithAPIEndpointReplace {
//		count++
//	}
//
//	return count
//}
