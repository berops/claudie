package cluster_builder

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/berops/claudie/internal/extemplates"
	"github.com/berops/claudie/internal/generics"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog"
	"github.com/zclconf/go-cty/cty"
)

// ClusterType enumerates supported clusters that can be build
// using the [ClusterBuilder]
type ClusterType string

const (
	// String identifier of a kubernetes cluster type.
	KubernetesCluster ClusterType = "K8s"

	// String identifier of a loadbalancer cluster type.
	LoadbalancerCluster ClusterType = "LB"
)

const (
	// Root directory of the external templates.
	TemplatesRootDir = "services/terraformer/templates"

	// Output directory where the external templates will
	// be generated to for each cluster.
	Output = "services/terraformer/clusters"

	// Cache directory for caching providers for Tofu.
	CacheDir = "services/terraformer/cache"
)

const (
	// Directory to which the common networking infrastructure is to be generated to.
	NetworkingGenTarget = "common-networking"

	// Directory to which individual nodepools are to be generated to.
	NodepoolsGenTarget = "nodepools"

	// providerVersionFileName is the name of the file which will contain the version of all necessary
	// providers needed for the infrastructure.
	providerVersionFileName = "provider_version.tf"
)

// DnsStateKey returns the key for the S3 storage under which the DNS state file is stored.
func DnsStateKey(clusterId string) string { return fmt.Sprintf("%s-dns", clusterId) }

// NodePoolStateKey returns the key for the S3 storage under which the nodepool state file is stored.
func NodePoolStateKey(clusterId, nodepoolName string) string {
	return fmt.Sprintf("%s-%s-%s", clusterId, NodepoolsGenTarget, nodepoolName)
}

// CommonInfraStateKey returns the key for the S3 storage under which the state file
// for the common nodepool insfrastructure is stored.
func CommonInfraStateKey(clusterId string) string {
	return fmt.Sprintf("%s-%s", clusterId, NetworkingGenTarget)
}

func tofuFormatLevel() func(any) string  { return func(a any) string { return "" } }
func tofuFormatCaller() func(any) string { return func(a any) string { return "" } }

type ProviderBlock struct {
	Source  string `cty:"source"`
	Version string `cty:"version"`
}

type RequiredProvidersBlock struct {
	Providers map[string]ProviderBlock `hcl:",remain"`
}

type TerraformBlock struct {
	RequiredProviders RequiredProvidersBlock `hcl:"required_providers,block"`
	Remain            hcl.Body               `hcl:",remain"`
}

type ProviderVersionFile struct {
	Terraform TerraformBlock `hcl:"terraform,block"`
	Remain    hcl.Body       `hcl:",remain"`
}

// parseProviderVersions parses the given provider_version.tpl file.
//
// The file is expected to be plain HCL declaring a terraform block with
// a non-empty required_providers block, where each entry pins both a source
// and a version constraint, i.e.
//
//	terraform {
//	  required_providers {
//	    hcloud = {
//	      source  = "hetznercloud/hcloud"
//	      version = "~> 1.60.0"
//	    }
//	  }
//	}
func parseProviderVersions(path string) (map[string]ProviderBlock, error) {
	file, diag := hclparse.NewParser().ParseHCLFile(path)
	if diag.HasErrors() {
		return nil, diag
	}

	var v ProviderVersionFile
	if diag := gohcl.DecodeBody(file.Body, nil, &v); diag.HasErrors() {
		return nil, diag
	}

	providers := v.Terraform.RequiredProviders.Providers
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers declared in the required_providers block of %q", path)
	}

	for name, pb := range providers {
		if pb.Source == "" || pb.Version == "" {
			return nil, fmt.Errorf("provider %q in %q must have both source and version set", name, path)
		}
		if _, err := lowerBound(pb.Version); err != nil {
			return nil, fmt.Errorf("provider %q in %q: %w", name, path, err)
		}
	}
	return providers, nil
}

