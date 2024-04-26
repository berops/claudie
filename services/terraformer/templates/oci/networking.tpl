{{- $clusterName := .ClusterData.ClusterName}}
{{- $clusterHash := .ClusterData.ClusterHash}}

{{- range $_, $region := .Regions }}
{{- $specName := $.Provider.SpecName }}

{{- if eq $.ClusterData.ClusterType "K8s" }}
variable "oci_storage_disk_name_{{ $region }}_{{ $specName }}" {
  default = "oraclevdb"
  type    = string
}
{{- end }}

variable "default_compartment_id_{{ $region }}_{{ $specName }}" {
  type    = string
  default = "{{ $.Provider.OciCompartmentOcid }}"
}

resource "oci_core_vcn" "claudie_vcn_{{ $region }}_{{ $specName }}" {
  provider        = oci.nodepool_{{ $region }}_{{ $specName }}
  compartment_id  = var.default_compartment_id_{{ $region }}_{{ $specName }}
  display_name    = "vcn-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
  cidr_blocks     = ["10.0.0.0/16"]

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "oci_core_internet_gateway" "claudie_gateway_{{ $region }}_{{ $specName }}" {
  provider        = oci.nodepool_{{ $region }}_{{ $specName }}
  compartment_id  = var.default_compartment_id_{{ $region }}_{{ $specName }}
  display_name    = "gtw-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
  vcn_id          = oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.id
  enabled         = true

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "oci_core_default_security_list" "claudie_security_rules_{{ $region }}_{{ $specName }}" {
  provider                    = oci.nodepool_{{ $region }}_{{ $specName }}
  manage_default_resource_id  = oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.default_security_list_id
  display_name                = "sl-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
    description = "Allow all egress"
  }

  ingress_security_rules {
    protocol    = "1"
    source      = "0.0.0.0/0"
    description = "Allow all ICMP"
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    tcp_options {
      min = "22"
      max = "22"
    }
    description = "Allow SSH connections"
  }

{{- if eq $.ClusterData.ClusterType "K8s" }}
  {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    tcp_options {
      max = "6443"
      min = "6443"
    }
    description = "Allow kube API port"
  }
  {{- end }}
{{- end }}

{{- if eq $.ClusterData.ClusterType "LB" }}
  {{- range $role := index $.Metadata "roles"}}
  ingress_security_rules {
    protocol  = "{{ protocolToOCIProtocolNumber $role.Protocol}}"
    source    = "0.0.0.0/0"
    tcp_options {
      max = "{{ $role.Port }}"
      min = "{{ $role.Port }}"
    }
    description = "LoadBalancer port defined in the manifest"
  }
  {{- end }}
{{- end }}

  ingress_security_rules {
    protocol    = "17"
    source      = "0.0.0.0/0"
    udp_options {
      max = "51820"
      min = "51820"
    }
    description = "Allow Wireguard VPN port"
  }

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "oci_core_default_route_table" "claudie_routes_{{ $region }}_{{ $specName }}" {
  provider                    = oci.nodepool_{{ $region }}_{{ $specName }}
  manage_default_resource_id  = oci_core_vcn.claudie_vcn_{{ $region }}_{{ $specName }}.default_route_table_id

  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.claudie_gateway_{{ $region }}_{{ $specName }}.id
    destination_type  = "CIDR_BLOCK"
  }

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

