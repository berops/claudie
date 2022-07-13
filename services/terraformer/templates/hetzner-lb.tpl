terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.34.3"
    }
  }
}

{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "hcloud" {
  token = "{{ (index .NodePools $index).Provider.Credentials }}" 
  alias = "lb-nodepool"
}

resource "hcloud_ssh_key" "platform" {
  provider     = hcloud.lb-nodepool
  name       = "key-{{ $clusterName }}-{{ $clusterHash }}"
  public_key = file("./public.pem")
}


{{range $nodepool := .NodePools}}

resource "hcloud_server" "{{$nodepool.Name}}" {
  provider     = hcloud.lb-nodepool
  count        = "{{ $nodepool.Count }}"
  name         = "{{ $clusterName }}-{{ $clusterHash }}-{{$nodepool.Name}}-${count.index +1}"
  server_type  = "{{ $nodepool.ServerType }}"
  image        = "{{ $nodepool.Image }}"
  datacenter   = "{{ $nodepool.Zone}}"
  ssh_keys = [
    hcloud_ssh_key.platform.id,
  ]
}

output "{{$nodepool.Name}}" {
  value = {
    for node in hcloud_server.{{$nodepool.Name}}:
    node.name => node.ipv4_address
  }
}

{{end}}
