{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{- $index :=  0}}
{{- range $i, $region := .Regions}}
provider "google" {
  credentials = "${file("{{ (index $.NodePools $index).Provider.SpecName }}")}"
  project     = "{{ (index $.NodePools 0).Provider.GcpProject }}"
  region      = "{{ $region }}"
  alias       = "k8s_nodepool_{{ $region }}"
}

resource "google_compute_network" "network_{{ $region }}" {
  provider                = google.k8s_nodepool_{{ $region }}
  name                    = "{{ $clusterName }}-{{ $region }}-network"
  auto_create_subnetworks = false
  description             = "Managed by Claudie; GCP {{ $region }}"
}

resource "google_compute_firewall" "firewall_{{ $region }}" {
  provider     = google.k8s_nodepool_{{ $region }}
  name         = "{{ $clusterName }}-{{ $region }}-firewall"
  network      = google_compute_network.network_{{ $region }}.self_link
  description  = "Managed by Claudie; GCP {{ $region }}"

  allow {
    protocol = "UDP"
    ports    = ["51820"]
  }

  {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
  allow {
      protocol = "TCP"
      ports    = ["6443"]
  }
  {{- end }}

  allow {
      protocol = "TCP"
      ports    = ["22"]
  }

  allow {
      protocol = "icmp"
   }

  source_ranges = [
      "0.0.0.0/0",
   ]
}

{{- end}}


{{- range $i, $nodepool := .NodePools }}
resource "google_compute_subnetwork" "{{ $nodepool.Name }}_subnet" {
  provider      = google.k8s_nodepool_{{ $nodepool.Region }}
  name          = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
  network       = google_compute_network.network-{{ $nodepool.Region }}.self_link
  ip_cidr_range = "{{getCIDR "10.0.0.0/24" 2 $i}}"
  description   = "Managed by Claudie; GCP {{ $region }}"
}

resource "google_compute_instance" "{{ $nodepool.Name }}" {
  provider     = google.k8s_nodepool_{{ $nodepool.Region }}
  count        = {{ $nodepool.Count }}
  zone         = "{{ $nodepool.Zone }}"
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  machine_type = "{{ $nodepool.ServerType }}"
  description  = "Managed by Claudie; GCP {{ $region }}"
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
}

output "{{ $nodepool.Name }}" {
  value = {
    for node in google_compute_instance.{{ $nodepool.Name }}:
    node.name => node.network_interface.0.access_config.0.nat_ip
  }
}
{{- end }}


