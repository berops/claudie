package extemplates

import (
	"errors"
)

var (
	// ErrEmptyRepository is returned when no repository is to be cloned.
	ErrEmptyRepository = errors.New("no repository to clone")

	// ErrUnsupportedProtocol is returned when the protocol for the templates is not supported.
	ErrUnsupportedProtocol = errors.New("unsupported protocol for downloading templates")

	// ErrUnknownCommit is returned when checkout to the specified commit fails.
	ErrUnknownCommit = errors.New("failed to checkout to specified commit")
)
