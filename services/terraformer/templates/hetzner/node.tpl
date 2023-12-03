{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
resource "hcloud_ssh_key" "claudie" {
  provider   = hcloud.nodepool
  name       = "key-{{ $clusterName }}-{{ $clusterHash }}"
  public_key = file("./public.pem")

  labels = {
    "managed-by"      : "Claudie"
    "claudie-cluster" : "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

{{- range $nodepool := .NodePools }}
{{- range $node := $nodepool.Nodes }}
resource "hcloud_server" "{{ $node.Name }}" {
  provider      = hcloud.nodepool
  name          = "{{ $node.Name }}"
  server_type   = "{{ $nodepool.NodePool.ServerType }}"
  image         = "{{ $nodepool.NodePool.Image }}"
  firewall_ids  = [hcloud_firewall.firewall.id]
  datacenter    = "{{ $nodepool.NodePool.Zone }}"
  public_net {
     ipv6_enabled = false
  }
  ssh_keys = [
    hcloud_ssh_key.claudie.id,
  ]
  labels = {
    "managed-by"      : "Claudie"
    "claudie-cluster" : "{{ $clusterName }}-{{ $clusterHash }}"
  }

{{- if and (eq $.ClusterType "K8s") (gt $nodepool.NodePool.StorageDiskSize 0) }}
    {{- if not $nodepool.IsControl }}
  user_data = <<EOF
#!/bin/bash
# Mount volume only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-id | grep "${hcloud_volume.{{ $node.Name }}_volume.id}" | awk '{print $NF}')
disk=$(basename "$disk")
if ! grep -qs "/dev/$disk" /proc/mounts; then
  mkdir -p /opt/claudie/data
  if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
    mkfs.xfs /dev/$disk
  fi
  mount /dev/$disk /opt/claudie/data
  echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
fi
EOF
    {{- end }}
{{- end }}
}

{{- if and (eq $.ClusterType "K8s") (gt $nodepool.NodePool.StorageDiskSize 0) }}
    {{- if not $nodepool.IsControl }}
resource "hcloud_volume" "{{ $node.Name }}_volume" {
  provider  = hcloud.nodepool
  name      = "{{ $node.Name }}-volume"
  size      = {{ $nodepool.NodePool.StorageDiskSize }}
  format    = "xfs"
  location = "{{ $nodepool.NodePool.Region }}"
}

resource "hcloud_volume_attachment" "{{ $node.Name }}_volume_att" {
  provider  = hcloud.nodepool
  volume_id = hcloud_volume.{{ $node.Name }}_volume.id
  server_id = hcloud_server.{{ $node.Name }}.id
  automount = false
}
    {{- end }}
{{- end }}

{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${hcloud_server.{{ $node.Name }}.name}" = hcloud_server.{{ $node.Name }}.ipv4_address
    {{- end }}
  }
}
{{- end }}
