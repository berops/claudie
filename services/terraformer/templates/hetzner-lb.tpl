terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.31.1"
    }
  }
}

{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "hcloud" {
  token = "{{ (index .NodePools $index).Provider.Credentials }}" 
}

resource "hcloud_ssh_key" "platform" {
  name       = "key-{{ $clusterName }}-{{ $clusterHash }}"
  public_key = file("./public.pem")
}


{{range $nodepool := .NodePools}}

resource "hcloud_server" "{{$nodepool.Name}}" {
  count       = "{{ $nodepool.Count }}"
  name        = "{{ $clusterName }}-{{ $clusterHash }}-{{$nodepool.Name}}-${count.index +1}"
  server_type = "{{ $nodepool.ServerType }}"
  image       = "{{ $nodepool.Image }}"

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
