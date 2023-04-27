package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/utils"
)

func (u *Usecases) SaveConfigFrontend(request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {

	// Input specs can be changed by 2 entities - by Autoscaler or by User. There is a possibility that both of them can do it
	// at the same time. Thus, we need to lock the config while one entity updates it in the database, so the other entity does
	// not overwrite it.
	u.configChangeMutex.Lock()
	defer u.configChangeMutex.Unlock()

	newConfig := request.GetConfig()
	log.Info().Msgf("Saving config %s from frontend microservice", newConfig.Name)

	newConfig.MsChecksum = utils.CalculateChecksum(newConfig.Manifest)

	// Check if config with this name already exists in MongoDB
	existingConfig, err := u.DB.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err == nil {

		// TODO: understand this portion

		if string(existingConfig.MsChecksum) != string(newConfig.MsChecksum) {

			existingConfig.Manifest = newConfig.Manifest
			existingConfig.MsChecksum = newConfig.MsChecksum

		}
		newConfig = existingConfig
	}

	err = u.DB.SaveConfig(newConfig)
	if err != nil {
		return nil, fmt.Errorf("Error while saving config %s in MongoDB: %w", newConfig.Name, err)
	}

	log.Info().Msgf("Config %s successfully save din MongoDB", newConfig.Name)

	return &pb.SaveConfigResponse{Config: newConfig}, nil
}
