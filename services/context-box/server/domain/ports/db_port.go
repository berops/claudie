package ports

import "github.com/berops/claudie/proto/pb"

type DBPort interface {
	GetConfig(id string, idType pb.IdType) (*pb.Config, error)
	DeleteConfig(id string, idType pb.IdType) error
	GetAllConfigs() ([]*pb.Config, error)
	SaveConfig(config *pb.Config) error
	UpdateSchedulerTTL(name string, newTTL int32) error
	UpdateBuilderTTL(name string, newTTL int32) error
	UpdateMsToNull(id string, idType pb.IdType) error
	UpdateDs(config *pb.Config) error
	UpdateCs(config *pb.Config) error
	UpdateWorkflowState(configName, clusterName string, workflow *pb.Workflow) error
	UpdateAllStates(configName string, state map[string]*pb.Workflow) error
}
