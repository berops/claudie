package usecases

import (
	"context"
	"sync"

	"github.com/berops/claudie/services/frontend/domain/ports"
)

type Usecases struct {
	// ContextBox is a connector used to query request from context-box.
	ContextBox ports.ContextBoxPort

	// inProgress are configs that are being tracked for their current workflow state
	// to provide more friendly logs in the service.
	inProgress sync.Map

	// SaveChannel is channel which is used to pass manifests which needs to be saved.
	SaveChannel chan *RawManifest

	// DeleteChannel is channel which is used to pass manifests which needs to be deleted.
	DeleteChannel chan *RawManifest

	// Context which when cancelled will close all channel/goroutines.
	Context context.Context
}

type RawManifest struct {
	Manifest   []byte
	SecretName string
	FileName   string
}
