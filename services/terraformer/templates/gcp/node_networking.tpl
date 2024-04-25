{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $nodepool := .NodePools }}
{{- $region   := $nodepool.NodePool.Region }}
{{- $specName := $nodepool.NodePool.Provider.SpecName }}
resource "google_compute_subnetwork" "{{ $nodepool.Name }}_{{ $clusterName}}_{{ $clusterHash }}_{{ $region }}_{{ $specName }}_subnet" {
  provider      = google.nodepool_{{ $region }}_{{ $specName }}
  name          = "snt-{{ $clusterHash }}-{{ $region }}-{{ $nodepool.Name }}"
  network       = google_compute_network.network_{{ $region }}_{{ $specName }}.self_link
  ip_cidr_range = "{{index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  description   = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}
{{- end }}