package managerclient

import (
	"errors"
	"io"

	"github.com/berops/claudie/internal/healthcheck"
)

var (
	// ErrVersionMismatch is returned when the requested operation errors out due to a mismatch in the document version.
	// Two writes using the same document version occurred but this writes failed as the document was modified by the other write.
	ErrVersionMismatch = errors.New("requested operation failed due to document version mismatch")

	// ErrNotFound is returned when the requested resource, i.e. Config, cluster, task etc. is not found.
	ErrNotFound = errors.New("not found")
)

// ClientAPI wraps all manager apis into a single interface.
type ClientAPI interface {
	io.Closer
	healthcheck.HealthChecker

	TaskAPI
	ManifestAPI
	CrudAPI
	StateAPI
}
