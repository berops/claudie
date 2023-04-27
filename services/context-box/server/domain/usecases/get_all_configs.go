package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) GetAllConfigs(request *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	log.Info().Msgf("Getting all configs from database")
	configs, err := u.DB.GetAllConfigs()
	if err != nil {
		return nil, fmt.Errorf("error getting all configs : %w", err)
	}
	log.Info().Msgf("All configs from database retrieved successfully")
	return &pb.GetAllConfigsResponse{Configs: configs}, nil
}
