{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{- range $i, $region := .Regions}}
resource "azurerm_resource_group" "rg_{{ replaceAll $region " " "_" }}" {
  provider = azurerm.k8s_nodepool
  name     = "{{ $clusterName }}-{{ $clusterHash }}-{{ replaceAll $region " " "-" }}"
  location = "{{ $region }}"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_virtual_network" "claudie_vn_{{ replaceAll $region " " "_"  }}" {
  provider            = azurerm.k8s_nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-vn"
  address_space       = ["10.0.0.0/16"]
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $region " " "_"  }}.name

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_network_security_group" "claudie_nsg_{{ replaceAll $region " " "_"  }}" {
  provider            = azurerm.k8s_nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-nsg"
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $region " " "_"  }}.name

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

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "azurerm_subnet" "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet" {
  provider             = azurerm.k8s_nodepool
  name                 = "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet"
  resource_group_name  = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_"  }}.name
  virtual_network_name = azurerm_virtual_network.claudie_vn_{{ replaceAll $nodepool.Region " " "_" }}.name
  address_prefixes     = ["{{index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"]
}

resource "azurerm_subnet_network_security_group_association" "{{ $nodepool.Name }}_associate_nsg" {
  provider                  = azurerm.k8s_nodepool
  subnet_id                 = azurerm_subnet.{{ $nodepool.Name }}_{{ $clusterHash }}_subnet.id
  network_security_group_id = azurerm_network_security_group.claudie_nsg_{{replaceAll $nodepool.Region " " "_" }}.id
}

{{- range $node := $nodepool.Nodes }}
resource "azurerm_public_ip" "{{ $node.Name }}_public_ip" {
  provider            = azurerm.k8s_nodepool
  name                = "{{ $node.Name }}-ip"
  location            = "{{ $nodepool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_" }}.name
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_network_interface" "{{ $node.Name }}_ni" {
  provider            = azurerm.k8s_nodepool
  name                = "{{ $node.Name }}-ni"
  location            = "{{ $nodepool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{replaceAll $nodepool.Region " " "_"  }}.name
  enable_accelerated_networking = {{ enableAccNet $nodepool.ServerType }}

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

resource "azurerm_linux_virtual_machine" "{{ $node.Name }}" {
  provider              = azurerm.k8s_nodepool
  name                  = "{{ $node.Name }}"
  location              = "{{ $nodepool.Region }}"
  resource_group_name   = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_"  }}.name
  network_interface_ids = [azurerm_network_interface.{{ $node.Name }}_ni.id]
  size                  = "{{$nodepool.ServerType}}"
  zone                  = "{{$nodepool.Zone}}"

  source_image_reference {
    publisher = split(":", "{{ $nodepool.Image }}")[0]
    offer     = split(":", "{{ $nodepool.Image }}")[1]
    sku       = split(":", "{{ $nodepool.Image }}")[2]
    version   = split(":", "{{ $nodepool.Image }}")[3]
  }

  os_disk {
      name                 = "{{ $node.Name }}-osdisk"
      caching              = "ReadWrite"
      storage_account_type = "Standard_LRS"
      disk_size_gb         = "{{ $nodepool.DiskSize }}"
  }

  disable_password_authentication = true
  admin_ssh_key {
    public_key = file("public.pem")
    username   = "claudie"
  }

  computer_name  = "{{ $node.Name }}"
  admin_username = "claudie"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_virtual_machine_extension" "{{ $node.Name }}_{{ $clusterHash }}_postcreation_script" {
  provider             = azurerm.k8s_nodepool
  name                 = "{{ $clusterName }}-{{ $clusterHash }}-postcreation-script"
  virtual_machine_id   = azurerm_linux_virtual_machine.{{ $node.Name }}.id
  publisher            = "Microsoft.Azure.Extensions"
  type                 = "CustomScript"
  type_handler_version = "2.0"

  protected_settings = <<PROT
  {
      "script": "${base64encode(<<EOF
      sudo sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
      sudo cat /root/.ssh/temp > /root/.ssh/authorized_keys
      sudo rm /root/.ssh/temp
      sudo echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
      EOF
      )}"
  }
PROT

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${azurerm_linux_virtual_machine.{{ $node.Name }}.computer_name}" = azurerm_public_ip.{{ $node.Name }}_public_ip.ip_address
    {{- end }}
  }
}
{{- end }}