provider "google" {
    credentials = "${file("{{.Provider.Credentials}}")}"
    project = "{{.Project}}"
    alias = "dns-hetzner"
}

data "google_dns_managed_zone" "hetzner-zone" {
  provider = google.dns-hetzner
  name = "{{.DNSZone}}"
}

{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- $hostnameHash := .HostnameHash }}
{{- range $nodepool := .NodePools}}

resource "google_dns_record_set" "{{$clusterName}}-{{$clusterHash}}-{{$nodepool.Name}}" {
  provider = google.dns-hetzner
  name = "{{ $hostnameHash }}.${data.google_dns_managed_zone.hetzner-zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.hetzner-zone.name

  rrdatas = [
        for node in hcloud_server.{{$nodepool.Name}} :node.ipv4_address
    ]
}

output "{{$clusterName}}-{{$clusterHash}}-{{$nodepool.Name}}" {
  value = { "{{$clusterName}}-{{$clusterHash}}-{{$nodepool.Name}}" = google_dns_record_set.{{$clusterName}}-{{$clusterHash}}-{{$nodepool.Name}}.name }
}
{{- end}}