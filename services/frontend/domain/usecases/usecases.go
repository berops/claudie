package usecases

import (
	"context"
	"sync"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/services/frontend/domain/ports"
)

type Usecases struct {
	// ContextBox is a connector used to query request from context-box.
	ContextBox ports.ContextBoxPort

	// inProgress are configs that are being tracked for their current workflow state
	// to provide more friendly logs in the service.
	inProgress sync.Map

	// SaveChannel is channel which is used to pass manifests which needs to be saved.
	SaveChannel chan *manifest.Manifest

	// DeleteChannel is channel which is used to pass manifests which needs to be deleted.
	DeleteChannel chan *manifest.Manifest

	// Context which when cancelled will close all channel/goroutines.
	Context context.Context
}

// RawManifest represents manifest and its metadata directly from secret.
type RawManifest struct {
	// Raw decoded manifest.
	Manifest []byte
	// Secret name for this manifest.
	SecretName string
	// File name for this manifest.
	FileName string
}