// mergeProviderVersions merges the provider requirements from src into dst.
//
// If a provider is declared by both, the provider with the higher version, as
// ordered by [lowerBound], wins. This is used for infrastructure shared among
// nodepools whose templates disagree on a provider version, in which case the
// highest of the pinned versions is used.
func mergeProviderVersions(dst, src map[string]ProviderBlock, log zerolog.Logger) error {
	for name, in := range src {
		cur, ok := dst[name]
		if !ok {
			dst[name] = in
			continue
		}
		if cur.Source != in.Source {
			return fmt.Errorf("provider %q is declared with two different sources, %q and %q", name, cur.Source, in.Source)
		}
		if cur.Version == in.Version {
			continue
		}

		inLB, err := lowerBound(in.Version)
		if err != nil {
			return fmt.Errorf("provider %q: %w", name, err)
		}
		curLB, err := lowerBound(cur.Version)
		if err != nil {
			return fmt.Errorf("provider %q: %w", name, err)
		}

		chosen := cur
		switch inLB.Compare(curLB) {
		case 1:
			chosen = in
		case 0:
			// Different constraints with an equal lower bound. Nodepool groups are
			// iterated in non-deterministic order, tie-break on the constraint string
			// so that repeated merges settle on the same version.
			if in.Version > cur.Version {
				chosen = in
			}
		}
		dst[name] = chosen

		log.
			Warn().
			Msgf(
				"provider %q is pinned to both %q and %q across nodepool templates, using %q",
				name,
				cur.Version,
				in.Version,
				chosen.Version,
			)
	}
	return nil
}

// lowerBound returns the version by which provider requirements are ordered.
// Pins ("1.60.0", "~> 1.60.0", ">= 1.60") as "1.60.0", the version mentioned
// by the constraint.
//
// Constraints that bound the version only from above ("< 2.0") cannot be ordered
// and are rejected, as is any syntax not understood by tofu.
func lowerBound(constraint string) (*version.Version, error) {
	if _, err := version.NewConstraint(constraint); err != nil {
		return nil, fmt.Errorf("invalid version constraint %q: %w", constraint, err)
	}

	var out *version.Version
	for clause := range strings.SplitSeq(constraint, ",") {
		m := constraintClauseRegexp.FindStringSubmatch(clause)
		if m == nil {
			// Guarded against by the NewConstraint validation above, unless the
			// upstream constraint grammar diverges from the mimicked one.
			return nil, fmt.Errorf("malformed clause %q in version constraint %q", clause, constraint)
		}
		// "<", "<=" and "!=" clauses bound the version from above or exclude
		// a single version, they carry no lower bound.
		if op := m[1]; op == "<" || op == "<=" || op == "!=" {
			continue
		}
		v, err := version.NewVersion(m[2])
		if err != nil {
			return nil, fmt.Errorf("invalid version in constraint clause %q: %w", clause, err)
		}
		if out == nil || v.GreaterThan(out) {
			out = v
		}
	}
	if out == nil {
		return nil, fmt.Errorf("version constraint %q has no lower bound to order by", constraint)
	}
	return out, nil
}

// generateProviderVersions generates the file pinning the provider
// versions for the infrastructure shared among all of the nodepools, as
// aggregated by [mergeProviderVersions].
//
// The file is built with hclwrite directly, as the gohcl encoder ignores
// fields tagged with "remain", which is how the arbitrarily named provider
// entries of the required_providers block are decoded, and would therefore
// generate an empty block.
func generateProviderVersions(path string, usedProviders map[string]ProviderBlock) error {
	if len(usedProviders) == 0 {
		return nil
	}

	f := hclwrite.NewEmptyFile()
	tf := f.Body().AppendNewBlock("terraform", nil).Body()
	rp := tf.AppendNewBlock("required_providers", nil).Body()

	for name, p := range generics.IterateMapInOrder(usedProviders) {
		rp.SetAttributeValue(name, cty.ObjectVal(map[string]cty.Value{
			"source":  cty.StringVal(p.Source),
			"version": cty.StringVal(p.Version),
		}))
	}
	return os.WriteFile(path, hclwrite.Format(f.Bytes()), 0o644)
}

func ExplainUnknownCommit(err error, clusterId string) error {
	if err == nil || !errors.Is(err, extemplates.ErrUnknownCommit) {
		return err
	}
	return fmt.Errorf(
		"cannot destroy infrastructure of %q: %w. The templates commit pinned by the current "+
			"state no longer exists in the templates repository. Point the provider's templates "+
			"reference in the InputManifest to a reachable commit, let the cluster reconcile "+
			"(which re-pins the commit), then retry the deletion",
		clusterId, err,
	)
}
