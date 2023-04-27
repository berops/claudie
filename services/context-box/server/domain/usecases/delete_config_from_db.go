package usecases

import (
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

// DeleteConfigFromDB removes the config from the request from the mongoDB c.usecases.MongoDB.
func (u *Usecases) DeleteConfigFromDB(request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Info().Msgf("Deleting config %s from database", request.Id)
	if err := u.DB.DeleteConfig(request.GetId(), request.GetType()); err != nil {
		return nil, err
	}
	log.Info().Msgf("Config %s successfully deleted from database", request.Id)
	return &pb.DeleteConfigResponse{Id: request.GetId()}, nil
}
