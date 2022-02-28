provider "google" {
    credentials = "${file("{{.Provider.Credentials}}")}"
    project = "{{.Project}}"
    alias = "dns-gcp"
}

data "google_dns_managed_zone" "gcp-zone" {
  provider = google.dns-gcp
  name = "{{.DNSZone}}"
}

{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- $hostnameHash := .HostnameHash }}
{{- range $nodepool := .NodePools}}

resource "google_dns_record_set" "{{$nodepool.Name}}-{{$clusterName}}" {
  provider = google.dns-gcp

  name = "{{ $hostnameHash }}.${data.google_dns_managed_zone.gcp-zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.gcp-zone.name

  rrdatas = [
      for node in google_compute_instance.{{$nodepool.Name}} : node.network_interface.0.access_config.0.nat_ip
    ]
}

output "{{$clusterName}}-{{$clusterHash}}-{{$nodepool.Name}}" {
  value = { "{{$clusterName}}-{{$clusterHash}}-{{$nodepool.Name}}" = google_dns_record_set.{{$nodepool.Name}}-{{$clusterName}}.name }
}
{{- end}}