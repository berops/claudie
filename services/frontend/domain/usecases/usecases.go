package usecases

import (
	"context"

	"github.com/berops/claudie/services/frontend/domain/ports"
)

type Usecases struct {
	// ContextBox is a connector used to query request from context-box.
	ContextBox ports.ContextBoxPort

	// Context which when cancelled will close all channel/goroutines.
	Context context.Context
}
