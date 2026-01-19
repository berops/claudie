package service

import (
	"github.com/berops/claudie/proto/pb/spec"
)

// Rolling update compares the commit hashes between the templates of the current state, desired state and
// the latest commit of the template repository. Returns a [spec.TaskEvent] for replacing a nodepool for which
// an updated commit hashes was found, After each addition of a new nodepool the old one is remove from the cluster
// and this is done gradually as the rolling update is processed.
func ClustersRollingUpdate(current, desired *spec.Clusters) *spec.TaskEvent {
	// TODO:
	return nil
}
