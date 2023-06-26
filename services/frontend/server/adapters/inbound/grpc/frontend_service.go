package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/frontend/server/domain/usecases"
)

type FrontendGrpcService struct {
	pb.UnimplementedFrontendServiceServer
	usecases *usecases.Usecases
}

func (f *FrontendGrpcService) SendAutoscalerEvent(ctx context.Context, request *pb.SendAutoscalerEventRequest) (*pb.SendAutoscalerEventResponse, error){
	return f.usecases.SendAutoscalerEvent(request)
}