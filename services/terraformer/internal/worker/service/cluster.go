package service

import "github.com/rs/zerolog"

type Cluster interface {
	// Build builds the cluster.
	Build(logger zerolog.Logger) error

	// Destroy destroys the cluster.
	Destroy(logger zerolog.Logger) error

	// Id returns a cluster ID for the cluster.
	Id() string

	// whether the cluster is a kubernetes cluster, if not it is a loadbalancer.
	IsKubernetes() bool

	// UpdateCurrentState sets the current state equal to the desired state.
	UpdateCurrentState()

	// If current state exists.
	HasCurrentState() bool
}
