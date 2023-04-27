package usecases

import (
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) GetConfigScheduler(request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	configInfo := u.schedulerQueue.Dequeue()
	if configInfo != nil {
		log.Info().Msgf("Sending config %s to Scheduler", configInfo.GetName())
		config, err := u.DB.GetConfig(configInfo.GetName(), pb.IdType_NAME)
		if err != nil {
			return nil, err
		}
		return &pb.GetConfigResponse{Config: config}, nil
	}
	return &pb.GetConfigResponse{Config: nil}, nil
}
