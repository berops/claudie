provider "google" {
  credentials = "${file("../../../../../keys/platform-296509-d6ddeb344e91.json")}"
  region = "europe-west1"
  project = "platform-296509"
  alias  = "lb-nodepool"
}
{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}

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