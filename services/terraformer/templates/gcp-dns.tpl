provider "google" {
    credentials = "${file("{{.Provider.Credentials}}")}"
    project = "{{.Project}}"
}

data "google_dns_managed_zone" "zone" {
  name = "{{.DNSZone}}"
}

{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- $hostnameHash := .HostnameHash }}
{{- range $nodepool := .NodePools}}

resource "google_dns_record_set" "{{$nodepool.Name}}-{{$clusterName}}" {

  name = "{{ $hostnameHash }}.${data.google_dns_managed_zone.zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.zone.name

  rrdatas = [
      for node in google_compute_instance.{{$nodepool.Name}} : node.network_interface.0.access_config.0.nat_ip
    ]
}

output "{{$clusterName}}-{{$clusterHash}}" {
  value = { {{$clusterName}}-{{$clusterHash}} = google_dns_record_set.{{$nodepool.Name}}-{{$clusterName}}.name }
}
{{- end}}