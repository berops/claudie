provider "hetznerdns" {
    apitoken = "${file("{{ .Provider.SpecName }}")}"
    alias = "hetzner_dns"
}

data "hetznerdns_zone" "hetzner-zone" {
  provider = hetznerdns.hetzner_dns
    name = "{{ .DNSZone }}"
}

{{ range $IP := .NodeIPs }}
resource "hetznerdns_record" "record_{{ replaceAll $IP "." "-" }}" {
  provider = hetznerdns.hetzner_dns
  zone_id = data.hetznerdns_zone.hetzner-zone.id
  name = "{{ $.HostnameHash }}"
  value = "{{ $IP }}"
  type = "A"
  ttl= 300
}
{{- end }}

output "{{ .ClusterName }}-{{ .ClusterHash }}" {
  value = { "{{ .ClusterName }}-{{ .ClusterHash }}-endpoint" = format("%s.%s", "{{ .HostnameHash }}", "{{ .DNSZone }}")}
}