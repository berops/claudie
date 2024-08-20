package ports

//
//type DBPort interface {
//	GetConfig(id string, idType pb.IdType) (*spec.Config, error)
//	DeleteConfig(id string, idType pb.IdType) error
//	GetAllConfigs() ([]*spec.Config, error)
//	SaveConfig(config *spec.Config) error
//	UpdateSchedulerTTL(name string, newTTL int32) error
//	UpdateTaskLease(name, cluster string, newLease int32) error
//	UpdateMsToNull(id string, idType pb.IdType) error
//	UpdateDs(config *spec.Config) error
//	UpdateCs(config *spec.Config) error
//	UpdateWorkflowState(configName, clusterName string, workflow *spec.Workflow) error
//	UpdateAllStates(configName string, state map[string]*spec.Workflow) error
//	PushEvents(configName string, events map[string]*spec.Events) error
//}
