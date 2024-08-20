package grpc

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/domain/usecases"
)

type ContextBoxGrpcService struct {
	pb.UnimplementedContextBoxServiceServer

	usecases *usecases.Usecases
}

//func (c *ContextBoxGrpcService) SaveConfigOperator(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
//	return c.usecases.SaveConfigOperator(request)
//}
//
//func (c *ContextBoxGrpcService) SaveWorkflowState(ctx context.Context, request *pb.SaveWorkflowStateRequest) (*pb.SaveWorkflowStateResponse, error) {
//	return c.usecases.SaveWorkflowState(request)
//}
//
//func (c *ContextBoxGrpcService) SaveConfigScheduler(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
//	return c.usecases.SaveConfigScheduler(request)
//}
//
//func (c *ContextBoxGrpcService) SaveConfigBuilder(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
//	return c.usecases.SaveConfigBuilder(request)
//}
//
//func (c *ContextBoxGrpcService) GetConfigFromDB(ctx context.Context, request *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
//	return c.usecases.GetConfigFromDB(request)
//}
//
//func (c *ContextBoxGrpcService) GetConfigScheduler(ctx context.Context, request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
//	return c.usecases.GetConfigScheduler(request)
//}
//
//func (c *ContextBoxGrpcService) GetTask(ctx context.Context, request *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
//	return c.usecases.GetTask(request)
//}
//
//func (c *ContextBoxGrpcService) GetAllConfigs(ctx context.Context, request *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
//	return c.usecases.GetAllConfigs(request)
//}
//
//func (c *ContextBoxGrpcService) DeleteConfig(ctx context.Context, request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
//	return c.usecases.DeleteConfig(request)
//}
//
//func (c *ContextBoxGrpcService) DeleteConfigFromDB(ctx context.Context, request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
//	return c.usecases.DeleteConfigFromDB(request)
//}
//
//func (c *ContextBoxGrpcService) UpdateNodepool(ctx context.Context, request *pb.UpdateNodepoolRequest) (*pb.UpdateNodepoolResponse, error) {
//	return c.usecases.UpdateNodepool(request)
//}
