{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- $index :=  0 }}
{{- range $i, $region := .Regions }}
provider "google" {
  credentials = "${file("{{ (index $.NodePools $index).Provider.SpecName }}")}"
  project     = "{{ (index $.NodePools 0).Provider.GcpProject }}"
  region      = "{{ $region }}"
  alias       = "lb_nodepool_{{ $region }}"
}

resource "google_compute_network" "network_{{ $region }}" {
  provider                = google.lb_nodepool_{{ $region }}
  name                    = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-network"
  auto_create_subnetworks = false
  description             = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}

resource "google_compute_firewall" "firewall_{{ $region }}" {
  provider    = google.lb_nodepool_{{ $region }}
  name        = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-firewall"
  network     = google_compute_network.network_{{ $region }}.self_link
  description = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"

  {{- range $role := index $.Metadata "roles" }}
  allow {
      protocol = "{{ $role.Protocol }}"
      ports = ["{{ $role.Port }}"]
  }
  {{- end }}

  allow {
      protocol = "TCP"
      ports    = ["22"]
  }

  allow {
    protocol = "UDP"
    ports    = ["51820"]
  }

  allow {
      protocol = "icmp"
   }

  source_ranges = [
      "0.0.0.0/0",
   ]
}

{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "google_compute_subnetwork" "{{ $nodepool.Name }}_subnet" {
  provider      = google.lb_nodepool_{{ $nodepool.Region }}
  name          = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
  network       = google_compute_network.network_{{ $nodepool.Region }}.self_link
  ip_cidr_range = "{{ index  $.Metadata (printf "%s-subnet-cidr" $nodepool.Name)  }}"
  description   = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}

resource "google_compute_instance" "{{ $nodepool.Name }}" {
  provider                  = google.lb_nodepool_{{ $nodepool.Region }}
  count                     = {{ $nodepool.Count }}
  zone                      = "{{ $nodepool.Zone }}"
  name                      = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  machine_type              = "{{ $nodepool.ServerType }}"
  description   = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = "{{ $nodepool.DiskSize }}"
      image = "{{ $nodepool.Image }}"
    }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.{{ $nodepool.Name }}_subnet.self_link
    access_config {}
  }
  metadata = {
    ssh-keys = "root:${file("./public.pem")}"
  }
  metadata_startup_script = "echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart"

  labels = {
    managed-by = "claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

output "{{ $nodepool.Name }}" {
  value = {
    for node in google_compute_instance.{{ $nodepool.Name }}:
    node.name => node.network_interface.0.access_config.0.nat_ip
  }
}
{{- end }}