package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) GetConfigFromDB(request *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
	log.Info().Msgf("Retrieving config %s from database", request.Id)
	config, err := u.DB.GetConfig(request.Id, request.Type)
	if err != nil {
		return nil, fmt.Errorf("error while getting a config %s from database : %w", request.Id, err)
	}
	log.Info().Msgf("Config %s successfully retrieved from database", request.Id)
	return &pb.GetConfigFromDBResponse{Config: config}, nil
}
