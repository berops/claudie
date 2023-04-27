package usecases

import (
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) DeleteConfig(request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Info().Msgf("Deleting config %s", request.Id)
	err := u.MongoDB.UpdateMsToNull(request.Id)
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("Deletion for config %s will start shortly", request.Id)
	return &pb.DeleteConfigResponse{Id: request.GetId()}, nil
}
