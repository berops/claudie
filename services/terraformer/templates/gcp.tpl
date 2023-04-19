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

resource "google_compute_network" "network_{{ $clusterName}}_{{ $clusterHash}}_{{ $region }}" {
  provider                = google.k8s_nodepool_{{ $region }}
  name                    = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-network"
  auto_create_subnetworks = false
  description             = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}

resource "google_compute_firewall" "firewall_{{ $region }}" {
  provider     = google.k8s_nodepool_{{ $region }}
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-firewall"
  network      = google_compute_network.network_{{ $clusterName}}_{{ $clusterHash}}_{{ $region }}.self_link
  description  = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"

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

{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "google_compute_subnetwork" "{{ $nodepool.Name }}_subnet" {
  provider      = google.k8s_nodepool_{{ $nodepool.Region }}
  name          = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
  network       = google_compute_network.network_{{ $clusterName}}_{{ $clusterHash}}_{{ $nodepool.Region }}.self_link
  ip_cidr_range = "{{index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  description   = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
}

{{- range $node := $nodepool.Nodes }}
resource "google_compute_instance" "{{ $node.Name }}" {
  provider                  = google.k8s_nodepool_{{ $nodepool.Region }}
  zone                      = "{{ $nodepool.Zone }}"
  name                      = "{{ $node.Name }}"
  machine_type              = "{{ $nodepool.ServerType }}"
  description   = "Managed by Claudie for cluster {{ $clusterName }}-{{ $clusterHash }}"
  allow_stopping_for_update = true
  boot_disk {
    initialize_params {
      size = "100"
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
  metadata_startup_script = <<EOF
# Allow ssh as root
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && service sshd restart

{{- if not $nodepool.IsControl }}
# Mount managed disk only when not mounted yet
if ! grep -qs "/dev/sdb" /proc/mounts; then
  mkdir -p /opt/claudie/data
  # turns out this can be whatever we want it to be.
  # https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_attached_disk#device_name
  DISK="/dev/disk/by-id/google-{{ $node.Name }}"
  mkfs.xfs "$DISK" || return
  mount "$DISK" /opt/claudie/data
  echo "$DISK /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
fi
{{- end }}
EOF
  
  labels = {
    managed-by = "claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

{{- if not $nodepool.IsControl }}
resource "google_compute_disk" "{{ $node.Name }}_disk" {
  provider = google.k8s_nodepool_{{ $nodepool.Region }}
  name     = "{{ $node.Name }}-disk"
  type     = "pd-ssd"
  zone     = "{{ $nodepool.Zone }}"
  size     = {{ $nodepool.StorageDiskSize }}

  labels = {
    managed-by = "claudie"
    claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "google_compute_attached_disk" "{{ $node.Name }}_disk_att" {
  provider    = google.k8s_nodepool_{{ $nodepool.Region }}
  disk        = google_compute_disk.{{ $node.Name }}_disk.id
  instance    = google_compute_instance.{{ $node.Name }}.id
  zone        = "{{ $nodepool.Zone }}"
  device_name = "{{ $node.Name }}"
}
{{- end }}
{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
  {{- range $node := $nodepool.Nodes }}
    "${google_compute_instance.{{ $node.Name }}.name}" = google_compute_instance.{{ $node.Name }}.network_interface.0.access_config.0.nat_ip
  {{- end }}
  }
}
{{- end }}


