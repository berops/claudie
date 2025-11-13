package service

import (
	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/proto/pb/spec"
	"google.golang.org/protobuf/proto"
)

// templatesUpdated checks if at least 1 used provider had their template repository updated.
func templatesUpdated(c *spec.ConfigV2) (bool, error) {
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

func commitHashUpdated(p *spec.Provider) (bool, error) {
	t := proto.Clone(p.Templates).(*spec.TemplateRepository)
	if err := manifest.FetchCommitHash(t); err != nil {
		return false, err
	}

	return p.Templates.CommitHash != t.CommitHash, nil
}
