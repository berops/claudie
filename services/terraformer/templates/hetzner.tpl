terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.23.0"
    }
  }
}

provider "hcloud" {
  token = "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1" 
}

resource "hcloud_ssh_key" "platform" {
  name       = "test_key"
  public_key = file("{{ .Cluster.PublicKey }}")
}

resource "hcloud_server" "control_plane" {
  count       = {{ .Cluster.Providers.hetzner.ControlNodeSpecs.Count }}
  name        = "{{ .Metadata.Id }}-test-terraformer-control-${count.index + 1}"
  server_type = "{{ .Cluster.Providers.hetzner.ControlNodeSpecs.ServerType }}"
  image       = "{{ .Cluster.Providers.hetzner.ControlNodeSpecs.Image }}"

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

resource "hcloud_server" "compute_plane" {
  count       = {{ .Cluster.Providers.hetzner.ComputeNodeSpecs.Count }}
  name        = "{{ .Metadata.Id }}-test-terraformer-compute-${count.index + 1}"
  server_type = "{{ .Cluster.Providers.hetzner.ComputeNodeSpecs.ServerType }}"
  image       = "{{ .Cluster.Providers.hetzner.ComputeNodeSpecs.Image }}"

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

resource "local_file" "output_hetzner" { 
    content = templatefile("templates/output_hetzner.tpl",
        {
            control = hcloud_server.control_plane.*
            compute = hcloud_server.compute_plane.*
        }
    )
    filename = "terraform/output"
}
