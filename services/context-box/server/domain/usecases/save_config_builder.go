package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) SaveConfigBuilder(request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	config := request.GetConfig()
	log.Info().Msgf("Saving config %s from Builder", config.Name)

	// Save new config to the DB, update csState as dsState
	config.CsChecksum = config.DsChecksum
	config.BuilderTTL = 0
	// In Builder, the desired state is also updated i.e. in terraformer (node IPs, etc) thus
	// we need to update it in database,
	// however, if deletion has been triggered, the desired state should be nil
	if dbConf, err := u.MongoDB.GetConfig(config.Id, pb.IdType_HASH); err != nil {
		log.Warn().Msgf("Got error while checking the desired state in the database : %v", err)
	} else {
		if dbConf.DesiredState != nil {
			if err := u.MongoDB.UpdateDs(config); err != nil {
				return nil, fmt.Errorf("error while updating desired state: %w", err)
			}
		}
	}

	// Update the current state so its equal to the desired state
	if err := u.MongoDB.UpdateCs(config); err != nil {
		return nil, fmt.Errorf("error while updating csChecksum for %s : %w", config.Name, err)
	}

	if err := u.MongoDB.UpdateBuilderTTL(config.Name, config.BuilderTTL); err != nil {
		return nil, fmt.Errorf("error while updating builderTTL for %s : %w", config.Name, err)
	}

	log.Info().Msgf("Config %s successfully saved from Builder", config.Name)
	return &pb.SaveConfigResponse{Config: config}, nil
}
