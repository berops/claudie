package client

import (
	"context"

	"github.com/berops/claudie/proto/pb"
)

// SendAutoscalerEvent will send the information about
func SendAutoscalerEvent(c pb.OperatorServiceClient, req *pb.SendAutoscalerEventRequest) error {
	_, err := c.SendAutoscalerEvent(context.Background(), req)
	return err
}
