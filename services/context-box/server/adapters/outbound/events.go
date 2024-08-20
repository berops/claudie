package outboundAdapters

//
//import (
//	"time"
//
//	"github.com/berops/claudie/proto/pb/spec"
//
//	"google.golang.org/protobuf/proto"
//	"google.golang.org/protobuf/types/known/timestamppb"
//)
//
//type Events struct {
//	TaskEvents []TaskEvent `bson:"taskEvents"`
//	TTL        int32       `bson:"ttl"`
//}
//
//type TaskEvent struct {
//	Id         string `bson:"id"`
//	ConfigName string `bson:"configName"`
//	Timestamp  string `bson:"timestamp"`
//	Event      string `bson:"event"`
//	Task       []byte `bson:"task"`
//}
//
//func (te TaskEvent) ID() string { return te.Id }
//
//// ConvertFromGRPCEvents converts the events data from GRPC to the database representation.
//func ConvertFromGRPCEvents(w map[string]*spec.Events) (map[string]Events, error) {
//	result := make(map[string]Events)
//
//	for k, v := range w {
//		var te []TaskEvent
//		for _, e := range v.Events {
//			b, err := proto.Marshal(e.Task)
//			if err != nil {
//				return nil, err
//			}
//
//			te = append(te, TaskEvent{
//				Id:         e.Id,
//				ConfigName: e.ConfigName,
//				Timestamp:  e.Timestamp.AsTime().Format(time.RFC3339),
//				Event:      e.Event.String(),
//				Task:       b,
//			})
//		}
//
//		result[k] = Events{
//			TaskEvents: te,
//			TTL:        v.Ttl,
//		}
//	}
//
//	return result, nil
//}
//
//// ConvertToGRPCEvents converts the database representation of events to GRPC.
//func ConvertToGRPCEvents(w map[string]Events) (map[string]*spec.Events, error) {
//	result := make(map[string]*spec.Events)
//
//	for k, v := range w {
//		var te []*spec.TaskEvent
//		for _, e := range v.TaskEvents {
//			var task spec.Task
//			if err := proto.Unmarshal(e.Task, &task); err != nil {
//				return nil, err
//			}
//
//			t, err := time.Parse(time.RFC3339, e.Timestamp)
//			if err != nil {
//				return nil, err
//			}
//
//			te = append(te, &spec.TaskEvent{
//				Id:         e.Id,
//				ConfigName: e.ConfigName,
//				Timestamp:  timestamppb.New(t),
//				Event:      spec.Event(spec.Event_value[e.Event]),
//				Task:       &task,
//			})
//		}
//		result[k] = &spec.Events{
//			Events: te,
//			Ttl:    v.TTL,
//		}
//	}
//
//	return result, nil
//}
