{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{- $index :=  0 }}
provider "azurerm" {
  features {}
  subscription_id = "{{ (index $.NodePools 0).Provider.AzureSubscriptionId }}"
  tenant_id       = "{{ (index $.NodePools 0).Provider.AzureTenantId }}"
  client_id       = "{{ (index $.NodePools 0).Provider.AzureClientId }}"
  client_secret   = file("{{ (index $.NodePools 0).Provider.SpecName }}")
  alias           = "lb_nodepool"
}

{{- range $i, $region := .Regions }}
resource "azurerm_resource_group" "rg_{{ replaceAll $region " " "_" }}" {
  provider = azurerm.lb_nodepool
  name     = "{{ $clusterName }}-{{ $clusterHash }}-{{ replaceAll $region " " "_" }}"
  location = "{{ $region }}"
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_virtual_network" "claudie_vn_{{ replaceAll $region " " "_" }}" {
  provider            = azurerm.lb_nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-vn"
  address_space       = ["10.0.0.0/16"]
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $region " " "_" }}.name
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_network_security_group" "claudie_nsg_{{ replaceAll $region " " "_" }}" {
  provider            = azurerm.lb_nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-nsg"
  location            = "{{ $region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $region " " "_" }}.name

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
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "azurerm_subnet" "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet" {
  provider             = azurerm.lb_nodepool
  name                 = "{{ $nodepool.Name }}_{{ $clusterHash }}_subnet"
  resource_group_name  = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_"}}.name
  virtual_network_name = azurerm_virtual_network.claudie_vn_{{ replaceAll $nodepool.Region " " "_" }}.name
  address_prefixes     = ["{{ getCIDR "10.0.0.0/24" 2 $i }}"]
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_subnet_network_security_group_association" "{{ $nodepool.Name }}_associate_nsg" {
  provider                  = azurerm.lb_nodepool
  subnet_id                 = azurerm_subnet.{{ $nodepool.Name }}_{{ $clusterHash }}_subnet.id
  network_security_group_id = azurerm_network_security_group.claudie_nsg_{{ replaceAll $nodepool.Region " " "_" }}.id
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_public_ip" "{{ $nodepool.Name }}_{{ $clusterHash }}_public_ip" {
  provider            = azurerm.lb_nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}-ip"
  count               = {{ $nodepool.Count }}
  location            = "{{ $nodepool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_"}}.name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_network_interface" "{{ $nodepool.Name }}_{{ $clusterHash }}_ni" {
  provider            = azurerm.lb_nodepool
  count               = {{ $nodepool.Count }}
  name                = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-ni-${count.index + 1}"
  location            = "{{ $nodepool.Region }}"
  resource_group_name = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_"}}.name
  enable_accelerated_networking = {{ enableAccNet $nodepool.ServerType }}

  ip_configuration {
    name                          = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}-ip-conf"
    subnet_id                     = azurerm_subnet.{{ $nodepool.Name }}_{{ $clusterHash }}_subnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = element(azurerm_public_ip.{{ $nodepool.Name }}_{{ $clusterHash }}_public_ip, count.index).id
    primary                       = true
  }
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_linux_virtual_machine" "{{ $nodepool.Name }}" {
  provider              = azurerm.lb_nodepool
  count                 = {{ $nodepool.Count }}
  name                  = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  location              = "{{ $nodepool.Region }}"
  resource_group_name   = azurerm_resource_group.rg_{{ replaceAll $nodepool.Region " " "_"}}.name
  network_interface_ids = [element(azurerm_network_interface.{{ $nodepool.Name }}_{{ $clusterHash }}_ni, count.index).id]
  size                  = "{{ $nodepool.ServerType }}"
  zone                  = "{{ $nodepool.Zone }}"

  source_image_reference {
    publisher = split(":", "{{ $nodepool.Image }}")[0]
    offer     = split(":", "{{ $nodepool.Image }}")[1]
    sku       = split(":", "{{ $nodepool.Image }}")[2]
    version   = split(":", "{{ $nodepool.Image }}")[3]
  }

  os_disk {
      name                 = "{{ $nodepool.Name }}-{{ $clusterHash }}-osdisk-${count.index+1}"
      caching              = "ReadWrite"
      storage_account_type = "Standard_LRS"
      disk_size_gb         = "{{ $nodepool.DiskSize }}"
  }

  disable_password_authentication = true
  admin_ssh_key {
    public_key = file("public.pem")
    username   = "claudie"
  }

  computer_name  = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  admin_username = "claudie"
  tags = {
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

resource "azurerm_virtual_machine_extension" "{{ $nodepool.Name }}_{{ $clusterHash }}_postcreation_script" {
  provider = azurerm.lb_nodepool
  name                 = "{{ $clusterName }}-{{ $clusterHash }}-postcreation-script"
  for_each             = { for vm in azurerm_linux_virtual_machine.{{$nodepool.Name}} : vm.name => vm }
  virtual_machine_id   = each.value.id
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
    environment = "Managed by Claudie"
    name        = "Azure {{ $region }}"
  }
}

output "{{ $nodepool.Name }}" {
  value = {
    for index, ip in azurerm_public_ip.{{ $nodepool.Name }}_{{ $clusterHash }}_public_ip:
    "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${index + 1}" => ip.ip_address
  }
}
{{- end }}