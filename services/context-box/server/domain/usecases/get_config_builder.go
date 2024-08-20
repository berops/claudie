package usecases

//import (
//	"github.com/berops/claudie/proto/pb/spec"
//	"github.com/rs/zerolog/log"
//
//	"github.com/berops/claudie/proto/pb"
//)
//
//// GetTask returns the next available task.
//func (u *Usecases) GetTask(_ *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
//	task := u.tasksQueue.Dequeue()
//	if task == nil {
//		return &pb.GetTaskResponse{Event: nil}, nil
//	}
//
//	log.Info().Msgf("Sending task %s to Builder", task.ID())
//
//	return &pb.GetTaskResponse{Event: task.(*spec.TaskEvent)}, nil
//}
