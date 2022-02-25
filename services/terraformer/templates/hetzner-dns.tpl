provider "google" {
    credentials = "${file("../../../../../keys/platform-infrastructure-316112-bd7953f712df.json")}"
    region = "europe-west1"
    project = "platform-infrastructure-316112"
}

data "google_dns_managed_zone" "zone" {
  name = "{{.Zone}}"
}

{{- $clusterName := .ClusterName }}
{{- $hostnameHash := .HostnameHash }}
{{- range $nodepool := .NodePools}}


resource "google_dns_record_set" "{{$nodepool.Name}}-{{$clusterName}}" {

  name = "{{ $hostnameHash }}.${data.google_dns_managed_zone.zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.zone.name

  rrdatas = [
        for node in hcloud_server.{{$nodepool.Name}} :node.ipv4_address
    ]
}

output "{{$clusterName}}-{{$clusterHash}}" {
  value = { APIEndpoint = google_dns_record_set.{{$nodepool.Name}}-{{$clusterName}}.name }
}
{{- end}}