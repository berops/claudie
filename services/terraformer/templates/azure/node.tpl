{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}

{{- range $i, $nodepool := .NodePools }}
{{- $sanitisedRegion := replaceAll $nodepool.NodePool.Region " " "_"}}
{{- range $node := $nodepool.Nodes }}

resource "azurerm_linux_virtual_machine" "{{ $node.Name }}" {
  provider              = azurerm.nodepool
  name                  = "{{ $node.Name }}"
  location              = "{{ $nodepool.NodePool.Region }}"
  resource_group_name   = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  network_interface_ids = [azurerm_network_interface.{{ $node.Name }}_ni.id]
  size                  = "{{$nodepool.NodePool.ServerType}}"
  zone                  = "{{$nodepool.NodePool.Zone}}"

  source_image_reference {
    publisher = split(":", "{{ $nodepool.NodePool.Image }}")[0]
    offer     = split(":", "{{ $nodepool.NodePool.Image }}")[1]
    sku       = split(":", "{{ $nodepool.NodePool.Image }}")[2]
    version   = split(":", "{{ $nodepool.NodePool.Image }}")[3]
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

{{- if eq $.ClusterType "LB" }}
  os_disk {
    name                 = "{{ $node.Name }}-osdisk"
    caching              = "ReadWrite"
    storage_account_type = "StandardSSD_LRS"
    disk_size_gb         = "50"
  }
{{- end }}

{{- if eq $.ClusterType "K8s" }}
  os_disk {
    name                 = "{{ $node.Name }}-osdisk"
    caching              = "ReadWrite"
    storage_account_type = "StandardSSD_LRS"
    disk_size_gb         = "100"
  }
{{- end }}
}

resource "azurerm_virtual_machine_extension" "{{ $node.Name }}_{{ $clusterHash }}_postcreation_script" {
  provider             = azurerm.nodepool
  name                 = "{{ $clusterName }}-{{ $clusterHash }}-postcreation-script"
  virtual_machine_id   = azurerm_linux_virtual_machine.{{ $node.Name }}.id
  publisher            = "Microsoft.Azure.Extensions"
  type                 = "CustomScript"
  type_handler_version = "2.0"

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }

  protected_settings = <<PROT
  {
  "script": "${base64encode(<<EOF
#!/bin/bash
set -euxo pipefail

# Allow ssh as root
sudo sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
sudo cat /root/.ssh/temp > /root/.ssh/authorized_keys
sudo rm /root/.ssh/temp
sudo echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
# Mount managed disk only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-path | grep "lun-${azurerm_virtual_machine_data_disk_attachment.{{ $node.Name }}_disk_att.lun}" | awk '{print $NF}')
disk=$(basename "$disk")
if ! grep -qs "/dev/$disk" /proc/mounts; then
  mkdir -p /opt/claudie/data
  if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
    mkfs.xfs /dev/$disk
  fi
  mount /dev/$disk /opt/claudie/data
  echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
fi
    {{- end }}
EOF
)}"
  }
PROT
}

{{- if eq $.ClusterType "K8s" }}
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
resource "azurerm_managed_disk" "{{ $node.Name }}_disk" {
  provider             = azurerm.nodepool
  name                 = "{{ $node.Name }}-disk"
  location             = "{{ $nodepool.NodePool.Region }}"
  zone                 = {{ $nodepool.NodePool.Zone }}
  resource_group_name  = azurerm_resource_group.rg_{{ $sanitisedRegion }}_{{ $clusterName }}_{{ $clusterHash }}.name
  storage_account_type = "StandardSSD_LRS"
  create_option        = "Empty"
  disk_size_gb         = {{ $nodepool.NodePool.StorageDiskSize }}

  tags = {
    managed-by      = "Claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "azurerm_virtual_machine_data_disk_attachment" "{{ $node.Name }}_disk_att" {
  provider           = azurerm.nodepool
  managed_disk_id    = azurerm_managed_disk.{{ $node.Name }}_disk.id
  virtual_machine_id = azurerm_linux_virtual_machine.{{ $node.Name }}.id
  lun                = "1"
  caching            = "ReadWrite"
}
    {{- end }}
{{- end }}

{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${azurerm_linux_virtual_machine.{{ $node.Name }}.computer_name}" = azurerm_public_ip.{{ $node.Name }}_public_ip.ip_address
    {{- end }}
  }
}
{{- end }}
