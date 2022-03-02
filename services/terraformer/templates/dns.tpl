provider "google" {
    credentials = "${file("{{.Provider.Credentials}}")}"
    project = "{{.Project}}"
    alias = "dns-gcp"
}

data "google_dns_managed_zone" "gcp-zone" {
  provider = google.dns-gcp
  name = "{{.DNSZone}}"
}

resource "google_dns_record_set" "record" {
  provider = google.dns-gcp

  name = "{{ .HostnameHash }}.${data.google_dns_managed_zone.gcp-zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.gcp-zone.name

  rrdatas = [
      {{range $IP := .NodeIPs}}
      "{{$IP}}",
      {{end}}
    ]
}

output "{{.ClusterName}}-{{.ClusterHash}}" {
  value = { "{{.ClusterName}}-{{.ClusterHash}}-endpoint" = google_dns_record_set.record.name }
}