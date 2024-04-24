{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $i, $nodepool := .NodePools }}
{{- $sanitisedRegion := replaceAll $nodepool.NodePool.Region " " "_"}}
resource "azurerm_subnet" "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet" {
  provider             = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $nodepool.NodePool.Provider.SpecName }}
  name                 = "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet"
  resource_group_name  = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  virtual_network_name = azurerm_virtual_network.claudie_vn_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  address_prefixes     = ["{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name)  }}"]
}

resource "azurerm_subnet_network_security_group_association" "{{ $nodepool.Name }}_associate_nsg" {
  provider                  = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $nodepool.NodePool.Provider.SpecName }}
  subnet_id                 = azurerm_subnet.{{ $nodepool.Name }}_{{ $clusterHash }}_subnet.id
  network_security_group_id = azurerm_network_security_group.claudie_nsg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.id
}

{{- range $node := $nodepool.Nodes }}
resource "azurerm_public_ip" "{{ $node.Name }}_public_ip" {
  provider            = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $nodepool.NodePool.Provider.SpecName }}
  name                = "{{ $node.Name }}-ip"
  location            = "{{ $nodepool.NodePool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_network_interface" "{{ $node.Name }}_ni" {
  provider            = azurerm.nodepool_{{ $sanitisedRegion }}_{{ $nodepool.NodePool.Provider.SpecName }}
  name                = "{{ $node.Name }}-ni"
  location            = "{{ $nodepool.NodePool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  enable_accelerated_networking = {{ enableAccNet $nodepool.NodePool.ServerType }}

  ip_configuration {
    name                          = "{{ $node.Name }}-ip-conf"
    subnet_id                     = azurerm_subnet.{{ $nodepool.Name }}_{{ $clusterHash }}_subnet.id
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
