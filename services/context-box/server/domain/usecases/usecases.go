package usecases

import (
	"sync"

	"github.com/berops/claudie/services/context-box/server/domain/ports"
)

type Usecases struct {
	MongoDB           ports.MongoDBPort
	configChangeMutex sync.Mutex
}

// // SaveWorkflowState updates the workflow for a single cluster
// func (*server) SaveWorkflowState(ctx context.Context, req *pb.SaveWorkflowStateRequest) (*pb.SaveWorkflowStateResponse, error) {
// 	if req.Workflow == nil {
// 		return &pb.SaveWorkflowStateResponse{}, nil
// 	}

// 	err := database.UpdateWorkflowState(req.ConfigName, req.ClusterName, req.Workflow)
// 	return &pb.SaveWorkflowStateResponse{}, err
// }

// // SaveConfigScheduler is a gRPC servie: the function saves config to the DB after receiving it from Scheduler
// func (*server) SaveConfigScheduler(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
// 	config := req.GetConfig()
// 	log.Info().Msgf("Saving config %s from Scheduler", config.Name)
// 	// Save new config to the DB
// 	config.DsChecksum = config.MsChecksum
// 	config.SchedulerTTL = 0
// 	err := database.UpdateDs(config)
// 	if err != nil {
// 		return nil, fmt.Errorf("error while updating dsChecksum for %s : %w", config.Name, err)
// 	}

// 	err = database.UpdateSchedulerTTL(config.Name, config.SchedulerTTL)
// 	if err != nil {
// 		return nil, fmt.Errorf("error while updating schedulerTTL for %s : %w", config.Name, err)
// 	}

// 	log.Info().Msgf("Config %s successfully saved from Scheduler", config.Name)
// 	return &pb.SaveConfigResponse{Config: config}, nil
// }

// 	// save config to DB
// 	err = database.SaveConfig(newConfig)
// 	if err != nil {
// 		return nil, fmt.Errorf("error while saving config %s in db : %w", newConfig.Name, err)
// 	}
// 	log.Info().Msgf("Config %s successfully saved from FrontEnd", newConfig.Name)
// 	return &pb.SaveConfigResponse{Config: newConfig}, nil
// }

// // SaveConfigBuilder is a gRPC service: the function saves config to the DB after receiving it from Builder
// func (*server) SaveConfigBuilder(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
// 	config := req.GetConfig()
// 	log.Info().Msgf("Saving config %s from Builder", config.Name)

// 	// Save new config to the DB, update csState as dsState
// 	config.CsChecksum = config.DsChecksum
// 	config.BuilderTTL = 0
// 	// In Builder, the desired state is also updated i.e. in terraformer (node IPs, etc) thus
// 	// we need to update it in database,
// 	// however, if deletion has been triggered, the desired state should be nil
// 	if dbConf, err := database.GetConfig(config.Id, pb.IdType_HASH); err != nil {
// 		log.Warn().Msgf("Got error while checking the desired state in the database : %v", err)
// 	} else {
// 		if dbConf.DesiredState != nil {
// 			if err := database.UpdateDs(config); err != nil {
// 				return nil, fmt.Errorf("error while updating desired state: %w", err)
// 			}
// 		}
// 	}

// 	// Update the current state so its equal to the desired state
// 	if err := database.UpdateCs(config); err != nil {
// 		return nil, fmt.Errorf("error while updating csChecksum for %s : %w", config.Name, err)
// 	}

// 	if err := database.UpdateBuilderTTL(config.Name, config.BuilderTTL); err != nil {
// 		return nil, fmt.Errorf("error while updating builderTTL for %s : %w", config.Name, err)
// 	}

// 	log.Info().Msgf("Config %s successfully saved from Builder", config.Name)
// 	return &pb.SaveConfigResponse{Config: config}, nil
// }

// // GetConfigById is a gRPC service: function returns one config from the DB based on the requested index/name
// func (*server) GetConfigFromDB(ctx context.Context, req *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
// 	log.Info().Msgf("Retrieving config %s from database", req.Id)
// 	config, err := database.GetConfig(req.Id, req.Type)
// 	if err != nil {
// 		return nil, fmt.Errorf("error while getting a config %s from database : %w", req.Id, err)
// 	}
// 	log.Info().Msgf("Config %s successfully retrieved from database", req.Id)
// 	return &pb.GetConfigFromDBResponse{Config: config}, nil
// }

// // GetConfigScheduler is a gRPC service: function returns oldest config from the queueScheduler
// func (*server) GetConfigScheduler(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
// 	configInfo := queueScheduler.Dequeue()
// 	if configInfo != nil {
// 		log.Info().Msgf("Sending config %s to Scheduler", configInfo.GetName())
// 		config, err := database.GetConfig(configInfo.GetName(), pb.IdType_NAME)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &pb.GetConfigResponse{Config: config}, nil
// 	}
// 	return &pb.GetConfigResponse{Config: nil}, nil
// }

