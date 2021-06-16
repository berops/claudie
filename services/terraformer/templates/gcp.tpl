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
    ssh-keys = "root:{{.Cluster.PublicKey }}"
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
    ssh-keys = <<EOT
                root:{{ .Cluster.PublicKey }}
                EOT
  }
  metadata_startup_script = "echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart"
}

output "gcp" {
  value = {
    control = {
      for node in google_compute_instance.control_plane:
      node.name => node.network_interface.0.access_config.0.nat_ip
    }
    compute = {
      for node in google_compute_instance.compute_plane:
      node.name => node.network_interface.0.access_config.0.nat_ip
    }
  }
}