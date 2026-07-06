package extofu

import (
	"encoding/hex"
	"fmt"
	"iter"
	"path/filepath"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/tmplutils"
	"github.com/berops/claudie/proto/pb/spec"
)

// Returns the key to be used for reading the output of the templates used within the terraformer service.
func NodePoolTerraformKey(np *spec.NodePool) string {
	d := np.GetDynamicNodePool()
	f := hash.Digest128(filepath.Join(d.GetProvider().GetSpecName(), TemplatesPath(d.GetProvider())))
	return fmt.Sprintf("%s_%s_%s", np.Name, d.GetProvider().GetSpecName(), hex.EncodeToString(f))
}

// Returns the key to be used for reading the output of the templates used within the terraformer service.
func DnsEndpointTerraformKey(dns *spec.DNS, clusterID, alternativeName string) string {
	f := hash.Digest128(filepath.Join(dns.GetProvider().GetSpecName(), TemplatesPath(dns.GetProvider())))
	resourceSuffix := fmt.Sprintf("%s_%s", dns.GetProvider().GetSpecName(), hex.EncodeToString(f))
	resource := clusterID
	if alternativeName != "" {
		resource += "_" + tmplutils.SanitizeStringForResourceName(alternativeName)
	}
	return fmt.Sprintf("%s_%s", resource, resourceSuffix)
}

// Returns a deterministic unique string for each distinct provider.
func Fingerprint(p *spec.Provider) string {
	return hex.EncodeToString(hash.Digest128(filepath.Join(p.SpecName, TemplatesPath(p))))
}

// Returns the path under which the terraformer templates for the provider should be extracted to.
//
// If the URL of the [spec.TemplateRepository] of the [spec.Provider] is invalid a default one will
// be used instead.
func TemplatesPath(p *spec.Provider) string {
	if p == nil || p.Templates == nil {
		return ""
	}

	return p.Templates.TemplatesPath(p.Templates.Paths.Terraformer, p.CloudProviderName)
}

// ByTemplates returns an iterator that groups nodepools by provider terraform templates path.
func NodePoolsByTemplatesPath(nps []*spec.NodePool) iter.Seq2[string, []*spec.NodePool] {
	m := make(map[string][]*spec.NodePool)

	for _, nodepool := range nps {
		np, ok := nodepool.Type.(*spec.NodePool_DynamicNodePool)
		if !ok {
			continue
		}

		p := TemplatesPath(np.DynamicNodePool.Provider)
		m[p] = append(m[p], nodepool)
	}

	return func(yield func(string, []*spec.NodePool) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	}
}
