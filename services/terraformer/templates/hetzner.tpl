terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.31.1"
    }
  }
}

resource "hcloud_firewall" "defaultfirewall" {
  name = "default-firewall"
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

{{- $index := .Index }}

provider "hcloud" {
  token = "{{ (index .Cluster.NodePools $index).Provider.Credentials }}" 
}

resource "hcloud_ssh_key" "platform" {
  name       = "key-{{ .Cluster.Name }}"
  public_key = file("./public.pem")
}


resource "hcloud_server" "control_plane" {
  count       = "{{ (index .Cluster.NodePools $index).Master.Count }}"
  name        = "{{ .Cluster.Name }}-hetzner-control-${count.index + 1}"
  server_type = "{{ (index .Cluster.NodePools $index).Master.ServerType }}"
  image       = "{{ (index .Cluster.NodePools $index).Master.Image }}"
  firewall_ids = [hcloud_firewall.defaultfirewall.id]

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

resource "hcloud_server" "compute_plane" {
  count       = "{{ (index .Cluster.NodePools $index).Worker.Count }}"
  name        = "{{ .Cluster.Name}}-hetzner-compute-${count.index + 1}"
  server_type = "{{ (index .Cluster.NodePools $index).Worker.ServerType }}"
  image       = "{{ (index .Cluster.NodePools $index).Worker.Image }}"
  firewall_ids = [hcloud_firewall.defaultfirewall.id]

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

output "hetzner" {
  value = {
    control = {
      for node in hcloud_server.control_plane:
      node.name => node.ipv4_address
    }
    compute = {
      for node in hcloud_server.compute_plane:
      node.name => node.ipv4_address
    }
  }
}
