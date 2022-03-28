provider "google" {
  credentials = "${file("../../../../../keys/platform-296509-d6ddeb344e91.json")}"
  region = "europe-west1"
  project = "platform-296509"
}

{{- $cluster := .Cluster}}

resource "google_compute_network" "network" {
  name                    = "{{ .Cluster.Name }}-{{.Cluster.Hash}}-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  name          = "{{ .Cluster.Name }}-{{.Cluster.Hash}}-subnet"
  network       = google_compute_network.network.self_link
  region        = "europe-west1"
  ip_cidr_range = "10.0.0.0/8"
}

resource "google_compute_firewall" "firewall" {
  name    = "{{ .Cluster.Name }}-{{.Cluster.Hash}}-firewall"
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

{{range $nodepool := .NodePools}}
resource "google_compute_instance" "{{$nodepool.Name}}" {
  count        = {{$nodepool.Count}}
  zone         = "europe-west1-c"
  name         = "{{$cluster.Name}}-{{$cluster.Hash}}-{{$nodepool.Name}}-${count.index + 1}"
  machine_type = "{{$nodepool.ServerType}}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = 10
      image = "{{$nodepool.Image}}"
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

output "{{$nodepool.Name}}" {
  value = {
    for node in google_compute_instance.{{$nodepool.Name}}:
    node.name => node.network_interface.0.access_config.0.nat_ip
  }
}
{{end}}


