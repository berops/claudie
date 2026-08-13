// Package extofu (external tofu templates) provides exported types and necessary
// helper methods to work with external terraform files which can be downloaded from
// any public or private git repository.
//
// # What you are writing
//
// Templates are Go-templated terraform files (.tpl) stored in a git repository,
// grouped per provider under a path you reference from the InputManifest, e.g.:
//
//	claudie-config/templates/terraformer/gcp
//
// Claudie downloads that subtree (sparse checkout, pinned to a commit), renders
// the .tpl files with the data described below, and runs `tofu apply` on the
// result. You are not writing one terraform project — you are writing up to
// three independent ones. Each is applied separately, with its own state file:
//
//	networking/  ONE project per cluster. The rendered networking templates of
//	             EVERY provider used by the cluster land together in one folder.
//	             Holds everything nodepools share: VPCs, subnets, firewalls.
//	nodepool/    ONE project PER NODEPOOL. Holds everything owned by a single
//	             nodepool: VMs, disks, IPs.
//	dns/         ONE project per loadbalancer cluster. Holds the DNS records.
//
// Because these are separate projects with separate state, a nodepool template
// can never reference a networking resource by its terraform address
// (google_compute_subnetwork.my_subnet.id will not resolve — the resource lives
// in a different state). The ONLY way to pass data from networking to nodepools
// is: networking exports an output, claudie reads it with `tofu output` after
// applying, and hands the values to your nodepool templates.
//
// # What claudie generates for you — and what that forbids
//
// Into every rendered project folder claudie places:
//
//   - backend.tf — where the state lives. Never declare your own backend.
//   - provider_version.tf — the required_providers block, generated from your
//     provider_version.tpl (see below). Never declare required_providers yourself.
//   - a credentials file named after the provider's SpecName, containing the raw
//     credentials from the InputManifest. Reference it from your provider block,
//     e.g. credentials = file("./{{ .Data.Provider.SpecName }}").
//   - (nodepool projects only) the nodepool's public SSH key, in a file named
//     after the nodepool: file("./{{ .Data.NodePool.Name }}").
//
// What claudie does NOT generate are the provider configuration blocks
// themselves. Each of the networking, nodepool and dns subdirectories MUST
// contain a file named "provider.tpl" declaring exactly the provider
// configuration blocks the other files of that directory use — and nothing
// else: no resources, no outputs, no terraform blocks. No other file may
// declare provider blocks. Claudie renders provider.tpl separately from the
// rest of its directory, so provider blocks hidden in other files either go
// missing or render twice as duplicate configurations.
//
// # provider_version.tpl (required)
//
// Next to the networking/nodepool/dns subdirectories, the subtree root must
// contain a file named "provider_version.tpl". Despite the extension it is NOT
// a template — it is plain HCL, and must pin source and version for every
// provider your templates use:
//
//	terraform {
//	  required_providers {
//	    google = {
//	      source  = "hashicorp/google"
//	      version = "~> 6.0"
//	    }
//	  }
//	}
//
// Every version constraint must have a lower bound ("~> 6.0", ">= 6.1", "6.1.2"
// are fine, "< 7.0" alone is rejected). When a cluster mixes nodepools from
// several template repositories, all their pins are merged into the shared
// networking project: the same provider name must use the same source
// everywhere, and the highest lower bound wins.
//
// # Writing networking templates
//
// Each file receives the [Networking] struct as .Data: the provider
// (.Data.Provider), the regions the cluster's nodepools of this provider use
// (.Data.Regions, .Data.RegionNetwork), cluster identity (.Data.ClusterData),
// and, depending on the cluster type, .Data.K8sData.HasAPIServer or
// .Data.LBData.Roles for firewall rules.
//
// Remember that your rendered files share one folder and one state with the
// networking templates of every other provider in the cluster. Make every
// resource and output name unique by embedding .Fingerprint — a string claudie
// passes to each template that is unique per (provider, template repository):
//
//	resource "google_compute_network" "net_{{ .Fingerprint }}" { ... }
//
//	output "subnet_{{ $region }}_{{ .Fingerprint }}" {
//	  value = google_compute_subnetwork.subnet_{{ $region }}_{{ .Fingerprint }}.self_link
//	}
//
// Export through outputs everything your nodepool templates will need — subnet
// ids, security group ids. Nothing else crosses the boundary.
//
// # Writing nodepool templates
//
// Each file receives the [Nodepool] struct as .Data, describing exactly ONE
// nodepool: .Data.NodePool (Name, Details, Nodes, IsControl — node names are
// known at render time, their IPs are not, the CIDR in Details is already
// assigned) and .Data.Networking.All — a map holding every output your
// networking templates exported, keyed by output name:
//
//	subnetwork = {{ index .Data.Networking.All (printf "subnet_%s_%s" $region .Fingerprint) }}
//
// Every nodepool project MUST export one output that tells claudie the public
// address of each node, named exactly "<nodepoolName>_<specName>_<fingerprint>"
// (see [NodePoolTerraformKey]):
//
//	output "{{ .Data.NodePool.Name }}_{{ .Data.NodePool.Details.Provider.SpecName }}_{{ .Fingerprint }}" {
//	  value = {
//	    {{- range $node := .Data.NodePool.Nodes }}
//	    "{{ $node.Name }}" = google_compute_instance.{{ $node.Name }}_{{ $.Fingerprint }}.network_interface.0.access_config.0.nat_ip
//	    {{- end }}
//	  }
//	}
//
// The value per node is either the IP, or [IP, sshPort], or
// [IP, sshPort, wireguardPort] for shared-IP/NAT setups where nodes are reached
// on mapped host ports. A node missing from this output is treated as a failed
// build.
//
// # Writing dns templates
//
// Each file receives the [DNS] struct as .Data (hostname, zone, record IPs,
// provider). The project must export the created endpoint under the output
// "<clusterId>_<specName>_<fingerprint>" (see [DnsEndpointTerraformKey]) whose
// value maps "<clusterId>-endpoint" to the fully qualified domain name.
// Optional extensions (alternative names, provider extras) are documented on
// [AlternativeNamesExtension] and [ProviderExtrasExtension]; probe for them
// with the hasExtension template function.
//
// # Rules your templates must follow
//
// Claudie re-renders and re-applies these projects independently, repeatedly,
// and with a changing set of nodepools. Applies must converge, so:
//
//   - Be deterministic. The same input data must always render the same
//     resources, names and outputs. No randomness, no timestamps.
//
//   - Derive networking resources only from the input data. When a nodepool is
//     added or removed, your networking templates are re-rendered with the
//     grown or shrunk region list — that render must create or remove exactly
//     the resources of the affected regions and leave the resources and output
//     values of every remaining region untouched.
//
//   - Treat outputs as a stable API. Nodepool projects bake the output VALUES
//     in at render time. When claudie re-applies the networking project it
//     re-renders and re-applies every nodepool against the fresh outputs — an
//     output value that changes gratuitously therefore churns every nodepool.
//
//   - Keep role/API-port differences inside firewall rules. When only
//     .Data.LBData.Roles or .Data.K8sData.HasAPIServer change, claudie applies
//     the networking project WITHOUT re-applying the nodepools. Rendering
//     differences driven by these fields must be confined to firewall/security
//     rules and must not change any output value.
//
//   - Derive provider aliases purely from .Data.Regions and
//     .Data.RegionNetwork. Claudie may render networking/provider.tpl with a
//     WIDER region set than the resource files: when a nodepool is deleted,
//     its regions stay in the provider.tpl render until the shared
//     infrastructure they anchor is destroyed — the destroy still needs those
//     provider aliases, even though no resource file references them anymore.
//     provider.tpl must therefore produce valid configuration for any region
//     set it is given.
//
// # Example Layout of a subtree
//
//	templates/terraformer/gcp
//	├── provider_version.tpl
//	├── networking
//	│   └── networking.tpl
//	│   └── provider.tpl
//	├── nodepool
//	│   ├── node.tpl
//	│   └── node_networking.tpl
//	│   └── provider.tpl
//	└── dns
//	    └── dns.tpl
//	    └── provider.tpl
//
// Working examples for every supported provider: https://github.com/berops/claudie-config
package extofu
