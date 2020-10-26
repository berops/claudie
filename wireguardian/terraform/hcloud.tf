provider "hcloud" {
  token = "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1" 
}

resource "hcloud_ssh_key" "kubeone" {
  name       = "vpn-${var.cluster_name}"
  public_key = file(var.ssh_public_key_file)
}

resource "hcloud_server" "control_plane" {
  count       = 10
  name        = "${var.cluster_name}-node-${count.index + 1}"
  server_type = var.control_plane_type
  image       = var.image
  location    = var.datacenter

  ssh_keys = [
    hcloud_ssh_key.kubeone.id,
  ]

  labels = {
    "kubeone_cluster_name" = var.cluster_name
    "role"                 = "api"
  }
}

