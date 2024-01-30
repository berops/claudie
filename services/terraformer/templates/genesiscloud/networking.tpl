{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}

{{- range $i, $region := .Regions }}
resource "genesiscloud_security_group" "claudie_security_group_{{ $region }}" {
  provider = genesiscloud.nodepool_{{ $region }}
  name   = "{{ $clusterName }}-{{ $clusterHash }}-security-group"
  region = "{{ $region }}"
  rules = [
    {
      direction      = "ingress"
      protocol       = "tcp"
      port_range_min = 22
      port_range_max = 22
    },
    {
      direction      = "ingress"
      protocol       = "tcp"
      port_range_min = 51820
      port_range_max = 51820
    },
{{- if eq $.ClusterType "LB" }}
    {{- range $role := index $.Metadata "roles" }}
    {
      direction      = "ingress"
      protocol       = "{{ $role.Protocol }}"
      port_range_min = {{ $role.Port }}
      port_range_max = {{ $role.Port }}
    },
    {{- end }}
{{- end }}
{{- if eq $.ClusterType "K8s" }}
    {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
     {
       direction      = "ingress"
       protocol       = "tcp"
       port_range_min = 6443
       port_range_max = 6443
     },
    {{- end }}
{{- end }}
    {
      direction      = "ingress"
      protocol       = "icmp"
    }
  ]
}
{{- end }}