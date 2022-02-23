provider "google" {
    credentials = "${file("../../../../../keys/platform-infrastructure-316112-bd7953f712df.json")}"
    region = "europe-west1"
    project = "platform-infrastructure-316112"
}

resource "google_dns_managed_zone" "lb-zone" {
  name        = "{{ .ClusterName }}-{{ .ClusterHash }}"
  dns_name    = "{{ .Hostname }}"
  visibility = "public"
}
{{- $clusterName := .ClusterName }}
{{- range $nodepool := .NodePools}}

resource "google_dns_record_set" "{{$nodepool.Name}}-{{$clusterName}}" {

  name = "{{ .ClusterName }}-{{ .ClusterHash }}.${google_dns_managed_zone.lb-zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = google_dns_managed_zone.lb-zone.name

  rrdatas = [
      for node in google_compute_instance.{{$nodepool.Name}} : node.network_interface.0.access_config.0.nat_ip
    ]
}
{{- end}}