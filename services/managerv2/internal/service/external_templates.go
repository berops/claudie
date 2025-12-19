package service

import (
	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/proto/pb/spec"
	"google.golang.org/protobuf/proto"
)

// templatesUpdated checks if at least 1 used provider had their template repository updated.
func templatesUpdated(c *spec.Config) (bool, error) {
	for _, cluster := range c.Clusters {
		for _, n := range cluster.Current.GetK8S().GetClusterInfo().GetNodePools() {
			n := n.GetDynamicNodePool()
			if n == nil || n.Provider.Templates.Tag != nil {
				continue
			}

			updated, err := commitHashUpdated(n.Provider)
			if err != nil {
				return false, err
			}

			if updated {
				return true, nil
			}
		}

		for _, lb := range cluster.Current.GetLoadBalancers().GetClusters() {
			for _, n := range lb.ClusterInfo.NodePools {
				n := n.GetDynamicNodePool()
				if n == nil || n.Provider.Templates.Tag != nil {
					continue
				}

				updated, err := commitHashUpdated(n.Provider)
				if err != nil {
					return false, err
				}

				if updated {
					return true, nil
				}
			}

			updated, err := commitHashUpdated(lb.Dns.Provider)
			if err != nil {
				return false, err
			}

			if updated {
				return true, nil
			}
		}
	}

	return false, nil
}

// Checks whether the templates for the provider has been updated.
func commitHashUpdated(p *spec.Provider) (bool, error) {
	t := proto.Clone(p.Templates).(*spec.TemplateRepository)
	if err := manifest.FetchCommitHash(t); err != nil {
		return false, err
	}

	return p.Templates.CommitHash != t.CommitHash, nil
}

// Fetches the latest commit hash for the NodePools which have a provider with the lates Tag option.
// If the nodepool does not have such a privder false is returned, otherwise true to signal the commit
// hash has ben fetched. On any error during the network calls the error is propagated to the caller.
func syncWithRemoteRepo(np *spec.NodePool) (bool, error) {
	n := np.GetDynamicNodePool()
	if n == nil || n.Provider.Templates.Tag != nil {
		return false, nil
	}

	if err := manifest.FetchCommitHash(n.Provider.Templates); err != nil {
		return false, err
	}

	return true, nil
}
