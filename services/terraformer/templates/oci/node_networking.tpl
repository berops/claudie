{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $i, $nodepool := .NodePools }}
{{- $region   := $nodepool.NodePool.Region }}
{{- $specName := $nodepool.NodePool.Provider.SpecName }}
resource "oci_core_subnet" "{{ $nodepool.Name }}_{{ $region }}_{{ $specName }}_subnet" {
  provider            = oci.nodepool_{{ $region }}_{{ $specName }}
  vcn_id              = oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.id
  cidr_block          = "{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name)  }}"
  compartment_id      = var.default_compartment_id_{{ $region }}_{{ $specName }}
  display_name        = "snt-{{ $clusterHash }}-{{ $region }}-{{ $nodepool.Name }}"
  security_list_ids   = [oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.default_security_list_id]
  route_table_id      = oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.default_route_table_id
  dhcp_options_id     = oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.default_dhcp_options_id
  availability_domain = "{{ $nodepool.NodePool.Zone }}"

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}
