{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $nodepool := .NodePools }}

{{- $region   := $nodepool.NodePool.Region }}
{{- $specName := $nodepool.NodePool.Provider.SpecName }}

{{- range $node := $nodepool.Nodes }}

{{- if and (eq $.ClusterData.ClusterType "K8s") (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
resource "genesiscloud_volume" "{{ $node.Name }}_{{ $region }}_{{ $specName }}_volume" {
  provider = genesiscloud.nodepool_{{ $region }}_{{ $specName }}
  name   = "{{ $node.Name }}d"
  region = "{{ $region }}"
  size   = {{ $nodepool.NodePool.StorageDiskSize}}
  type   = "hdd"
}
{{- end }}

resource "genesiscloud_instance" "{{ $node.Name }}_{{ $region }}_{{ $specName }}" {
  provider = genesiscloud.nodepool_{{ $region }}_{{ $specName }}
  name   = "{{ $node.Name }}"
  region = "{{ $region }}"

  image_id = data.genesiscloud_images.base_os_{{ $region }}_{{ $specName }}.images[index(data.genesiscloud_images.base_os_{{ $region }}_{{ $specName }}.images.*.name, "{{ $nodepool.NodePool.Image}}")].id
  type     = "{{ $nodepool.NodePool.ServerType }}"

  public_ip_type = "static"

{{- if and (eq $.ClusterData.ClusterType "K8s") (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
  volume_ids = [
    genesiscloud_volume.{{ $node.Name }}_{{ $region }}_{{ $specName }}_volume.id
  ]
{{- end }}

  ssh_key_ids = [
    genesiscloud_ssh_key.claudie_{{ $region }}_{{ $specName }}.id,
  ]

  security_group_ids = [
    genesiscloud_security_group.claudie_security_group_{{ $region }}_{{ $specName }}.id
  ]

  {{- if eq $.ClusterData.ClusterType "LB" }}
  metadata = {
    startup_script = <<EOF
#!/bin/bash
set -eo pipefail
sudo sed -i -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart
EOF
  }
  {{- end }}

  {{- if eq $.ClusterData.ClusterType "K8s" }}
  metadata = {
    startup_script = <<EOF
#!/bin/bash
set -eo pipefail

# Allow ssh as root
sudo sed -i -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart

# startup script
mkdir -p /opt/claudie/data
    {{- if not $nodepool.IsControl }}
     {{- if gt $nodepool.NodePool.StorageDiskSize 0 }}
     sleep 30
# The IDs listed by `/dev/disk/by-id` are different then the volume ids assigned by genesis cloud.
# This is a hacky way assuming that only the longhorn volume will be mounted at startup and no other volume
longhorn_diskuuid=$(blkid | grep genesis_cloud | grep -oP 'UUID="\K[^"]+')
disk=$(ls -l /dev/disk/by-uuid/ | grep $longhorn_diskuuid | awk '{print $NF}')
disk=$(basename "$disk")

# The volume is automatically mounted, since we want it for longhorn specifically we have to re-mount the volume under /opt/claudie/data.
umount -l /dev/$disk

if ! grep -qs "/dev/$disk" /proc/mounts; then
  if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
    mkfs.xfs -f /dev/$disk
  fi
  mount /dev/$disk /opt/claudie/data
  echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
fi
     {{- end }}
    {{- end }}
EOF

  }
  {{- end }}
}
{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${genesiscloud_instance.{{ $node.Name }}_{{ $region }}_{{ $specName }}.name}" = genesiscloud_instance.{{ $node.Name }}_{{ $region }}_{{ $specName }}.public_ip
    {{- end }}
  }
}
{{- end }}
