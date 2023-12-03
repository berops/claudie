{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}

{{- if eq $.ClusterType "K8s" }}
variable "oci_storage_disk_name" {
  default = "oraclevdb"
  type    = string
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
{{- range $node := $nodepool.Nodes }}
resource "oci_core_instance" "{{ $node.Name }}" {
  provider            = oci.nodepool_{{ $nodepool.NodePool.Region }}
  compartment_id      = var.default_compartment_id
  availability_domain = "{{ $nodepool.NodePool.Zone }}"
  shape               = "{{ $nodepool.NodePool.ServerType }}"
  display_name        = "{{ $node.Name }}"

{{if $nodepool.NodePool.MachineSpec}}
   shape_config {
       memory_in_gbs = {{ $nodepool.NodePool.MachineSpec.Memory }}
       ocpus = {{ $nodepool.NodePool.MachineSpec.CpuCount }}
   }
{{end}}

  create_vnic_details {
    assign_public_ip  = true
    subnet_id         = oci_core_subnet.{{ $nodepool.Name }}_subnet.id
  }

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }

{{- if eq $.ClusterType "LB" }}
  source_details {
    source_id               = "{{ $nodepool.NodePool.Image }}"
    source_type             = "image"
    boot_volume_size_in_gbs = "50"
  }

  metadata = {
      ssh_authorized_keys = file("./public.pem")
      user_data = base64encode(<<EOF
      #cloud-config
      runcmd:
        # Allow Claudie to ssh as root
        - sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
        - cat /root/.ssh/temp > /root/.ssh/authorized_keys
        - rm /root/.ssh/temp
        - echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
        # Disable iptables
        # Accept all traffic to avoid ssh lockdown via iptables firewall rules
        - iptables -P INPUT ACCEPT
        - iptables -P FORWARD ACCEPT
        - iptables -P OUTPUT ACCEPT
        # Flush and cleanup
        - iptables -F
        - iptables -X
        - iptables -Z
        # Make changes persistent
        - netfilter-persistent save
      EOF
      )
  }
{{- end }}

{{- if eq $.ClusterType "K8s" }}
  source_details {
    source_id               = "{{ $nodepool.NodePool.Image }}"
    source_type             = "image"
    boot_volume_size_in_gbs = "100"
  }

  metadata = {
      ssh_authorized_keys = file("./public.pem")
      user_data = base64encode(<<EOF
      #cloud-config
      runcmd:
        # Allow Claudie to ssh as root
        - sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
        - cat /root/.ssh/temp > /root/.ssh/authorized_keys
        - rm /root/.ssh/temp
        - echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
        # Disable iptables
        # Accept all traffic to avoid ssh lockdown via iptables firewall rules
        - iptables -P INPUT ACCEPT
        - iptables -P FORWARD ACCEPT
        - iptables -P OUTPUT ACCEPT
        # Flush and cleanup
        - iptables -F
        - iptables -X
        - iptables -Z
        # Make changes persistent
        - netfilter-persistent save
        {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
        # Mount volume
        - |
          sleep 50
          disk=$(ls -l /dev/oracleoci | grep "${var.oci_storage_disk_name}" | awk '{print $NF}')
          disk=$(basename "$disk")
          if ! grep -qs "/dev/$disk" /proc/mounts; then
            mkdir -p /opt/claudie/data
            if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
              mkfs.xfs /dev/$disk
            fi
            mount /dev/$disk /opt/claudie/data
            echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
          fi
        {{- end }}
      EOF
      )
  }
{{- end }}
}

{{- if eq $.ClusterType "K8s" }}
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
resource "oci_core_volume" "{{ $node.Name }}_volume" {
  provider            = oci.nodepool_{{ $nodepool.NodePool.Region }}
  compartment_id      = var.default_compartment_id
  availability_domain = "{{ $nodepool.NodePool.Zone }}"
  size_in_gbs         = "{{ $nodepool.NodePool.StorageDiskSize }}"
  display_name        = "{{ $node.Name }}-volume"
  vpus_per_gb         = 10

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "oci_core_volume_attachment" "{{ $node.Name }}_volume_att" {
  provider        = oci.nodepool_{{ $nodepool.NodePool.Region }}
  attachment_type = "paravirtualized"
  instance_id     = oci_core_instance.{{ $node.Name }}.id
  volume_id       = oci_core_volume.{{ $node.Name }}_volume.id
  display_name    = "{{ $node.Name }}-volume-att"
  device          = "/dev/oracleoci/${var.oci_storage_disk_name}"
}
    {{- end }}
{{- end }}

{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
  {{- range $node := $nodepool.Nodes }}
    "${oci_core_instance.{{ $node.Name }}.display_name}" = oci_core_instance.{{ $node.Name }}.public_ip
  {{- end }}
  }
}
{{- end }}
