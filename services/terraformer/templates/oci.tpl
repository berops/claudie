{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

variable "default_compartment_id" {
  type = string
  default = "{{(index .NodePools 0).Provider.OciCompartmentId}}"
}

provider "oci" {
   tenancy_ocid = "{{(index .NodePools 0).Provider.TenancyOcid}}"
   user_ocid = "{{(index .NodePools 0).Provider.UserOcid}}"
   fingerprint = "{{(index .NodePools 0).Provider.OciFingerprint}}"
   private_key_path = "{{(index .NodePools 0).Provider.SpecName}}" 
   region = "{{(index .NodePools 0).Region}}"
}

resource "oci_core_vcn" "claudie_vcn" {
    compartment_id = var.default_compartment_id
    display_name = "{{ $clusterName }}-{{ $clusterHash }}-vcn"
    cidr_blocks = ["10.0.0.0/16"]
}

resource "oci_core_subnet" "claudie_subnet" {
    vcn_id = oci_core_vcn.claudie_vcn.id
    cidr_block = "10.0.0.0/24"
    compartment_id = var.default_compartment_id
    display_name = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
    security_list_ids = [oci_core_vcn.claudie_vcn.default_security_list_id]
    route_table_id = oci_core_vcn.claudie_vcn.default_route_table_id
    dhcp_options_id   = oci_core_vcn.claudie_vcn.default_dhcp_options_id
}

resource "oci_core_internet_gateway" "claudie_gateway" {
  compartment_id = var.default_compartment_id
  display_name   = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
  vcn_id         = oci_core_vcn.claudie_vcn.id
  enabled = true
}

resource "oci_core_default_security_list" "claudie_security_rules" {
  manage_default_resource_id = oci_core_vcn.claudie_vcn.default_security_list_id
  display_name   = "{{ $clusterName }}-{{ $clusterHash }}_security_rules"

  egress_security_rules {  
    destination = "0.0.0.0/0"
    protocol    = "all"
    description = "Allow all egress"
  }

  ingress_security_rules {
    protocol = "1"
    source   = "0.0.0.0/0"
    description = "Allow all ICMP"
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    tcp_options {
      min = "22"
      max = "22"
    }
    description = "Allow SSH connections"
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    tcp_options {
      max = "6443"
      min = "6443"
    }
    description = "Allow kube API port"
  }

  ingress_security_rules {
    protocol    = "17"
    source      = "0.0.0.0/0"
    udp_options {
    max = "51820"
    min = "51820"
    }
    description = "Allow Wireguard VPN port"
  }
}

resource "oci_core_default_route_table" "claudie_routes" {
  manage_default_resource_id = oci_core_vcn.claudie_vcn.default_route_table_id

  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.claudie_gateway.id
    destination_type  = "CIDR_BLOCK"
  }
}

{{ range $nodepool := .NodePools }}
resource "oci_core_instance" "{{ $nodepool.Name }}" {
    compartment_id = var.default_compartment_id
    count = {{ $nodepool.Count }}
    availability_domain = "{{ $nodepool.Zone }}"
    shape = "{{ $nodepool.ServerType }}"
    display_name = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  
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
          # Accept all traffic first to avoid ssh lockdown  via iptables firewall rules 
          - iptables -P INPUT ACCEPT
          - iptables -P FORWARD ACCEPT
          - iptables -P OUTPUT ACCEPT
          # Flush and cleanup
          - iptables -F
          - iptables -X
          - iptables -Z 
        EOF
        )
    }
  
    source_details {
        source_id   = "{{ $nodepool.Image }}"
        source_type = "image"
        boot_volume_size_in_gbs = "{{ $nodepool.DiskSize }}"
    }

    create_vnic_details {
        assign_public_ip = true
        subnet_id = oci_core_subnet.claudie_subnet.id
    }
}

output "{{$nodepool.Name}}" {
  value = {
    for node in oci_core_instance.{{$nodepool.Name}}:
    node.display_name => node.public_ip
  }
}
{{end}}
