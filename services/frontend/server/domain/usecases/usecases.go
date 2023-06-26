package usecases

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/berops/claudie/services/frontend/server/domain/ports"
)

type Usecases struct {
	// ContextBox is a connector used to query request from context-box.
	ContextBox ports.ContextBoxPort

	// Context which when cancelled will close all channel/goroutines.
	Context context.Context

	// SaveAutoscalerEvent is channel which is used to pass autoscaler event to controller
	SaveAutoscalerEvent chan event.GenericEvent
}
