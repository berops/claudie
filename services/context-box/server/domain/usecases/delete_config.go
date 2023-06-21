package usecases

import (
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

// DeleteConfig sets the manifest to nil so that the iteration workflow for this
// config destroys the previous build infrastructure.
func (u *Usecases) DeleteConfig(request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Info().Msgf("Deleting config %s", request.Id)

	err := u.DB.UpdateMsToNull(request.Id, request.Type)
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("Deletion for config %s will start shortly", request.Id)
	return &pb.DeleteConfigResponse{Id: request.GetId()}, nil
}
