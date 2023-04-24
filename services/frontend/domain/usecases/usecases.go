package usecases

import (
	"sync"

	"github.com/berops/claudie/services/frontend/domain/ports"
)

type Usecases struct {
	ContextBox ports.ContextBoxPort

	// configsBeingDeleted is a go-routine safe map that stores id of configs that are being currently deleted
	// to avoid having multiple go-routines deleting the same configs from MongoDB (of contextBox microservice).
	configsBeingDeleted sync.Map

	// inProgress are configs that are being tracked for their current workflow state
	// to provide more friendly logs in the service.
	inProgress sync.Map

	// done indicates that the server is in shutdown.
	Done chan struct{}
}
