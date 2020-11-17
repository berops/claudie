provider "hcloud" {
  token = "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1" 
}

resource "hcloud_ssh_key" "platform" {
  name       = "test_key"
  public_key = "~/.ssh/samuel_stolicny"
}

resource "hcloud_server" "control_plane" {
  count       = 2
  name        = "test-node-${count.index + 1}"
  server_type = "cpx11"
  image       = "ubuntu-20.04"

  ssh_keys = [
    hcloud_ssh_key.kubeone.id,
  ]
}

resource "hcloud_server" "compute_plane" {
  count       = 2
  name        = "test-node-${count.index + 1}"
  server_type = "cpx11"
  image       = "ubuntu-20.04"

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}