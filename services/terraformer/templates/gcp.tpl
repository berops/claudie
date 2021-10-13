provider "google" {
  credentials = "${file("../../../../keys/platform-296509-d6ddeb344e91.json")}"
  region = "europe-west1"
  project = "platform-296509"
}

{{- $index := .Index }}

resource "google_compute_network" "network" {
  name                    = "{{ .Cluster.Name }}-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  name          = "{{ .Cluster.Name }}-subnet"
  network       = google_compute_network.network.self_link
  region        = "europe-west1"
  ip_cidr_range = "10.0.0.0/8"
}

resource "google_compute_firewall" "firewall" {
  name    = "{{ .Cluster.Name }}-firewall"
  network = google_compute_network.network.self_link

  allow {
    protocol = "UDP"
    ports    = ["51820"]
  }

  allow {
      protocol = "TCP"
      ports    = ["22", "6443"]
  }

  allow {
      protocol = "icmp"
   }

  source_ranges = [
      "0.0.0.0/0",
   ]
}

resource "google_compute_instance" "control_plane" {
  count        = {{ (index .Cluster.NodePools $index).Master.Count }}
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
    subnetwork = google_compute_subnetwork.subnet.self_link
    access_config {}
  }
  metadata = {
    ssh-keys = "root:${file("./public.pem")}"
  }
  metadata_startup_script = "echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart"
}

resource "google_compute_instance" "compute_plane" {
  count        = {{ (index .Cluster.NodePools $index).Worker.Count }}
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
    subnetwork = google_compute_subnetwork.subnet.self_link
    access_config {}
  }
  metadata = {
    ssh-keys = "root:${file("./public.pem")}"
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