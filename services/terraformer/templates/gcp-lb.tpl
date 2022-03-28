{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "google" {
  credentials = "${file("{{(index .NodePools $index).Provider.Name}}")}"
  region = "europe-west1"
  project = "platform-296509"
  alias  = "lb-nodepool"
}

resource "google_compute_network" "network" {
  provider     = google.lb-nodepool
  name                    = "{{ $clusterName }}-{{ $clusterHash }}-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  provider     = google.lb-nodepool
  name          = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
  network       = google_compute_network.network.self_link
  region        = "europe-west1"
  ip_cidr_range = "10.0.0.0/8"
}

resource "google_compute_firewall" "firewall" {
  provider     = google.lb-nodepool
  name    = "{{ $clusterName }}-{{ $clusterHash }}-firewall"
  network = google_compute_network.network.self_link

  allow {
      protocol = "TCP"
      ports    = ["1-65535"]
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
  provider     = google.lb-nodepool
  count        = {{$nodepool.Count}}
  zone         = "europe-west1-c"
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{$nodepool.Name}}-${count.index + 1}"
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