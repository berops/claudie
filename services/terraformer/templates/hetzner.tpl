terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.31.1"
    }
  }
}

{{- $cluster := .Cluster}}
{{$index :=  0}}

resource "hcloud_firewall" "defaultfirewall" {
  name = "{{ $cluster.Name }}-{{$cluster.Hash}}-firewall"
  rule {
    direction = "in"
    protocol  = "icmp"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "22"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "6443"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol  = "udp"
    port      = "51820"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }
}


provider "hcloud" {
  token = "{{ (index .NodePools $index).Provider.Credentials }}" 
}

resource "hcloud_ssh_key" "platform" {
  name       = "key-{{ $cluster.Name }}-{{$cluster.Hash}}"
  public_key = file("./public.pem")
}


{{range $nodepool := .NodePools}}

resource "hcloud_server" "{{$nodepool.Name}}" {
  count       = "{{ $nodepool.Count }}"
  name        = "{{ $cluster.Name }}-{{$cluster.Hash}}-{{$nodepool.Name}}-${count.index +1}"
  server_type = "{{ $nodepool.ServerType }}"
  image       = "{{ $nodepool.Image }}"
  firewall_ids = [hcloud_firewall.defaultfirewall.id]

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

output "{{$nodepool.Name}}" {
  value = {
    for node in hcloud_server.{{$nodepool.Name}}:
    node.name => node.ipv4_address
  }
}

{{end}}

