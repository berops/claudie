{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}

{{- range $i, $region := .Regions}}
resource "google_compute_network" "network_{{ $clusterName}}_{{ $clusterHash}}_{{ $region }}" {
  provider                = google.nodepool_{{ $region }}
  name                    = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-network"
  auto_create_subnetworks = false
  description             = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}

resource "google_compute_firewall" "firewall_{{ $region }}" {
  provider     = google.nodepool_{{ $region }}
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-firewall"
  network      = google_compute_network.network_{{ $clusterName}}_{{ $clusterHash}}_{{ $region }}.self_link
  description  = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"

{{- if eq $.ClusterType "LB" }}
  {{- range $role := index $.Metadata "roles" }}
  allow {
      protocol = "{{ $role.Protocol }}"
      ports = ["{{ $role.Port }}"]
  }
  {{- end }}
{{- end }}

{{- if eq $.ClusterType "K8s" }}
  {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
  allow {
      protocol = "TCP"
      ports    = ["6443"]
  }
  {{- end }}
{{- end }}

  allow {
    protocol = "UDP"
    ports    = ["51820"]
  }

  allow {
      protocol = "TCP"
      ports    = ["22"]
  }

  allow {
      protocol = "icmp"
   }

  source_ranges = [
      "0.0.0.0/0",
   ]
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "google_compute_subnetwork" "{{ $nodepool.Name }}_subnet" {
  provider      = google.nodepool_{{ $nodepool.NodePool.Region }}
  name          = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
  network       = google_compute_network.network_{{ $clusterName}}_{{ $clusterHash}}_{{ $nodepool.NodePool.Region }}.self_link
  ip_cidr_range = "{{index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  description   = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}
{{- end }}
