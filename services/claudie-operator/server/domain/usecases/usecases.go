package usecases

import (
	"context"

	managerclient "github.com/berops/claudie/services/manager/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type Usecases struct {
	// Manager is a connector used to query request from manager.
	Manager managerclient.ClientAPI

	// Context which when cancelled will close all channel/goroutines.
	Context context.Context

	// SaveAutoscalerEvent is channel which is used to pass autoscaler event to controller
	SaveAutoscalerEvent chan event.GenericEvent
}
