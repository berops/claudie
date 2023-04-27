package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) SaveConfigScheduler(request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	config := request.GetConfig()
	log.Info().Msgf("Saving config %s from Scheduler", config.Name)
	// Save new config to the DB
	config.DsChecksum = config.MsChecksum
	config.SchedulerTTL = 0
	err := u.DB.UpdateDs(config)
	if err != nil {
		return nil, fmt.Errorf("error while updating dsChecksum for %s : %w", config.Name, err)
	}

	err = u.DB.UpdateSchedulerTTL(config.Name, config.SchedulerTTL)
	if err != nil {
		return nil, fmt.Errorf("error while updating schedulerTTL for %s : %w", config.Name, err)
	}

	log.Info().Msgf("Config %s successfully saved from Scheduler", config.Name)
	return &pb.SaveConfigResponse{Config: config}, nil
}
