{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{- $index :=  0 }}
provider "hcloud" {
  token = "{{ (index .NodePools $index).Provider.Credentials }}" 
  alias = "k8s_nodepool"
}

resource "hcloud_firewall" "defaultfirewall" {
  provider = hcloud.k8s_nodepool
  name     = "{{ $clusterName }}-{{ $clusterHash }}-firewall"
  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }
  {{- end }}

  rule {
    direction  = "in"
    protocol   = "udp"
    port       = "51820"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  labels = {
    "managed-by"      : "Claudie"
    "claudie-cluster" : "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "hcloud_ssh_key" "claudie" {
  provider   = hcloud.k8s_nodepool
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
  provider      = hcloud.k8s_nodepool
  name          = "{{ $node.Name }}"
  server_type   = "{{ $nodepool.ServerType }}"
  image         = "{{ $nodepool.Image }}"
  firewall_ids  = [hcloud_firewall.defaultfirewall.id]
  datacenter    = "{{ $nodepool.Zone }}"

  ssh_keys = [
    hcloud_ssh_key.claudie.id,
  ]

  labels = {
    "managed-by"      : "Claudie"
    "claudie-cluster" : "{{ $clusterName }}-{{ $clusterHash }}"
  }

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
}

{{- if not $nodepool.IsControl }}
resource "hcloud_volume" "{{ $node.Name }}_volume" {
  provider  = hcloud.k8s_nodepool
  name      = "{{ $node.Name }}-volume"
  size      = {{ $nodepool.StorageDiskSize }}
  format    = "xfs"
  location = "{{ $nodepool.Region }}"
}

resource "hcloud_volume_attachment" "{{ $node.Name }}_volume_att" {
  provider  = hcloud.k8s_nodepool
  volume_id = hcloud_volume.{{ $node.Name }}_volume.id
  server_id = hcloud_server.{{ $node.Name }}.id
  automount = false
}
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