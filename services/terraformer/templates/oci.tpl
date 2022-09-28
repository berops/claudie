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
    cidr_block = "10.0.0.0/16"
    compartment_id = var.default_compartment_id
    display_name = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
    security_list_ids = [oci_core_security_list.claudie_security_rules.id]
}

resource "oci_core_internet_gateway" "claudie_gateway" {
  compartment_id = var.default_compartment_id
  display_name   = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
  vcn_id         = oci_core_vcn.claudie_vcn.id
}

resource "oci_core_default_route_table" "claudie_routes" {
  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.claudie_gateway.id
  }
  manage_default_resource_id = oci_core_vcn.claudie_vcn.default_route_table_id
}

resource "oci_core_security_list" "claudie_security_rules" {
  vcn_id         = oci_core_vcn.claudie_vcn.id
  display_name   = "{{ $clusterName }}-{{ $clusterHash }}_security_rules"
  compartment_id = var.default_compartment_id

  egress_security_rules {
    destination      = "0.0.0.0/0"
    protocol         = "all"
    destination_type = "CIDR_BLOCK"
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    source_type = "CIDR_BLOCK"

    tcp_options {
      max = "22"
      min = "22"
    }
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    source_type = "CIDR_BLOCK"

    tcp_options {
      max = "6443"
      min = "6443"
    }
  }

  ingress_security_rules {
    protocol    = "17"
    source      = "0.0.0.0/0"
    source_type = "CIDR_BLOCK"

    udp_options {
      max = "51820"
      min = "51820"
    }
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
        #script to allow Claudie to ssh as root
        user_data = base64encode(<<EOF
        #cloud-config
        runcmd:
          - sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
          - cat /root/.ssh/temp > /root/.ssh/authorized_keys
          - rm /root/.ssh/temp
          - echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
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
