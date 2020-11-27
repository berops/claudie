provider "google" {
  region = "europe-west3"
  credentials = "./keys/platform-296509-d6ddeb344e91.json"
}

resource "google_compute_instance" "control_plane" {
  count        = {{ .ControlPlane }}
  project      = "platform-296509"
  zone         = "europe-west3-c"
  name         = "test-terraformer-control-${count.index + 1}"
  machine_type = "{{ .ControlPlaneType }}"
  allow_stopping_for_update = true

  boot_disk {
    initialize_params {
      size = 10
      image = "ubuntu-os-cloud/ubuntu-2004-lts"
    }
  }

  network_interface {
    network = "default"
    access_config {
      #nat_ip = google_compute_address.static_ip.address
    }
  }

  metadata = {
    ssh-keys = file("{{ .PublicKey }}")
  }
}

resource "google_compute_instance" "compute_plane" {
  count        = {{ .ComputePlane }}
  project      = "platform-296509"
  zone         = "europe-west3-c"
  name         = "test-terraformer-compute-${count.index + 1}"
  machine_type = "{{ .ComputePlaneType }}"
  allow_stopping_for_update = true

  boot_disk {
    initialize_params {
      size = 10
      image = "ubuntu-os-cloud/ubuntu-2004-lts"
    }
  }

  network_interface {
    network = "default"
    access_config {
      #nat_ip = google_compute_address.static_ip.address
    }
  }

  metadata = {
    ssh-keys = file("{{ .PublicKey }}")
  }
}

resource "local_file" "output" {
    content = templatefile("templates/output.tpl",
        {
            control_public_ip = "${google_compute_instance.control_plane[*].network_interface.0.access_config.0.nat_ip}",
            compute_public_ip = "${google_compute_instance.compute_plane[*].network_interface.0.access_config.0.nat_ip}",
        }
    )
    filename = "terraform/output"
}