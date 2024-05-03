{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $nodepool := .NodePools }}

{{- $specName := $nodepool.NodePool.Provider.SpecName }}

{{- range $node := $nodepool.Nodes }}
resource "hcloud_server" "{{ $node.Name }}_{{ $specName }}" {
  provider      = hcloud.nodepool_{{ $specName }}
  name          = "{{ $node.Name }}"
  server_type   = "{{ $nodepool.NodePool.ServerType }}"
  image         = "{{ $nodepool.NodePool.Image }}"
  firewall_ids  = [hcloud_firewall.firewall_{{ $specName }}.id]
  datacenter    = "{{ $nodepool.NodePool.Zone }}"
  public_net {
     ipv6_enabled = false
  }
  ssh_keys = [
    hcloud_ssh_key.claudie_{{ $specName }}.id,
  ]
  labels = {
    "managed-by"      : "Claudie"
    "claudie-cluster" : "{{ $clusterName }}-{{ $clusterHash }}"
  }

{{- if eq $.ClusterData.ClusterType "K8s" }}
  user_data = <<EOF
#!/bin/bash
# Create longhorn volume directory
mkdir -p /opt/claudie/data
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
# Mount volume only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-id | grep "${hcloud_volume.{{ $node.Name }}_{{ $specName }}_volume.id}" | awk '{print $NF}')
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
{{- end }}
}

{{- if eq $.ClusterData.ClusterType "K8s" }}
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
resource "hcloud_volume" "{{ $node.Name }}_{{ $specName }}_volume" {
  provider  = hcloud.nodepool_{{ $specName }}
  name      = "{{ $node.Name }}d"
  size      = {{ $nodepool.NodePool.StorageDiskSize }}
  format    = "xfs"
  location = "{{ $nodepool.NodePool.Region }}"
}

resource "hcloud_volume_attachment" "{{ $node.Name }}_{{ $specName }}_volume_att" {
  provider  = hcloud.nodepool_{{ $specName }}
  volume_id = hcloud_volume.{{ $node.Name }}_{{ $specName }}_volume.id
  server_id = hcloud_server.{{ $node.Name }}_{{ $specName }}.id
  automount = false
}
    {{- end }}
{{- end }}

{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${hcloud_server.{{ $node.Name }}_{{ $specName }}.name}" = hcloud_server.{{ $node.Name }}_{{ $specName }}.ipv4_address
    {{- end }}
  }
}
{{- end }}
