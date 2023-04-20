package usecases

import (
	"sync"

	"github.com/berops/claudie/services/frontend/domain/ports"
)

type Usecases struct {
	ContextBox ports.ContextBoxPort

	// inProgress are configs that are being tracked for their current workflow state
	// to provide more friendly logs in the service.
	inProgress sync.Map

	// done indicates that the server is in shutdown.
	done chan struct{}
}
