{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $nodepool := .NodePools }}
{{- $sanitisedRegion := replaceAll $nodepool.NodePool.Region " " "_"}}
{{- $specName := $nodepool.NodePool.Provider.SpecName }}

resource "azurerm_subnet" "{{ $nodepool.Name }}_{{ $sanitisedRegion }}_{{ $specName }}_subnet" {
  provider             = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  name                 = "snt-{{ $clusterHash }}-{{ $sanitisedRegion }}-{{ $nodepool.Name }}"
  resource_group_name  = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $specName }}.name
  virtual_network_name = azurerm_virtual_network.claudie_vn_{{ $sanitisedRegion }}_{{ $specName  }}.name
  address_prefixes     = ["{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name)  }}"]
}

resource "azurerm_subnet_network_security_group_association" "{{ $nodepool.Name }}_{{ $sanitisedRegion }}_{{ $specName }}_associate_nsg" {
  provider                  = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  subnet_id                 = azurerm_subnet.{{ $nodepool.Name }}_{{ $sanitisedRegion }}_{{ $specName }}_subnet.id
  network_security_group_id = azurerm_network_security_group.claudie_nsg_{{ $sanitisedRegion }}_{{ $specName  }}.id
}

{{- range $node := $nodepool.Nodes }}
resource "azurerm_public_ip" "{{ $node.Name }}_public_ip" {
  provider            = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  name                = "ip-{{ $node.Name }}"
  location            = "{{ $nodepool.NodePool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $specName }}.name
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_network_interface" "{{ $node.Name }}_ni" {
  provider            = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $specName }}
  name                = "ni-{{ $node.Name }}"
  location            = "{{ $nodepool.NodePool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $specName }}.name
  enable_accelerated_networking = {{ enableAccNet $nodepool.NodePool.ServerType }}

  ip_configuration {
    name                          = "ip-cfg-{{ $node.Name }}"
    subnet_id                     = azurerm_subnet.{{ $nodepool.Name }}_{{ $sanitisedRegion }}_{{ $specName }}_subnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.{{ $node.Name }}_public_ip.id
    primary                       = true
  }

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}
{{- end }}
