provider "google" {
  region = "europe-west1"
}

resource "google_compute_instance" "control_plane" {
  count        = {{ .Cluster.Providers.gcp.ControlNodeSpecs.Count }}
  project      = "platform-296509"
  zone         = "europe-west1-c"
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
    ssh-keys = "root:${file("{{.Cluster.PublicKey }}")}"
  }
  metadata_startup_script = "echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart"
}

resource "google_compute_instance" "compute_plane" {
  count        = {{ .Cluster.Providers.gcp.ComputeNodeSpecs.Count }}
  project      = "platform-296509"
  zone         = "europe-west1-c"
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
    ssh-keys = "root:${file("{{.Cluster.PublicKey }}")}"
  }
  metadata_startup_script = "echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart"
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