{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "hcloud" {
  token = "{{ (index .NodePools $index).Provider.Credentials }}" 
  alias = "lb-nodepool"
}

resource "hcloud_ssh_key" "claudie" {
  provider     = hcloud.lb-nodepool
  name       = "key-{{ $clusterName }}-{{ $clusterHash }}"
  public_key = file("./public.pem")
}

resource "hcloud_firewall" "firewall" {
  provider     = hcloud.lb-nodepool
  name = "{{ $clusterName }}-{{ $clusterHash }}-firewall"
  rule {
    direction = "in"
    protocol  = "icmp"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "22"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "6443"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol  = "udp"
    port      = "51820"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }
}


{{range $nodepool := .NodePools}}

resource "hcloud_server" "{{$nodepool.Name}}" {
  provider     = hcloud.lb-nodepool
  count        = "{{ $nodepool.Count }}"
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{$nodepool.Name}}-${count.index +1}"
  server_type  = "{{ $nodepool.ServerType }}"
  image        = "{{ $nodepool.Image }}"
  firewall_ids = [hcloud_firewall.firewall.id]
  datacenter   = "{{ $nodepool.Zone}}"
  ssh_keys = [
    hcloud_ssh_key.claudie.id,
  ]
}

output "{{$nodepool.Name}}" {
  value = {
    for node in hcloud_server.{{$nodepool.Name}}:
    node.name => node.ipv4_address
  }
}

{{end}}
