provider "google" {
    credentials = "${file("{{ .Provider.SpecName }}")}"
    project     = "{{ .Provider.GcpProject }}"
    alias       = "dns_gcp"
}

data "google_dns_managed_zone" "gcp_zone" {
  provider  = google.dns_gcp
  name      = "{{ .DNSZone }}"
}

resource "google_dns_record_set" "record" {
  provider = google.dns_gcp

  name = "{{ .HostnameHash }}.${data.google_dns_managed_zone.gcp_zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.gcp_zone.name

  rrdatas = [
      {{- range $IP := .NodeIPs }}
      "{{ $IP }}",
      {{- end }}
    ]
}

output "{{.ClusterName}}-{{.ClusterHash}}" {
  value = { "{{.ClusterName}}-{{.ClusterHash}}-endpoint" = google_dns_record_set.record.name }
}