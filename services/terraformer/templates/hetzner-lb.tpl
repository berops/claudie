{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- $index :=  0 }}
provider "hcloud" {
  token = "{{ (index .NodePools $index).Provider.Credentials }}" 
  alias = "lb_nodepool"
}

resource "hcloud_ssh_key" "claudie" {
  provider   = hcloud.lb_nodepool
  name       = "key-{{ $clusterName }}-{{ $clusterHash }}"
  public_key = file("./public.pem")
  labels = {
    "environment" : "Claudie"
  }
}

resource "hcloud_firewall" "firewall" {
  provider = hcloud.lb_nodepool
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

  {{- range $role := index .Metadata "roles" }}
  rule {
    direction  = "in"
    protocol   = "{{ $role.Protocol }}"
    port       = "{{ $role.Port }}"
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
    "environment" : "Claudie"
  }
}


{{- range $nodepool := .NodePools }}
resource "hcloud_server" "{{ $nodepool.Name }}" {
  provider     = hcloud.lb_nodepool
  count        = "{{ $nodepool.Count }}"
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index +1}"
  server_type  = "{{ $nodepool.ServerType }}"
  image        = "{{ $nodepool.Image }}"
  firewall_ids = [hcloud_firewall.firewall.id]
  datacenter   = "{{ $nodepool.Zone }}"
  ssh_keys = [
    hcloud_ssh_key.claudie.id,
  ]
  labels = {
    "environment" : "Claudie"
  }
}

output "{{ $nodepool.Name }}" {
  value = {
    for node in hcloud_server.{{ $nodepool.Name }}:
    node.name => node.ipv4_address
  }
}
{{- end }}