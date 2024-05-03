{{- $clusterName := .ClusterData.ClusterName}}
{{- $clusterHash := .ClusterData.ClusterHash}}

{{- range $_, $region := .Regions}}
{{- $specName := $.Provider.SpecName }}

{{- if eq $.ClusterData.ClusterType "K8s" }}
variable "gcp_storage_disk_name_{{ $region }}_{{ $specName }}" {
  default = "storage-disk"
  type    = string
}
{{- end }}

resource "google_compute_network" "network_{{ $region }}_{{ $specName }}" {
  provider                = google.nodepool_{{ $region }}_{{ $specName }}
  name                    = "net-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
  auto_create_subnetworks = false
  description             = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}

resource "google_compute_firewall" "firewall_{{ $region }}_{{ $specName }}" {
  provider     = google.nodepool_{{ $region }}_{{ $specName }}
  name         = "fwl-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
  network      = google_compute_network.network_{{ $region }}_{{ $specName }}.self_link
  description  = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"

{{- if eq $.ClusterData.ClusterType "LB" }}
  {{- range $role := index $.Metadata "roles" }}
  allow {
      protocol = "{{ $role.Protocol }}"
      ports = ["{{ $role.Port }}"]
  }
  {{- end }}
{{- end }}

{{- if eq $.ClusterData.ClusterType "K8s" }}
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

