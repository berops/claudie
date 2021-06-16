terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.23.0"
    }
  }
}

{{- $index := .Index }}

provider "hcloud" {
  token = "{{ (index .Cluster.NodePools $index).Provider.Credentials }}" 
}

resource "hcloud_server" "control_plane" {
  count       = "{{ (index .Cluster.NodePools $index).Master.Count }}"
  name        = "{{ .Cluster.Name }}-hetzner-control-${count.index + 1}"
  server_type = "{{ (index .Cluster.NodePools $index).Master.ServerType }}"
  image       = "{{ (index .Cluster.NodePools $index).Master.Image }}"

  ssh_keys = [
    3626771,
  ]
}

resource "hcloud_server" "compute_plane" {
  count       = "{{ (index .Cluster.NodePools $index).Worker.Count }}"
  name        = "{{ .Cluster.Name}}-hetzner-compute-${count.index + 1}"
  server_type = "{{ (index .Cluster.NodePools $index).Worker.ServerType }}"
  image       = "{{ (index .Cluster.NodePools $index).Worker.Image }}"

  ssh_keys = [
    3626771,
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
