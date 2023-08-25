{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}

{{- range $i, $region := .Regions }}
{{- $sanitisedRegion := replaceAll $region " " "_"}}
resource "azurerm_resource_group" "rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}" {
  provider = azurerm.nodepool
  name     = "{{ $clusterName }}-{{ $clusterHash }}-{{ $sanitisedRegion }}"
  location = "{{ $region }}"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_virtual_network" "claudie_vn_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}" {
  provider            = azurerm.nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-vn"
  address_space       = ["10.0.0.0/16"]
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_network_security_group" "claudie_nsg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}" {
  provider            = azurerm.nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-nsg"
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name

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

{{- if eq $.ClusterType "LB" }}
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

{{- if eq $.ClusterType "K8s" }}
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

{{- range $i, $nodepool := .NodePools }}
{{- $sanitisedRegion := replaceAll $nodepool.NodePool.Region " " "_"}}
resource "azurerm_subnet" "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet" {
  provider             = azurerm.nodepool
  name                 = "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet"
  resource_group_name  = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  virtual_network_name = azurerm_virtual_network.claudie_vn_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  address_prefixes     = ["{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name)  }}"]
}

resource "azurerm_subnet_network_security_group_association" "{{ $nodepool.Name }}_associate_nsg" {
  provider                  = azurerm.nodepool
  subnet_id                 = azurerm_subnet.{{ $nodepool.Name }}_{{ $clusterHash }}_subnet.id
  network_security_group_id = azurerm_network_security_group.claudie_nsg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.id
}

{{- range $node := $nodepool.Nodes }}
resource "azurerm_public_ip" "{{ $node.Name }}_public_ip" {
  provider            = azurerm.nodepool
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
  provider            = azurerm.nodepool
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
