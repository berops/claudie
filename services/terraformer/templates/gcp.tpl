provider "google" {
  region = "europe-west1"
}

resource "google_compute_instance" "control_plane" {
  count        = {{ .Cluster.Providers.gcp.ControlNodeSpecs.Count }}
  project      = "platform-296509"
  zone         = "europe-west1-c"
  name         = "gcp-control-{{ .Metadata.Id }}-${count.index + 1}"
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
  name         = "gcp-compute-{{ .Metadata.Id }}-${count.index + 1}"
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
    content = templatefile("templates/output_gcp.tpl",
        {
            control = google_compute_instance.control_plane[*]
            compute = google_compute_instance.compute_plane[*]
        }
    )
    filename = "terraform/output"
}