package usecases

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// SendAutoscalerEvent will recive an autoscaler event, and send it to the autoscaler channel
func (u *Usecases) SendAutoscalerEvent(request *pb.SendAutoscalerEventRequest) (*pb.SendAutoscalerEventResponse, error) {
	im := v1beta1.InputManifest{}
	im.SetName(request.InputManifestName)
	im.SetNamespace(request.InputManifestNamespace)
	u.SaveAutoscalerEvent <- event.GenericEvent{Object: &im}
	return &pb.SendAutoscalerEventResponse{}, nil
}
