package managerclient

import (
	"errors"
	"io"

	"github.com/berops/claudie/internal/healthcheck"
)

// ErrVersionMismatch is returned when the requested operation errors out due to a mismatch in the document version.
// Two writes using the same document version occurred but this writes failed as the document was modified by the other write.
var ErrVersionMismatch = errors.New("requested operation failed due to document version mismatch. Manual merging of the two state is required by the client code")

// ErrNotFound is returned when the requested resource (i.e a Config, or a cluster within a config etc...) is not found.
var ErrNotFound = errors.New("not found")

// ClientAPI wraps all manager apis into a single interface.
type ClientAPI interface {
	io.Closer
	healthcheck.HealthChecker

	TaskAPI
	ManifestAPI
	CrudAPI
	StateAPI
}
