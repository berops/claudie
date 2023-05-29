package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/utils"
)

// SaveConfigFrontend saves config to MongoDB after receiving it from the frontend microservice
func (u *Usecases) SaveConfigFrontEnd(request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	// Input specs can be changed by 2 entities - by Autoscaler or by User. There is a possibility that both of them can do it
	// at the same time. Thus, we need to lock the config while one entity updates it in the database, so the other entity does
	// not overwrite it.
	u.configChangeMutex.Lock()
	defer u.configChangeMutex.Unlock()

	newConfig := request.GetConfig()
	log.Info().Msgf("Saving config %s from Frontend", newConfig.Name)

	newConfig.MsChecksum = utils.CalculateChecksum(newConfig.Manifest)

	// Check if config with this name already exists in MongoDB
	oldConfig, err := u.DB.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err == nil {
		if string(oldConfig.MsChecksum) != string(newConfig.MsChecksum) {
			oldConfig.Manifest = newConfig.Manifest
			oldConfig.MsChecksum = newConfig.MsChecksum
			oldConfig.SchedulerTTL = 0
			oldConfig.BuilderTTL = 0
		}
		newConfig = oldConfig
	}

	if err = u.DB.SaveConfig(newConfig); err != nil {
		return nil, fmt.Errorf("error while saving config %s in MongoDB: %w", newConfig.Name, err)
	}

	log.Info().Msgf("Config %s successfully saved from Frontend", newConfig.Name)

	return &pb.SaveConfigResponse{Config: newConfig}, nil
}
