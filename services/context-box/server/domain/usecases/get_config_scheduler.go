package usecases

//import (
//	"github.com/berops/claudie/proto/pb/spec"
//	"github.com/rs/zerolog/log"
//
//	"github.com/berops/claudie/proto/pb"
//)
//
//// GetConfigScheduler is a gRPC service: function returns oldest config from the queueScheduler
//func (u *Usecases) GetConfigScheduler(_ *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
//	nextConfig := u.schedulerQueue.Dequeue()
//	if nextConfig == nil {
//		return &pb.GetConfigResponse{Config: nil}, nil
//	}
//
//	cfg := (nextConfig).(*spec.Config)
//
//	log.Info().Msgf("Sending config %s to Scheduler", cfg.GetName())
//
//	return &pb.GetConfigResponse{Config: cfg}, nil
//}
