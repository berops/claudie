package managerclient

import "context"

type ManifestAPI interface {
	// UpsertManifest will update the [store.Manifest] and [store.KubernetesContext] of an existing
	// config or will create a new config (if not present) from the passed in values.
	// The function will return the ErrVersionMismatch error indicating a Dirty write,
	// the application code should execute the Read/Update/Write cycle again, to resolve the merge conflicts.
	UpsertManifest(ctx context.Context, request *UpsertManifestRequest) error

	// MarkForDeletion will mark the Infrastructure of the specified Config to be deleted.
	// If the requested config with the specific version is not found the ErrVersionMismatch error is
	// returned indicating a Dirty write. On a Dirty write the application code should execute
	// the Read/Update/Write cycle again. If the config is not present an error will be returned
	// along with other errors.
	MarkForDeletion(ctx context.Context, request *MarkForDeletionRequest) error
}

type UpsertManifestRequest struct {
	Name     string
	Manifest *Manifest
	K8sCtx   *KubernetesContext
}

type Manifest struct{ Raw string }
type KubernetesContext struct{ Name, Namespace string }

type MarkForDeletionRequest struct{ Name string }
