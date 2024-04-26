{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $region := .Regions }}
{{- $sanitisedRegion := replaceAll $region " " "_"}}
{{- $specName := $.Provider.SpecName }}

resource "azurerm_resource_group" "rg_{{ $sanitisedRegion }}_{{ $specName }}" {
  provider = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  name     = "rg-{{ $clusterHash }}-{{ $sanitisedRegion }}-{{ $specName }}"
  location = "{{ $region }}"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_virtual_network" "claudie_vn_{{ $sanitisedRegion }}_{{ $specName }}" {
  provider            = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  name                = "vn-{{ $clusterHash }}-{{ $sanitisedRegion }}-{{ $specName }}"
  address_space       = ["10.0.0.0/16"]
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $specName }}.name

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_network_security_group" "claudie_nsg_{{ $sanitisedRegion }}_{{ $specName }}" {
  provider            = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  name                = "nsg-{{ $clusterHash }}-{{ $sanitisedRegion }}-{{ $specName }}"
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $specName }}.name

  security_rule {
    name                       = "SSH"
    priority                   = 101
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "Wireguard"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Udp"
    source_port_range          = "*"
    destination_port_range     = "51820"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "ICMP"
    priority                   = 102
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Icmp"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

{{- if eq $.ClusterData.ClusterType "LB" }}
  {{- range $i,$role := index $.Metadata "roles" }}
  security_rule {
    name                       = "Allow-{{ $role.Name }}"
    priority                   = {{ assignPriority $i }}
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "{{ protocolToAzureProtocolString $role.Protocol }}"
    source_port_range          = "*"
    destination_port_range     = "{{ $role.Port }}"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  {{- end }}
{{- end }}

{{- if eq $.ClusterData.ClusterType "K8s" }}
  {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
  security_rule {
    name                       = "KubeApi"
    priority                   = 103
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "6443"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  {{- end }}
{{- end }}

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}
