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

  {{- if index .Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
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
resource "hcloud_server" "{{ $nodepool.Name }}" {
  provider      = hcloud.k8s_nodepool
  count         = "{{ $nodepool.Count }}"
  name          = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index +1}"
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
}

output "{{ $nodepool.Name }}" {
  value = {
    for node in hcloud_server.{{ $nodepool.Name }}:
    node.name => node.ipv4_address
  }
}
{{- end }}