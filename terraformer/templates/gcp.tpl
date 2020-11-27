provider "google" {
  region = "europe-west3"
  credentials = "./keys/platform-296509-d6ddeb344e91.json"
}

resource "google_compute_instance" "control_plane" {
  count        = {{ .Cluster.Providers.gcp.ControlNodeSpecs.Count }}
  project      = "platform-296509"
  zone         = "europe-west3-c"
  name         = "test-terraformer-control-{{ .Metadata.Id }}-${count.index + 1}"
  machine_type = "{{ .Cluster.Providers.gcp.ControlNodeSpecs.ServerType }}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = 10
      image = "{{ .Cluster.Providers.gcp.ControlNodeSpecs.Image }}"
    }
  }
  network_interface {
    network = "default"
    access_config {}
  }
  metadata = {
    ssh-keys = file("{{ .Cluster.PublicKey }}")
  }
}

resource "google_compute_instance" "compute_plane" {
  count        = {{ .Cluster.Providers.gcp.ComputeNodeSpecs.Count }}
  project      = "platform-296509"
  zone         = "europe-west3-c"
  name         = "test-terraformer-compute-{{ .Metadata.Id }}-${count.index + 1}"
  machine_type = "{{ .Cluster.Providers.gcp.ComputeNodeSpecs.ServerType }}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = 10
      image = "{{ .Cluster.Providers.gcp.ComputeNodeSpecs.Image }}"
    }
  }
  network_interface {
    network = "default"
    access_config {}
  }
  metadata = {
    ssh-keys = file("{{ .Cluster.PublicKey }}")
  }
}

resource "local_file" "output_gcp" {
    content = templatefile("templates/output.tpl",
        {
            control_public_ip = "${google_compute_instance.control_plane[*].network_interface.0.access_config.0.nat_ip}",
            compute_public_ip = "${google_compute_instance.compute_plane[*].network_interface.0.access_config.0.nat_ip}",
        }
    )
    filename = "terraform/output"
}