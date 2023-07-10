package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/claudie-operator/server/domain/usecases"
)

type OperatorGrpcService struct {
	pb.UnimplementedOperatorServiceServer
	usecases *usecases.Usecases
}

func (f *OperatorGrpcService) SendAutoscalerEvent(ctx context.Context, request *pb.SendAutoscalerEventRequest) (*pb.SendAutoscalerEventResponse, error) {
	return f.usecases.SendAutoscalerEvent(request)
}
