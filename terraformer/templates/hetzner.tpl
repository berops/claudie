provider "hcloud" {
  token = "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1" 
}

resource "hcloud_ssh_key" "platform" {
  name       = "test_key"
  public_key = file("{{ .PublicKey }}")
}

resource "hcloud_server" "control_plane" {
  count       = {{ .ControlPlane }}
  name        = "test-terraformer-control-${count.index + 1}"
  server_type = "{{ .ControlPlaneType }}"
  image       = "ubuntu-20.04"

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

resource "hcloud_server" "compute_plane" {
  count       = {{ .ComputePlane }}
  name        = "test-terraformer-compute-${count.index + 1}"
  server_type = "{{ .ComputePlaneType }}"
  image       = "ubuntu-20.04"

  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

resource "local_file" "output" { 
    content = templatefile("templates/output.tpl",
        {
            control_public_ip = hcloud_server.control_plane.*.ipv4_address,
            compute_public_ip = hcloud_server.compute_plane.*.ipv4_address,
        }
    )
    filename = "terraform/output"
}