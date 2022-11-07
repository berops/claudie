{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "azurerm" {
  features {}
  subscription_id = "{{(index .NodePools 0).Provider.AzureSubscriptionId}}"
  tenant_id       = "{{(index .NodePools 0).Provider.AzureTenantId}}"
  client_id       = "{{(index .NodePools 0).Provider.AzureClientId}}"
  client_secret   = file("{{(index .NodePools 0).Provider.SpecName}}")
  alias           = "k8s-nodepool"
}

variable "default_rg_name" {
  default  = "{{(index .NodePools 0).Provider.AzureResourceGroup}}"
}

variable "default_rg_location" {
  default = "{{(index .NodePools 0).Region}}"
}

resource "azurerm_virtual_network" "claudie-vn" {
  provider = azurerm.k8s-nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-vn"
  address_space       = ["10.0.0.0/16"]
  location            = var.default_rg_location
  resource_group_name = var.default_rg_name
}

resource "azurerm_subnet" "claudie-subnet" {
  provider = azurerm.k8s-nodepool
  name                 = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
  resource_group_name  = var.default_rg_name
  virtual_network_name = azurerm_virtual_network.claudie-vn.name
  address_prefixes     = ["10.0.0.0/24"]
}

resource "azurerm_network_security_group" "claudie-nsg" {
  provider = azurerm.k8s-nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-nsg"
  location            = var.default_rg_location
  resource_group_name = var.default_rg_name

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
  {{ if index .Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
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
  {{end}}
}

resource "azurerm_subnet_network_security_group_association" "associate-nsg" {
  provider = azurerm.k8s-nodepool
  subnet_id                 = azurerm_subnet.claudie-subnet.id
  network_security_group_id = azurerm_network_security_group.claudie-nsg.id
}


{{ range $nodepool := .NodePools }}
resource "azurerm_public_ip" "{{ $nodepool.Name }}-{{ $clusterHash }}-public-ip" {
  provider = azurerm.k8s-nodepool
  name                = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}-ip"
  count               = {{$nodepool.Count}}
  location            = "{{ $nodepool.Region }}"
  resource_group_name = var.default_rg_name
  allocation_method   = "Static"
  zones               = ["{{ $nodepool.Zone }}"]
  sku                 = "Standard"
}

resource "azurerm_network_interface" "{{ $nodepool.Name }}-{{ $clusterHash }}-ni" {
  provider = azurerm.k8s-nodepool
  count               = {{$nodepool.Count}}
  name                = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-ni-${count.index + 1}"
  location            = "{{ $nodepool.Region }}"
  resource_group_name = var.default_rg_name
  enable_accelerated_networking = {{ enableAccNet $nodepool.ServerType }}

  ip_configuration {
    name                          = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}-ip-conf"
    subnet_id                     = azurerm_subnet.claudie-subnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = element(azurerm_public_ip.{{ $nodepool.Name }}-{{ $clusterHash }}-public-ip, count.index).id
    primary                       = true
  }
}

resource "azurerm_virtual_machine" "{{ $nodepool.Name }}" {
  provider = azurerm.k8s-nodepool
  count                 = {{$nodepool.Count}}
  name                  = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  location              = "{{ $nodepool.Region }}"
  resource_group_name   = var.default_rg_name
  network_interface_ids = [element(azurerm_network_interface.{{ $nodepool.Name }}-{{ $clusterHash }}-ni, count.index).id]
  vm_size               = "{{$nodepool.ServerType}}"
  zones                 = ["{{$nodepool.Zone}}"]

  delete_os_disk_on_termination = true
  delete_data_disks_on_termination = true

  storage_image_reference {
    publisher = split(":", "{{$nodepool.Image}}")[0]
    offer     = split(":", "{{$nodepool.Image}}")[1]
    sku       = split(":", "{{$nodepool.Image}}")[2]
    version   = split(":", "{{$nodepool.Image}}")[3]
  }

  storage_os_disk {
    name              = "{{ $nodepool.Name }}-{{ $clusterHash }}-osdisk-${count.index+1}"
    caching           = "ReadWrite"
    create_option     = "FromImage"
    managed_disk_type = "Standard_LRS"
    disk_size_gb      = "{{ $nodepool.DiskSize }}"
  }

  os_profile_linux_config {
    disable_password_authentication = true
    ssh_keys {
      key_data = file("public.pem")
      path = "/home/claudie/.ssh/authorized_keys"

    }
  }

  os_profile {
    computer_name  = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
    admin_username = "claudie"
  }
}

resource "azurerm_virtual_machine_extension" "{{ $nodepool.Name }}-{{ $clusterHash }}-postcreation-script" {
  provider = azurerm.k8s-nodepool
  name                 = "{{ $clusterName }}-{{ $clusterHash }}-postcreation-script"
  for_each             = { for vm in azurerm_virtual_machine.{{$nodepool.Name}} : vm.name => vm }
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
}

output "{{ $nodepool.Name }}" {
  value = {
    for index, ip in azurerm_public_ip.{{$nodepool.Name}}-{{ $clusterHash }}-public-ip:
    "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${index + 1}" => ip.ip_address
  }
}
{{end}}