// // GetConfigBuilder is a gRPC service: function returns oldest config from the queueBuilder
// func (*server) GetConfigBuilder(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
// 	configInfo := queueBuilder.Dequeue()
// 	if configInfo != nil {
// 		log.Info().Msgf("Sending config %s to Builder", configInfo.GetName())
// 		config, err := database.GetConfig(configInfo.GetName(), pb.IdType_NAME)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &pb.GetConfigResponse{Config: config}, nil
// 	}
// 	return &pb.GetConfigResponse{Config: nil}, nil
// }

// // GetAllConfigs is a gRPC service: function returns all configs from the DB
// func (*server) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
// 	log.Info().Msgf("Getting all configs from database")
// 	configs, err := database.GetAllConfigs()
// 	if err != nil {
// 		return nil, fmt.Errorf("error getting all configs : %w", err)
// 	}
// 	log.Info().Msgf("All configs from database retrieved successfully")
// 	return &pb.GetAllConfigsResponse{Configs: configs}, nil
// }

// // DeleteConfig sets the manifest to nil so that the iteration workflow for this
// // config destroys the previous build infrastructure.
// func (*server) DeleteConfig(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
// 	log.Info().Msgf("Deleting config %s", req.Id)
// 	err := database.UpdateMsToNull(req.Id)
// 	if err != nil {
// 		return nil, err
// 	}
// 	log.Info().Msgf("Deletion for config %s will start shortly", req.Id)
// 	return &pb.DeleteConfigResponse{Id: req.GetId()}, nil
// }

// // DeleteConfigFromDB removes the config from the request from the mongoDB database.
// func (*server) DeleteConfigFromDB(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
// 	log.Info().Msgf("Deleting config %s from database", req.Id)
// 	if err := database.DeleteConfig(req.GetId(), req.GetType()); err != nil {
// 		return nil, err
// 	}
// 	log.Info().Msgf("Config %s successfully deleted from database", req.Id)
// 	return &pb.DeleteConfigResponse{Id: req.GetId()}, nil
// }

// // UpdateNodepool updates the Nodepool struct in the database, which also initiates build. This function might return an error if the updation is
// // not allowed at this time (i.e.when config is being build).
// func (s *server) UpdateNodepool(ctx context.Context, req *pb.UpdateNodepoolRequest) (*pb.UpdateNodepoolResponse, error) {
// 	// Input specification can be changed on two places, by Autoscaler and by User, thus we need to lock it, so one does not overwrite the other.
// 	s.configChangeMutex.Lock()
// 	defer s.configChangeMutex.Unlock()
// 	log.Info().Msgf("CLIENT REQUEST: UpdateNodepoolCount for Project %s, Cluster %s Nodepool %s", req.ProjectName, req.ClusterName, req.Nodepool.Name)
// 	var config *pb.Config
// 	var err error
// 	if config, err = database.GetConfig(req.ProjectName, pb.IdType_NAME); err != nil {
// 		return nil, fmt.Errorf("the project %s was not found in the database : %w ", req.ProjectName, err)
// 	}
// 	// Check if config is currently not in any build stage or in a queue
// 	if config.BuilderTTL == 0 && config.SchedulerTTL == 0 && !queueScheduler.Contains(config) && !queueBuilder.Contains(config) {
// 		// Check if all checksums are equal, meaning config is not about to get pushed to the queue or is in the queue
// 		if checksum.Equals(config.MsChecksum, config.DsChecksum) && checksum.Equals(config.DsChecksum, config.CsChecksum) {
// 			// Find and update correct nodepool count & nodes in desired state.
// 			if err := updateNodepool(config.DesiredState, req.ClusterName, req.Nodepool.Name, req.Nodepool.Nodes, &req.Nodepool.Count); err != nil {
// 				return nil, fmt.Errorf("error while updating desired state in project %s : %w", config.Name, err)
// 			}
// 			// Find and update correct nodepool nodes in current state.
// 			// This has to be done in order
// 			if err := updateNodepool(config.CurrentState, req.ClusterName, req.Nodepool.Name, req.Nodepool.Nodes, nil); err != nil {
// 				return nil, fmt.Errorf("error while updating current state in project %s : %w", config.Name, err)
// 			}
// 			// Save new config in the database with dummy CsChecksum to initiate a build.
// 			config.CsChecksum = checksum.CalculateChecksum(utils.CreateHash(8))
// 			if err := database.SaveConfig(config); err != nil {
// 				return nil, err
// 			}
// 			return &pb.UpdateNodepoolResponse{}, nil
// 		}
// 		return nil, fmt.Errorf("the project %s is about to be in the build stage", req.ProjectName)
// 	}
// 	return nil, fmt.Errorf("the project %s is currently in the build stage", req.ProjectName)
// }
