{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $region := .Regions }}
{{- $specName := $.Provider.SpecName }}

data "genesiscloud_images" "base_os_{{ $region }}_{{ $specName }}" {
  provider   = genesiscloud.nodepool_{{ $region }}_{{ $specName }}
  filter = {
    type   = "base-os"
    region = "{{ $region }}"
  }
}

resource "genesiscloud_security_group" "claudie_security_group_{{ $region }}_{{ $specName }}" {
  provider = genesiscloud.nodepool_{{ $region }}_{{ $specName }}
  name   = "sg-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
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
{{- if eq $.ClusterData.ClusterType "LB" }}
    {{- range $role := index $.Metadata "roles" }}
    {
      direction      = "ingress"
      protocol       = "{{ $role.Protocol }}"
      port_range_min = {{ $role.Port }}
      port_range_max = {{ $role.Port }}
    },
    {{- end }}
{{- end }}
{{- if eq $.ClusterData.ClusterType "K8s" }}
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