{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}

{{- range $i, $region := .Regions }}
resource "genesiscloud_ssh_key" "claudie_{{ $clusterName }}_{{ $clusterHash }}_{{ $region }}" {
  provider   = genesiscloud.nodepool_{{ $region }}
  name       = "{{ $clusterName }}-{{ $clusterHash }}-key"
  public_key = file("./public.pem")
}

data "genesiscloud_images" "base_os_{{ $region }}" {
  provider   = genesiscloud.nodepool_{{ $region }}
  filter = {
    type   = "base-os"
    region = "{{ $region }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
    {{- range $node := $nodepool.Nodes }}

{{- if and (eq $.ClusterType "K8s") (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
resource "genesiscloud_volume" "{{ $node.Name }}_volume" {
  provider = genesiscloud.nodepool_{{ $nodepool.NodePool.Region }}
  name   = "{{ $node.Name }}-volume"
  region = "{{ $nodepool.NodePool.Region }}"
  size   = {{ $nodepool.NodePool.StorageDiskSize}}
  type   = "hdd"
}
{{- end }}

resource "genesiscloud_instance" "{{ $node.Name }}" {
  provider = genesiscloud.nodepool_{{ $nodepool.NodePool.Region }}
  name   = "{{ $node.Name }}"
  region = "{{ $nodepool.NodePool.Region }}"

  image_id = data.genesiscloud_images.base_os_{{ $nodepool.NodePool.Region }}.images[index(data.genesiscloud_images.base_os_{{ $nodepool.NodePool.Region }}.images.*.name, "{{ $nodepool.NodePool.Image}}")].id
  type     = "{{ $nodepool.NodePool.ServerType }}"

  public_ip_type = "static"

{{- if and (eq $.ClusterType "K8s") (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
  volume_ids = [
    genesiscloud_volume.{{ $node.Name }}_volume.id
  ]
{{- end }}

  ssh_key_ids = [
    genesiscloud_ssh_key.claudie_{{ $clusterName }}_{{ $clusterHash }}_{{ $nodepool.NodePool.Region }}.id,
  ]

  security_group_ids = [
    genesiscloud_security_group.claudie_security_group_{{ $clusterName }}_{{ $clusterHash}}_{{ $nodepool.NodePool.Region }}.id
  ]

  {{- if eq $.ClusterType "LB" }}
  metadata = {
    startup_script = <<EOF
#!/bin/bash
set -eo pipefail
sudo sed -i -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart
EOF
  }
  {{- end }}

  {{- if eq $.ClusterType "K8s" }}
  metadata = {
    startup_script = <<EOF
#!/bin/bash
set -eo pipefail

# Allow ssh as root
sudo sed -i -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart

# startup script
mkdir -p /opt/claudie/data
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
# Mount volume only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-id | grep "${genesiscloud_volume.{{ $node.Name }}_volume.id}" | awk '{print $NF}')
disk=$(basename "$disk")
if ! grep -qs "/dev/$disk" /proc/mounts; then

  if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
    mkfs.xfs /dev/$disk
  fi
  mount /dev/$disk /opt/claudie/data
  echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
fi
    {{- end }}
EOF

  }
  {{- end }}
}
    {{- end }}

output "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${genesiscloud_instance.{{ $node.Name }}.name}" = genesiscloud_instance.{{ $node.Name }}.public_ip
    {{- end }}
  }
}
{{- end }}