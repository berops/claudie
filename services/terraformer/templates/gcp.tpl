provider "google" {
  region = "europe-west1"
}

{{- $index := .Index }}

resource "google_compute_instance" "control_plane" {
  count        = {{ (index .Cluster.NodePools $index).Master.Count }}
  project      = "platform-296509"
  zone         = "europe-west1-c"
  name         = "{{ .Cluster.Name }}-gcp-control-${count.index + 1}"
  machine_type = "{{ (index .Cluster.NodePools $index).Master.ServerType }}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = 10
      image = "{{ (index .Cluster.NodePools $index).Master.Image }}"
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
  count        = {{ (index .Cluster.NodePools $index).Worker.Count }}
  project      = "platform-296509"
  zone         = "europe-west1-c"
  name         = "{{ .Cluster.Name }}-gcp-compute-${count.index + 1}"
  machine_type = "{{ (index .Cluster.NodePools $index).Worker.ServerType }}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = 10
      image = "{{ (index .Cluster.NodePools $index).Worker.Image }}"
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
    content = templatefile("../../templates/output_gcp.tpl",
        {
            control = google_compute_instance.control_plane[*]
            compute = google_compute_instance.compute_plane[*]
        }
    )
    filename = "output"
}